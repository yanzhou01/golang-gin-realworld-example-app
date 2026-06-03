#!/bin/bash
# setup-dm.sh
# 在 Aurora binlog 开启后，部署 TiDB DM 并启动 CDC 同步
# 在 EC2 上运行：bash ~/migration/setup-dm.sh
#
# 前置条件：
#   1. Aurora log_bin = ON（AWS 控制台修改参数组，见下方说明）
#   2. 已完成全量迁移（migrate-snapshot.sh），或将 task-mode 改为 "all"
#
set -euo pipefail

DM_CLUSTER="dm-realworld"
DM_VERSION="v7.5.0"
DMCTL="tiup dmctl --master-addr=127.0.0.1:8261"
MIGRATION_DIR="$HOME/migration"

echo "=== Step 1: 安装 tiup + DM ==="
if ! command -v tiup &>/dev/null; then
  curl --proto '=https' --tlsv1.2 -sSf https://tiup-mirrors.pingcap.com/install.sh | sh
  export PATH=$HOME/.tiup/bin:$PATH
fi
tiup install dm:$DM_VERSION dmctl:$DM_VERSION 2>/dev/null || true
echo "  ✓ DM $DM_VERSION"

echo ""
echo "=== Step 2: 下载 RDS CA 证书 ==="
curl -sSo ~/rds-ca.pem https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem
echo "  ✓ ~/rds-ca.pem"

echo ""
echo "=== Step 3: 验证 Aurora binlog ==="
AURORA_HOST="${AURORA_HOST:?need AURORA_HOST}"
AURORA_USER="${AURORA_USER:-admin}"
AURORA_PASS="${AURORA_PASS:?need AURORA_PASS}"

LOG_BIN=$(mysql -h "$AURORA_HOST" -P 3306 -u "$AURORA_USER" -p"$AURORA_PASS" \
  --ssl-mode=REQUIRED --ssl-ca=~/rds-ca.pem \
  -N -e "SHOW VARIABLES LIKE 'log_bin';" 2>/dev/null | awk '{print $2}')
if [ "$LOG_BIN" != "ON" ]; then
  echo "  ✗ log_bin=$LOG_BIN — binlog 未开启"
  echo ""
  echo "  请先在 AWS 控制台开启 Aurora binlog："
  echo "  1. RDS → 参数组 → 新建 Cluster 参数组"
  echo "  2. 修改参数：binlog_format = ROW"
  echo "  3. 将参数组应用到集群并重启"
  echo "  4. 重新运行本脚本"
  exit 1
fi
echo "  ✓ log_bin=ON"

echo ""
echo "=== Step 4: 部署 DM 集群 ==="
# tiup dm deploy 通过 SSH 连接目标机（含 localhost），需要密钥
# 生成本机 SSH 密钥并添加到 authorized_keys，使 localhost 免密登录
# tiup dm deploy 内部用 SSH 连 localhost，通过 ~/.ssh/config 指定密钥
if [ ! -f ~/.ssh/id_rsa ]; then
  ssh-keygen -t rsa -b 2048 -N "" -f ~/.ssh/id_rsa -q
fi
grep -qF "$(cat ~/.ssh/id_rsa.pub)" ~/.ssh/authorized_keys 2>/dev/null \
  || cat ~/.ssh/id_rsa.pub >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys

# 写入 SSH config，让 tiup 连接 127.0.0.1 时自动用 id_rsa
mkdir -p ~/.ssh
cat >> ~/.ssh/config << 'SSHCONF'
Host 127.0.0.1
  IdentityFile ~/.ssh/id_rsa
  StrictHostKeyChecking no
  UserKnownHostsFile /dev/null
SSHCONF
chmod 600 ~/.ssh/config

if tiup dm display "$DM_CLUSTER" &>/dev/null 2>&1; then
  echo "  DM 集群已存在，跳过部署"
else
  tiup dm deploy "$DM_CLUSTER" "$DM_VERSION" "$MIGRATION_DIR/dm-topology.yaml" -y
  echo "  ✓ DM 集群部署完成"
fi
tiup dm start "$DM_CLUSTER"
sleep 5
echo "  ✓ DM 集群已启动"
tiup dm display "$DM_CLUSTER"

echo ""
echo "=== Step 5: 注册 Aurora 为 DM Source ==="
$DMCTL operate-source create "$MIGRATION_DIR/dm-source.yaml"
echo "  ✓ Source 注册完成"
$DMCTL get-config source aurora-realworld

echo ""
echo "=== Step 6: 启动迁移任务 ==="
$DMCTL start-task "$MIGRATION_DIR/dm-task.yaml"
echo "  ✓ 任务已提交"

echo ""
echo "=== Step 7: 检查任务状态 ==="
sleep 10
$DMCTL query-status realworld-live-migration

echo ""
echo "=== DM 部署完成 ==="
echo "  查看同步状态：tiup dmctl --master-addr=127.0.0.1:8261 query-status realworld-live-migration"
echo "  暂停同步：    tiup dmctl --master-addr=127.0.0.1:8261 pause-task realworld-live-migration"
echo "  停止同步：    tiup dmctl --master-addr=127.0.0.1:8261 stop-task  realworld-live-migration"
