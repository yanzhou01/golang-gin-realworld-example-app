# Runbook：Aurora MySQL → TiDB Cloud 实时迁移完整复现步骤

## 环境变量配置

克隆本仓库后，先设置以下变量。**本地 Mac 和 EC2 上都需要 export**（SSH 不会自动传递本地变量）。

**本地 Mac（Step 1～3）：**
```bash
export SSH_KEY=~/Downloads/your-key.pem
export EC2_HOST=<your-ec2-public-dns>
export EC2_USER=ec2-user
export OPENAI_API_KEY=sk-proj-...
```

**EC2 上（Step 4～6，SSH 登录后执行）：**
```bash
export AURORA_HOST=<your-aurora-cluster-endpoint>
export AURORA_USER=admin
export AURORA_PASS=<aurora-password>

export TIDB_HOST=<your-tidb-cloud-endpoint>
export TIDB_PORT=4000
export TIDB_USER=root
export TIDB_PASS=<tidb-password>

# 便捷别名（Step 4～6 的命令均依赖这两个变量）
export AURORA="mysql -u $AURORA_USER -p$AURORA_PASS -h $AURORA_HOST --ssl-mode=REQUIRED --ssl-ca=~/rds-ca.pem"
export TIDB="mysql -u $TIDB_USER -h $TIDB_HOST -P $TIDB_PORT -p$TIDB_PASS"
```

> 建议将以上内容追加到 EC2 的 `~/.bash_profile`，避免重新 SSH 后需要重新设置。

---

## 架构概览

```
本地 Mac
  ├── deploy-aurora-ec2.sh     → 部署应用到 EC2
  ├── import_japanese_blogs.py → AI 生成日文博客
  └── workload.py (SCP到EC2)   → 读写压测

EC2
  ├── Docker Compose
  │     ├── backend  (Go/Gin)  →  Aurora MySQL
  │     └── frontend (React/nginx)
  ├── workload.py               → 模拟读写负载
  └── TiDB DM
        ├── dm-master :8261
        └── dm-worker :8262

Aurora MySQL (source)  ──DM CDC──→  TiDB Cloud (target)
```

---

## Step 1：部署应用到 EC2，连接 Aurora MySQL

**在本地 Mac 执行：**

```bash
bash deploy-aurora-ec2.sh
```

> `deploy-aurora-ec2.sh` 从环境变量读取 `EC2_HOST`、`SSH_KEY`、`AURORA_HOST`，运行前确保已 export（未设置会报错退出）。

**脚本做了什么：**
1. 在 EC2 上通过 `get.docker.com` 安装 Docker（跨发行版通用）
2. Clone 前后端 repo
3. `aurora-init.sh`：在 Aurora 上建库（`realworld`）和应用用户（`conduit`）
4. 启动 backend + frontend + seed（写入 8 个用户、30 篇文章）

**验证：**
```bash
curl http://$EC2_HOST:8080/api/ping/
# → {"message":"pong"}
```

前端访问：`http://$EC2_HOST:3001`

---

## Step 2：用 AI 导入 100 篇日文博客

```bash
pip3 install openai requests
python3 import_japanese_blogs.py
```

**说明：**
- 调用 GPT-4o 生成 10 批 × 10 篇日文文章（文化/美食/科技/旅游/动漫等主题）
- 轮流以 8 个种子用户身份通过 `POST /api/articles/` 写入 Aurora
- 全程打印进度 `[1/100] … [100/100]`

---

## Step 3：在 EC2 上跑读写负载

```bash
# 上传脚本
scp -i $SSH_KEY workload.py $EC2_USER@$EC2_HOST:~/

# 后台启动 4 小时（nohup 保证 SSH 断开后继续运行）
ssh -i $SSH_KEY $EC2_USER@$EC2_HOST \
  "nohup python3 -u ~/workload.py --duration 14400 > ~/workload.log 2>&1 &"
```

**查看进度：**
```bash
ssh -i $SSH_KEY $EC2_USER@$EC2_HOST "tail -f ~/workload.log"
```

```
[05:33:08] elapsed   1m  writes    193 (+193/min  err=0)  reads    2066 (+2066/min  err=0)  remaining 238m
```

**提前停止：**
```bash
ssh -i $SSH_KEY $EC2_USER@$EC2_HOST "pkill -f workload.py"
```

| 参数 | 默认值 | 说明 |
|---|---|---|
| `--duration` | 14400 | 运行秒数 |
| `--writers` | 4 | 写线程（POST /api/articles/） |
| `--readers` | 8 | 读线程（GET /api/articles） |

---

