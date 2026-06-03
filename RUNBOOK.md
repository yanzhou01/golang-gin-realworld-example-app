# Runbook：Aurora MySQL → TiDB Cloud 实时迁移完整复现步骤

## 架构概览

```
本地 Mac
  ├── deploy-aurora-ec2.sh    → 部署应用到 EC2
  ├── import_japanese_blogs.py → AI 生成日文博客
  └── workload.py (SCP到EC2)  → 读写压测

EC2 (ap-northeast-1)
  ├── Docker Compose
  │     ├── backend  (Go/Gin)  →  Aurora MySQL
  │     └── frontend (React/nginx)
  ├── workload.py              → 模拟读写负载
  └── TiDB DM
        ├── dm-master :8261
        └── dm-worker :8262

Aurora MySQL (source)  ──DM CDC──→  TiDB Cloud (target)
```

## 资源清单

| 资源 | 地址 / 说明 |
|---|---|
| EC2 SSH Key | `~/Downloads/yanzhou.pem` |
| EC2 Host | `ec2-13-231-221-52.ap-northeast-1.compute.amazonaws.com` |
| Aurora Host | `yanzhouw-newhire-test-source.cluster-cdximlzkzbgd.ap-northeast-1.rds.amazonaws.com` |
| Aurora 用户 | `admin` / `password` |
| TiDB Cloud Host | `privatelink-20616390.mczxoo2az8r7.clusters.tidb-cloud.com:4000` |
| TiDB 用户 | `root` / `password` |
| 应用种子账号 | `alice@example.com` / `password123`（共8个用户）|

---

## Step 1：部署应用到 EC2，连接 Aurora MySQL

**在本地 Mac 执行：**

```bash
cd /Users/yanzhou/selflearning/front-backend-project
bash deploy-aurora-ec2.sh
```

**脚本做了什么：**
1. 在 EC2 上通过 `get.docker.com` 安装 Docker
2. Clone 前后端 repo
3. `aurora-init.sh`：在 Aurora 上建库（`realworld`）和应用用户（`conduit`）
4. 启动 backend + frontend + seed（写入8用户、30篇文章）

**验证：**
```bash
curl http://ec2-13-231-221-52.ap-northeast-1.compute.amazonaws.com:8080/api/ping/
# → {"message":"pong"}
```

前端访问：`http://ec2-13-231-221-52.ap-northeast-1.compute.amazonaws.com:3001`

---

## Step 2：用 AI 导入 100 篇日文博客

**在本地 Mac 执行：**

```bash
cd /Users/yanzhou/selflearning/front-backend-project
pip3 install openai requests
OPENAI_API_KEY=sk-proj-xxx python3 import_japanese_blogs.py
```

**说明：**
- 调用 GPT-4o 生成10批 × 10篇日文文章（文化/美食/科技/旅游/动漫等主题）
- 轮流以8个种子用户身份通过 `POST /api/articles/` 写入 Aurora
- 全程打印进度 `[1/100] … [100/100]`

---

## Step 3：在 EC2 上跑 4 小时读写负载

**在本地 Mac 执行：**

```bash
# 上传脚本
scp -i ~/Downloads/yanzhou.pem workload.py \
  ec2-user@ec2-13-231-221-52.ap-northeast-1.compute.amazonaws.com:~/

# 后台启动（nohup 保证 SSH 断开后继续运行）
ssh -i ~/Downloads/yanzhou.pem \
  ec2-user@ec2-13-231-221-52.ap-northeast-1.compute.amazonaws.com \
  "nohup python3 -u ~/workload.py --duration 14400 > ~/workload.log 2>&1 &"
```

**查看进度：**
```bash
ssh -i ~/Downloads/yanzhou.pem \
  ec2-user@ec2-13-231-221-52.ap-northeast-1.compute.amazonaws.com \
  "tail -f ~/workload.log"
```

每分钟输出一行：
```
[05:33:08] elapsed   1m  writes    193 (+193/min  err=0)  reads    2066 (+2066/min  err=0)  remaining 238m
```

**提前停止：**
```bash
ssh ... "pkill -f workload.py"
```

