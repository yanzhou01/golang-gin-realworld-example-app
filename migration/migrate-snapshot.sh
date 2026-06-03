#!/bin/bash
# migrate-snapshot.sh
# 全量迁移：Aurora MySQL → TiDB Cloud
# 在 EC2 上运行：bash ~/migration/migrate-snapshot.sh
#
# 前置条件：
#   - ~/rds-ca.pem 已存在（脚本自动下载）
#   - EC2 能访问 Aurora 和 TiDB Cloud
#
set -euo pipefail

AURORA_HOST="yanzhouw-newhire-test-source.cluster-cdximlzkzbgd.ap-northeast-1.rds.amazonaws.com"
AURORA_USER="admin"
AURORA_PASS="password"
AURORA_DB="realworld"

TIDB_HOST="privatelink-20616390.mczxoo2az8r7.clusters.tidb-cloud.com"
TIDB_PORT=4000
TIDB_USER="root"
TIDB_PASS="password"
TIDB_DB="realworld"

DUMP_DIR="$HOME/aurora-dump"

AURORA_CLI="mysql -h $AURORA_HOST -P 3306 -u $AURORA_USER -p$AURORA_PASS \
  --ssl-mode=REQUIRED --ssl-ca=$HOME/rds-ca.pem"
TIDB_CLI="mysql -h $TIDB_HOST -P $TIDB_PORT -u $TIDB_USER -p$TIDB_PASS"

# ─────────────────────────────────────────────────────────────────────────────

echo "=== Step 1: 下载 RDS CA 证书 ==="
curl -sSo ~/rds-ca.pem https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem
echo "  ✓ ~/rds-ca.pem"

echo ""
echo "=== Step 2: 安装 tiup + Dumpling ==="
if ! command -v tiup &>/dev/null; then
  curl --proto '=https' --tlsv1.2 -sSf https://tiup-mirrors.pingcap.com/install.sh | sh
  export PATH=$HOME/.tiup/bin:$PATH
fi
tiup install dumpling 2>/dev/null || true
echo "  ✓ Dumpling ready"

echo ""
echo "=== Step 3: Dumpling 导出 Aurora 快照 ==="
mkdir -p "$DUMP_DIR"
# --consistency=snapshot 在 InnoDB 上做一致性快照（不锁表，workload 继续跑）
# --threads=4 并行导出
tiup dumpling \
  -u "$AURORA_USER" -p "$AURORA_PASS" \
  -h "$AURORA_HOST" -P 3306 \
  --ca ~/rds-ca.pem \
  -B "$AURORA_DB" \
  --filetype sql \
  --consistency none \
  --threads 4 \
  -o "$DUMP_DIR"
echo "  ✓ 快照已导出到 $DUMP_DIR"
ls -lh "$DUMP_DIR"

echo ""
echo "=== Step 4: 在 TiDB Cloud 创建目标库 ==="
$TIDB_CLI -e "CREATE DATABASE IF NOT EXISTS $TIDB_DB;"
echo "  ✓ 数据库 $TIDB_DB 创建完毕"

echo ""
echo "=== Step 5: 导入 Schema（建表） ==="
for f in "$DUMP_DIR"/${AURORA_DB}.*-schema.sql; do
  tbl=$(basename "$f" | sed "s/${AURORA_DB}\.//; s/-schema\.sql//")
  echo "  CREATE TABLE $tbl ..."
  $TIDB_CLI "$TIDB_DB" < "$f"
done
echo "  ✓ Schema 导入完毕"

echo ""
echo "=== Step 6: 导入数据 ==="
for f in "$DUMP_DIR"/${AURORA_DB}.*.*.sql; do
  fname=$(basename "$f")
  echo "  Loading $fname ..."
  $TIDB_CLI "$TIDB_DB" < "$f"
done
echo "  ✓ 数据导入完毕"

echo ""
echo "=== Step 7: 验证行数一致性 ==="
printf "%-30s %12s %12s %8s\n" "Table" "Aurora" "TiDB" "Match"
printf "%-30s %12s %12s %8s\n" "-----" "------" "----" "-----"

all_match=true
for tbl in user_models article_models article_user_models comment_models \
           tag_models article_tags favorite_models follow_models; do
  a=$($AURORA_CLI "$AURORA_DB" -N -e "SELECT COUNT(*) FROM $tbl;" 2>/dev/null || echo "ERR")
  t=$($TIDB_CLI "$TIDB_DB" -N -e "SELECT COUNT(*) FROM $tbl;" 2>/dev/null || echo "ERR")
  ok="✓"
  [ "$a" != "$t" ] && ok="✗" && all_match=false
  printf "%-30s %12s %12s %8s\n" "$tbl" "$a" "$t" "$ok"
done

echo ""
if $all_match; then
  echo "✅ 全量迁移验证通过，Aurora 与 TiDB 数据一致"
else
  echo "⚠️  存在差异，请检查以上标 ✗ 的表"
fi

echo ""
echo "=== 完成 ==="
echo "  TiDB 连接串: mysql -u $TIDB_USER -h $TIDB_HOST -P $TIDB_PORT -D $TIDB_DB -p'$TIDB_PASS'"
echo ""
echo "  下一步（切换应用）："
echo "  1. 修改 EC2 上的 db.env，将 DSN 改为 TiDB Cloud"
echo "  2. docker compose -f docker-compose.aurora.yml restart backend"
echo "  3. 跑 workload 验证应用正常"