## Step 4：全量迁移 Aurora → TiDB Cloud（Dumpling）

```bash
ssh -i $SSH_KEY $EC2_USER@$EC2_HOST
```

> SSH 进入 EC2 后是全新 shell，**需要重新 export EC2 的环境变量**（见顶部"EC2 上"那块），或将其写入 `~/.bash_profile`。

### 4.1 安装工具 & 下载 RDS CA 证书

```bash
curl --proto '=https' --tlsv1.2 -sSf https://tiup-mirrors.pingcap.com/install.sh | sh
source ~/.bash_profile
tiup install dumpling

# Aurora SSL 证书
curl -sSo ~/rds-ca.pem https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem
```

### 4.2 Dumpling 导出快照

```bash
tiup dumpling \
  -u "$AURORA_USER" -p"$AURORA_PASS" \
  -h $AURORA_HOST -P 3306 \
  --ca ~/rds-ca.pem \
  -B realworld \
  --filetype sql \
  --consistency none \
  --threads 4 \
  -o ~/aurora-dump
```

> `--consistency none`：不锁表，workload 继续跑。

导出后**记录 binlog 位点**（Step 5 增量同步的起点）：

```bash
cat ~/aurora-dump/metadata
# SHOW MASTER STATUS:
#     Log: mysql-bin-changelog.000002   ← 记下这个值
#     Pos: 123456789                     ← 记下这个值
```

### 4.3 导入 TiDB Cloud

```bash
$TIDB -e "CREATE DATABASE IF NOT EXISTS realworld;"

for f in ~/aurora-dump/realworld.*-schema.sql; do
  $TIDB realworld < "$f"
done

for f in ~/aurora-dump/realworld.*.*.sql; do
  $TIDB realworld < "$f"
done
```

### 4.4 验证行数

```bash
for tbl in user_models article_models comment_models tag_models favorite_models follow_models; do
  a=$($AURORA realworld -N -e "SELECT COUNT(*) FROM $tbl;" 2>/dev/null)
  t=$($TIDB   realworld -N -e "SELECT COUNT(*) FROM $tbl;" 2>/dev/null)
  printf "%-30s Aurora=%-6s TiDB=%-6s %s\n" "$tbl" "$a" "$t" \
    "$( [ "$a" = "$t" ] && echo ✓ || echo ✗ )"
done
```

> ⚠️ workload 在跑时 article_models 可能有差异——DM 增量同步会补齐。

---

## Step 5：增量 CDC 同步（TiDB DM）

### 5.1 配置文件

**dm-topology.yaml**（单节点，部署在 EC2 本机）：
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

**dm-source.yaml**（Aurora 为 source）：
```yaml
source-id: "aurora-realworld"
flavor: "mysql"
enable-gtid: false   # Aurora 使用 file:position 模式

from:
  host: "<AURORA_HOST>"
  port: 3306
  user: "<AURORA_USER>"
  password: "<AURORA_PASS>"
  security:
    ssl-ca: "/home/ec2-user/rds-ca.pem"
```

**dm-task.yaml**（全量已完成，只做增量）：
```yaml
name: "realworld-live-migration"
task-mode: "incremental"

target-database:
  host: "<TIDB_HOST>"
  port: 4000
  user: "<TIDB_USER>"
  password: "<TIDB_PASS>"

mysql-instances:
  - source-id: "aurora-realworld"
    block-allow-list: "realworld-only"
    meta:
      binlog-name: "<来自 metadata 的 Log>"   # 例：mysql-bin-changelog.000001
      binlog-pos:  <来自 metadata 的 Pos>      # 例：123456789

block-allow-list:
  realworld-only:
    do-dbs: ["realworld"]
```

> `migration/` 目录下有对应的示例文件，填入真实值后使用。

### 5.2 部署 DM

```bash
# tiup 连 localhost 需要 SSH 密钥（已存在则跳过生成）
[ -f ~/.ssh/id_rsa ] || ssh-keygen -t rsa -b 2048 -N "" -f ~/.ssh/id_rsa -q
cat ~/.ssh/id_rsa.pub >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys

cat >> ~/.ssh/config << 'EOF'
Host 127.0.0.1
  IdentityFile ~/.ssh/id_rsa
  StrictHostKeyChecking no
  UserKnownHostsFile /dev/null
EOF
chmod 600 ~/.ssh/config

tiup install dm:v7.5.0 dmctl:v7.5.0
tiup dm deploy dm-realworld v7.5.0 ~/migration/dm-topology.yaml -y
tiup dm start dm-realworld
```

### 5.3 注册 Source 并启动任务