| 参数 | 默认值 | 说明 |
|---|---|---|
| `--duration` | 14400 | 运行秒数 |
| `--writers` | 4 | 写线程（POST /api/articles/） |
| `--readers` | 8 | 读线程（GET /api/articles） |

---

## Step 4：全量迁移 Aurora → TiDB Cloud（Dumpling）

**登录 EC2 后执行：**

```bash
ssh -i ~/Downloads/yanzhou.pem \
  ec2-user@ec2-13-231-221-52.ap-northeast-1.compute.amazonaws.com
```

### 4.1 安装工具 & 下载 RDS CA 证书

```bash
# tiup + Dumpling（已装则跳过）
curl --proto '=https' --tlsv1.2 -sSf https://tiup-mirrors.pingcap.com/install.sh | sh
source ~/.bash_profile
tiup install dumpling

# RDS CA 证书（Aurora SSL 需要）
curl -sSo ~/rds-ca.pem https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem
```

### 4.2 Dumpling 导出快照

```bash
tiup dumpling \
  -u admin -p password \
  -h yanzhouw-newhire-test-source.cluster-cdximlzkzbgd.ap-northeast-1.rds.amazonaws.com \
  -P 3306 \
  --ca ~/rds-ca.pem \
  -B realworld \
  --filetype sql \
  --consistency none \
  --threads 4 \
  -o ~/aurora-dump
```

> `--consistency none`：不锁表，workload 继续跑。
> 导出完成后查看 `cat ~/aurora-dump/metadata`，记录 binlog 位点（用于 Step 5 CDC 起点）。

### 4.3 导入 TiDB Cloud

```bash
TIDB="mysql -u root -h privatelink-20616390.mczxoo2az8r7.clusters.tidb-cloud.com -P 4000 -ppassword"

# 建库
$TIDB -e "CREATE DATABASE IF NOT EXISTS realworld;"

# 导入 schema（建表）
for f in ~/aurora-dump/realworld.*-schema.sql; do
  echo "Schema: $f"
  $TIDB realworld < "$f"
done

# 导入数据
for f in ~/aurora-dump/realworld.*.*.sql; do
  echo "Data: $f"
  $TIDB realworld < "$f"
done
```

### 4.4 验证行数

```bash
AURORA="mysql -u admin -ppassword \
  -h yanzhouw-newhire-test-source.cluster-cdximlzkzbgd.ap-northeast-1.rds.amazonaws.com \
  --ssl-mode=REQUIRED --ssl-ca=~/rds-ca.pem"

for tbl in user_models article_models comment_models tag_models favorite_models follow_models; do
  a=$($AURORA realworld -N -e "SELECT COUNT(*) FROM $tbl;" 2>/dev/null)
  t=$($TIDB   realworld -N -e "SELECT COUNT(*) FROM $tbl;" 2>/dev/null)
  printf "%-30s Aurora=%-6s TiDB=%-6s %s\n" "$tbl" "$a" "$t" \
    "$( [ "$a" = "$t" ] && echo ✓ || echo ✗ )"
done
```

> ⚠️ workload 在跑时 article_models 可能有几十行差异，这是正常的——DM 的增量同步会在 Step 5 补齐。

---

## Step 5：增量 CDC 同步（TiDB DM）

### 5.1 准备配置文件

**查看并记录 Dumpling 导出时的 binlog 位点：**
```bash
cat ~/aurora-dump/metadata
# Started dump at: 2026-06-03 05:23:22
# SHOW MASTER STATUS:
#     Log: mysql-bin-changelog.000002
#     Pos: 587153
```

**dm-topology.yaml**（单节点，本机部署）：
```yaml
global:
  user: "ec2-user"
  deploy_dir: "/home/ec2-user/tidb-dm/deploy"
  data_dir:   "/home/ec2-user/tidb-dm/data"

master_servers:
  - host: 127.0.0.1
    port: 8261
    peer_port: 8291
    log_dir: "/home/ec2-user/tidb-dm/logs/master"

worker_servers:
  - host: 127.0.0.1
    port: 8262
    log_dir: "/home/ec2-user/tidb-dm/logs/worker"
```