```bash
tiup dmctl --master-addr=127.0.0.1:8261 \
  operate-source create ~/migration/dm-source.yaml

tiup dmctl --master-addr=127.0.0.1:8261 \
  start-task ~/migration/dm-task.yaml
```

### 5.4 验证同步状态

```bash
tiup dmctl --master-addr=127.0.0.1:8261 query-status realworld-live-migration
```

正常关键字段：
```json
"stage": "Running",
"secondsBehindMaster": "0"
```

---

## Step 6：Switchover——应用切换到 TiDB Cloud

> 停机时间 < 1 分钟。

### 6.1 确认 DM 已追上（延迟为 0）

```bash
tiup dmctl --master-addr=127.0.0.1:8261 query-status realworld-live-migration
```

### 6.2 停止写入，等待最后 binlog 同步

```bash
pkill -f workload.py
sleep 8
```

### 6.3 最终行数一致性确认

```bash
for tbl in user_models article_models comment_models tag_models favorite_models follow_models; do
  a=$($AURORA realworld -N -e "SELECT COUNT(*) FROM $tbl;" 2>/dev/null)
  t=$($TIDB   realworld -N -e "SELECT COUNT(*) FROM $tbl;" 2>/dev/null)
  printf "%-30s Aurora=%-6s TiDB=%-6s %s\n" "$tbl" "$a" "$t" \
    "$( [ "$a" = "$t" ] && echo ✓ || echo ✗ )"
done
```

### 6.4 修改 db.env，指向 TiDB Cloud

```bash
printf 'DB_DSN=%s:%s@tcp(%s:%s)/realworld?parseTime=true&charset=utf8mb4&loc=Local&tls=skip-verify\n' \
  "$TIDB_USER" "$TIDB_PASS" "$TIDB_HOST" "$TIDB_PORT" \
  > ~/front-backend-project/golang-gin-realworld-example-app/db.env
```

### 6.5 重建 backend 容器（必须用 up -d，restart 不重读 env_file）

```bash
cd ~/front-backend-project/golang-gin-realworld-example-app
sudo docker compose -f docker-compose.aurora.yml up -d backend
sleep 12
curl http://localhost:8080/api/ping/   # → {"message":"pong"}
```

### 6.6 停止 DM

```bash
tiup dmctl --master-addr=127.0.0.1:8261 stop-task realworld-live-migration
tiup dm stop dm-realworld -y
```

### 6.7 验证新写入只进 TiDB，不进 Aurora

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/users/login \
  -H 'Content-Type: application/json' \
  -d '{"user":{"email":"<seed-user-email>","password":"<seed-user-password>"}}' \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["user"]["token"])')

curl -s -X POST http://localhost:8080/api/articles/ \
  -H "Content-Type: application/json" \
  -H "Authorization: Token $TOKEN" \
  -d '{"article":{"title":"Switchover test","description":"after cutover","body":"Written to TiDB Cloud.","tagList":["tidb"]}}'

# TiDB 有 → Aurora 无 = 切换成功
$TIDB   realworld -e 'SELECT id,title FROM article_models WHERE title="Switchover test";'
$AURORA realworld -e 'SELECT id,title FROM article_models WHERE title="Switchover test";'
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
          修改 db.env → docker compose up -d backend
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
tiup dm stop    dm-realworld -y              # 关闭集群
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

| 组件 | 作用 |
|---|---|
| **Dumpling** | 导出一致性快照 + 记录对应的 binlog position |
| **DM dm-master** | 任务调度与协调中心 |
| **DM dm-worker** | 连接 Aurora 读 binlog，转换后写入 TiDB |

---

## 常见问题

| 问题 | 原因 | 解决 |
|---|---|---|
| `docker compose restart` 后 DSN 未变 | `restart` 不重读 `env_file` | 改用 `docker compose up -d` |
| Aurora `log_bin=OFF` 但 DM 可以工作 | Aurora 的 `log_bin` 变量报 OFF 是已知行为，实际 binlog 是开启的（文件名为 `mysql-bin-changelog.*`）| 用 `SHOW MASTER STATUS` 确认 |
| Dumpling `--consistency snapshot` panic | Dumpling 对非 TiDB 数据库的快照实现有 bug | 改用 `--consistency none` |
| DM deploy `tui.id_read_failed` | tiup dm 内部用 SSH 连 localhost，需要密钥 | 生成 `~/.ssh/id_rsa` 并加入 `authorized_keys` |
| DM source config 字段错误 | DM v2+ 的 `host/port/user/password` 要嵌套在 `from:` 下 | 参考 Step 5.1 的格式 |