**dm-source.yaml**（Aurora 作为 source）：
```yaml
source-id: "aurora-realworld"
flavor: "mysql"
enable-gtid: false   # Aurora 不用 GTID 模式

from:
  host: "yanzhouw-newhire-test-source.cluster-cdximlzkzbgd.ap-northeast-1.rds.amazonaws.com"
  port: 3306
  user: "admin"
  password: "password"
  security:
    ssl-ca: "/home/ec2-user/rds-ca.pem"
```

**dm-task.yaml**（只做增量，全量已完成）：
```yaml
name: "realworld-live-migration"
task-mode: "incremental"

target-database:
  host: "privatelink-20616390.mczxoo2az8r7.clusters.tidb-cloud.com"
  port: 4000
  user: "root"
  password: "password"

mysql-instances:
  - source-id: "aurora-realworld"
    block-allow-list: "realworld-only"
    meta:
      binlog-name: "mysql-bin-changelog.000002"  # 来自 metadata
      binlog-pos:  587153                         # 来自 metadata

block-allow-list:
  realworld-only:
    do-dbs: ["realworld"]
```

### 5.2 部署 DM

```bash
# 让 tiup 能 SSH 连接 localhost
ssh-keygen -t rsa -b 2048 -N "" -f ~/.ssh/id_rsa -q
cat ~/.ssh/id_rsa.pub >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys

cat >> ~/.ssh/config << 'EOF'
Host 127.0.0.1
  IdentityFile ~/.ssh/id_rsa
  StrictHostKeyChecking no
  UserKnownHostsFile /dev/null
EOF
chmod 600 ~/.ssh/config

# 安装 DM
tiup install dm:v7.5.0 dmctl:v7.5.0

# 部署
tiup dm deploy dm-realworld v7.5.0 ~/migration/dm-topology.yaml -y

# 启动
tiup dm start dm-realworld
```

### 5.3 注册 Source 并启动任务

```bash
# 注册 Aurora 为 DM source
tiup dmctl --master-addr=127.0.0.1:8261 \
  operate-source create ~/migration/dm-source.yaml

# 启动增量同步任务
tiup dmctl --master-addr=127.0.0.1:8261 \
  start-task ~/migration/dm-task.yaml
```

### 5.4 验证同步状态

```bash
tiup dmctl --master-addr=127.0.0.1:8261 \
  query-status realworld-live-migration
```

正常输出关键字段：
```json
"stage": "Running",
"unit": "Sync",
"secondsBehindMaster": "0",   ← 延迟为 0
"synced": false                ← workload 持续写入时始终 false，正常
```

**实时行数对比：**
```bash
for tbl in user_models article_models comment_models tag_models; do
  a=$($AURORA realworld -N -e "SELECT COUNT(*) FROM $tbl;" 2>/dev/null)
  t=$($TIDB   realworld -N -e "SELECT COUNT(*) FROM $tbl;" 2>/dev/null)
  printf "%-30s Aurora=%-6s TiDB=%-6s\n" "$tbl" "$a" "$t"
done
```

---

## Step 6：Switchover——应用切换到 TiDB Cloud

> 在 DM 延迟降到 0、行数一致后执行。整个切换过程停机时间 < 1 分钟。

### 6.1 确认 DM 已追上

```bash
tiup dmctl --master-addr=127.0.0.1:8261 query-status realworld-live-migration
# 确认：
#   "secondsBehindMaster": "0"
#   "stage": "Running"
```

### 6.2 停止写入流量（短暂停机）

```bash
# 停止 workload
pkill -f workload.py

# 等几秒让最后几条 binlog 同步完
sleep 5
```

### 6.3 最终行数一致性确认

```bash
AURORA="mysql -u admin -ppassword \
  -h yanzhouw-newhire-test-source.cluster-cdximlzkzbgd.ap-northeast-1.rds.amazonaws.com \
  --ssl-mode=REQUIRED --ssl-ca=~/rds-ca.pem"
TIDB="mysql -u root -ppassword \
  -h privatelink-20616390.mczxoo2az8r7.clusters.tidb-cloud.com -P 4000"

for tbl in user_models article_models comment_models tag_models favorite_models follow_models; do
  a=$($AURORA realworld -N -e "SELECT COUNT(*) FROM $tbl;" 2>/dev/null)
  t=$($TIDB   realworld -N -e "SELECT COUNT(*) FROM $tbl;" 2>/dev/null)
  printf "%-30s Aurora=%-6s TiDB=%-6s %s\n" "$tbl" "$a" "$t" \
    "$( [ "$a" = "$t" ] && echo ✓ || echo ✗ )"
done
# 所有表都 ✓ 后再继续
```

### 6.4 修改应用 DSN，指向 TiDB Cloud

```bash
cat > /home/ec2-user/front-backend-project/golang-gin-realworld-example-app/db.env << 'EOF'
DB_DSN=root:password@tcp(privatelink-20616390.mczxoo2az8r7.clusters.tidb-cloud.com:4000)/realworld?parseTime=true&charset=utf8mb4&loc=Local&tls=skip-verify
EOF
```

### 6.5 重启 backend

```bash
cd /home/ec2-user/front-backend-project/golang-gin-realworld-example-app
sudo docker compose -f docker-compose.aurora.yml restart backend

# 等待 healthy
sudo docker compose -f docker-compose.aurora.yml ps
```

### 6.6 验证应用正常运行在 TiDB Cloud 上

```bash
# API 健康检查
curl http://localhost:8080/api/ping/
# → {"message":"pong"}

# 读取文章（数据来自 TiDB）
curl -s http://localhost:8080/api/articles?limit=3 | python3 -m json.tool | grep title
```

### 6.7 停止 DM（迁移完成）

```bash
tiup dmctl --master-addr=127.0.0.1:8261 stop-task realworld-live-migration
tiup dm stop dm-realworld
```

### 6.8 恢复写入流量

```bash
nohup python3 -u ~/workload.py --duration 3600 > ~/workload-tidb.log 2>&1 &
# workload 现在写入的是 TiDB Cloud
```

---

## 完整迁移时间线

```
t=0       应用跑在 Aurora，workload 开始
          │
t=+Xmin   Dumpling 快照导出（不锁表，业务不停）
          记录 binlog position P
          │
t=+Ymin   全量数据导入 TiDB Cloud 完成
          │
t=+Zmin   DM 启动，从位点 P 追增量 binlog
          secondsBehindMaster → 0（追上）
          │
t=cutover 停 workload（< 1 min）
          确认两端行数一致
          修改 db.env → 重启 backend
          停 DM
          │
t=cutover 应用运行在 TiDB Cloud ✓
+1min     恢复 workload
```

---

## DM 常用运维命令

```bash
DMCTL="tiup dmctl --master-addr=127.0.0.1:8261"

$DMCTL query-status                          # 所有任务状态
$DMCTL query-status realworld-live-migration # 指定任务详情
$DMCTL pause-task  realworld-live-migration  # 暂停
$DMCTL resume-task realworld-live-migration  # 恢复
$DMCTL stop-task   realworld-live-migration  # 停止

tiup dm display dm-realworld                 # 集群节点状态
tiup dm stop    dm-realworld                 # 关闭集群
tiup dm start   dm-realworld                 # 启动集群
```

---

## 核心概念：为什么需要 binlog position

```
时间轴：
────────────────────────────────────────────────────────────▶
          │                                │
    Dumpling 开始                    Dumpling 结束
    记录位点 P                       数据文件写完
          │
          └── DM 从位点 P 开始读 binlog
              把快照之后的所有写操作追到 TiDB
              = 全量数据 + 增量 CDC = 零数据丢失
```

**三个核心组件：**

| 组件 | 作用 |
|---|---|
| **Dumpling** | 导出一致性快照 + 记录对应的 binlog position |
| **DM dm-master** | 任务调度与协调中心 |
| **DM dm-worker** | 连接 Aurora 读 binlog，转换后写入 TiDB |
