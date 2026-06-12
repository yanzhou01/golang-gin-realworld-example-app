# Repro：Feature Branch（MySQL）开发 DDL 变更 → 合并到 master（TiDB）

本文档承接 [REPRO.md](./REPRO.md)：假设产品已经按照 REPRO.md 的方式跑在 TiDB 上（master 分支 = TiDB 生产环境），
现在要在 feature branch 上开发一个**带 DDL 变更的新功能**，并保证合并回 master 后这些 DDL 变更能安全应用到 TiDB。

## 场景

| | master | feature branch |
|---|---|---|
| DB | TiDB（生产） | MySQL（本地开发，启动快、调试方便） |
| Schema 管理 | GORM `AutoMigrate`（[hello.go](./hello.go) `Migrate()`） | 同上 |

核心问题：**branch 上对着 MySQL 写的 model 变更（= DDL），怎么保证 merge 后对 TiDB 也是兼容的？**

解决思路：在 merge 前用一次性的 TiDB 实例跑一遍同样的 `AutoMigrate`，提前暴露 DDL 不兼容问题，
而不是等部署到生产 TiDB 才发现。

## 目录

1. [前提条件](#1-前提条件)
2. [Step 1：从 master 切出 feature branch，本地用 MySQL 开发](#step-1)
3. [Step 2：新功能 — Bookmark（阅读清单），新增一张表](#step-2)
4. [Step 3：注册 AutoMigrate](#step-3)
5. [Step 4：加最小 API](#step-4)
6. [Step 5：本地用 MySQL 验证](#step-5)
7. [Step 6：merge 前的 TiDB DDL 兼容性 gate](#step-6)
8. [Step 7：提交 & 合并到 master](#step-7)
9. [Step 8：master（TiDB）上部署验证](#step-8)
10. [附：DDL 兼容性注意事项](#附ddl-兼容性注意事项)
11. [附：本次新增的基础设施](#附本次新增的基础设施)

---

## 1. 前提条件

- 已完成 REPRO.md 第 1-7 节（全栈跑通、单元测试、e2e 测试）
- master 分支已经是 TiDB 版（参考 REPRO.md 第 9-12 节的 `Dockerfile.tidb` / TiDB 版 `docker-compose.yml`）
- 本仓库已包含：
  - `hello.go` 的 `MIGRATE_ONLY` 模式
  - `docker-compose.migration.yml` 里的 `tidb`、`tidb-init`、`tidb-schema-check` 服务
  - `scripts/check-tidb-ddl.sh`

（这三项是本文档配套新增的基础设施，见文末 [附：本次新增的基础设施](#附本次新增的基础设施)）

---

## Step 1：从 master 切出 feature branch，本地用 MySQL 开发

```bash
git checkout main
git checkout -b feature/reading-list

# 本地开发环境用 MySQL（启动快）
docker compose up -d mysql
```

---

## Step 2：新功能 — Bookmark（阅读清单），新增一张表

在 [articles/models.go](./articles/models.go) 仿照 `FavoriteModel`（约第 31 行）新增：

```go
type BookmarkModel struct {
	gorm.Model
	Article        ArticleModel
	ArticleID      uint
	Bookmarked     ArticleUserModel
	BookmarkedByID uint
}
```

这会在数据库里新建一张 `bookmark_models` 表 —— 这就是本次的 DDL 变更。

---

## Step 3：注册 AutoMigrate

在 [hello.go](./hello.go) 的 `Migrate()` 里加一行：

```go
func Migrate(db *gorm.DB) {
	users.AutoMigrate()
	db.AutoMigrate(&articles.ArticleModel{})
	db.AutoMigrate(&articles.TagModel{})
	db.AutoMigrate(&articles.FavoriteModel{})
	db.AutoMigrate(&articles.ArticleUserModel{})
	db.AutoMigrate(&articles.CommentModel{})
	db.AutoMigrate(&articles.BookmarkModel{}) // 新增
}
```

---

## Step 4：加最小 API

在 [articles/routers.go](./articles/routers.go) 仿照 `ArticleFavorite` / `ArticleUnfavorite`（约第 171/187 行），
新增 `ArticleBookmark` / `ArticleUnbookmark`，并在 `ArticlesRegister`（约第 14 行）注册：

```go
router.POST("/:slug/bookmark", ArticleBookmark)
router.DELETE("/:slug/bookmark", ArticleUnbookmark)
```

具体实现复用 `favoriteBy` / `unFavoriteBy` 的写法，换成操作 `BookmarkModel`。

---

## Step 5：本地用 MySQL 验证

```bash
docker compose up -d --build backend

# 确认 AutoMigrate 跑过没报错
docker compose logs backend | grep -i migrat

# 确认新表已建出来
mysql -h 127.0.0.1 -P 3306 -u conduit -pconduit realworld -e "DESC bookmark_models;"

# 跑现有单元测试，确保没有破坏其他功能
docker run --rm -v "$(pwd)":/app -w /app golang:1.25 go test ./...
```

---

## Step 6：merge 前的 TiDB DDL 兼容性 gate

在 merge 到 master 之前，用一次性的 TiDB 实例跑同样的 `AutoMigrate`：

```bash
./scripts/check-tidb-ddl.sh
```

预期输出：

```
[check-tidb-ddl] OK - schema changes apply cleanly to TiDB.
[check-tidb-ddl] (TiDB instance left running; './scripts/check-tidb-ddl.sh down' to remove it)
```

验证表结构：

```bash
mysql -h 127.0.0.1 -P 4000 -u root realworld -e "DESC bookmark_models;"
mysql -h 127.0.0.1 -P 4000 -u root realworld -e "SHOW CREATE TABLE bookmark_models;"
```

用完清理掉这个临时 TiDB 实例：

```bash
./scripts/check-tidb-ddl.sh down
```

> **这一步能查出什么 / 查不出什么**
> - ✅ 能查出来：DDL 语法/特性 TiDB 不支持，AutoMigrate 直接报错（这种 DDL 不能合并）
> - ❌ 查不出来：语法合法但不是 TiDB 最佳实践的写法，例如 `bookmark_models.id` 会是
>   `bigint AUTO_INCREMENT PRIMARY KEY`（GORM `gorm.Model` 默认）。这条 DDL 在 TiDB 上能跑通，
>   但属于写热点（详见 [附：DDL 兼容性注意事项](#附ddl-兼容性注意事项)），需要人工 review 是否要改成
>   `AUTO_RANDOM` 或业务 ID。

---

## Step 7：提交 & 合并到 master

```bash
git add hello.go articles/models.go articles/routers.go
git commit -m "feat: add reading-list bookmark feature"

git checkout main
git merge feature/reading-list
```

---

## Step 8：master（TiDB）上部署验证

master 的 `db.env` 指向 `tidb:4000`。部署时可以用 `MIGRATE_ONLY` 把"跑 DDL"和"启动 HTTP 服务"解耦：

```bash
# 方式 A：作为部署流水线里的独立 migration job，先跑迁移再滚动更新 backend
DB_DSN="conduit:conduit@tcp(tidb:4000)/realworld?parseTime=true&charset=utf8mb4&loc=Local" \
MIGRATE_ONLY=true go run .

# 方式 B：backend 启动时自带 AutoMigrate（现有行为，无需额外操作）
docker compose up -d --build backend
```

因为 Step 6 已经在 TiDB 上跑过一遍同样的 `AutoMigrate`，这里应该是无缝的 ——
`bookmark_models` 表已存在，`AutoMigrate` 是幂等的（检测到表已存在会跳过 `CREATE TABLE`）。

---

## 附：DDL 兼容性注意事项

写 migration / model 时，以下几类在 MySQL 上能跑、但在 TiDB 上要特别注意：

- **AUTO_INCREMENT 主键**：语法合法但是写热点，推荐 `AUTO_RANDOM` 或业务 UUID/雪花 ID
  （kmap §3.1 Schema 设计原则）。GORM 不会自动生成 `AUTO_RANDOM`，需要单独写迁移 SQL。
- **外键约束**：TiDB 6.6 之前 FK 只解析不强制执行，新版本默认开启 `foreign_key_checks`。
- **同一条 ALTER TABLE 里多个 DDL 操作**（multi-schema-change）：TiDB 支持范围和 MySQL 不完全一致，
  建议拆成单条 `ALTER TABLE`。
- **字符集/collation**：统一 `utf8mb4`，避免 MySQL 默认的某些 collation TiDB 不支持。

### 为什么"新增型 DDL"对滚动发布是安全的

TiDB 的 DDL 是异步执行的（kmap §2.5 DDL 执行流程）：

```
TiDB Server 收到 DDL → 写入 DDL Job Queue（TiKV）
  → DDL Owner 异步执行（多版本 Schema 过渡）
  → 各节点通过 schema lease 定期更新版本
  → 完成后写入 History Queue
```

这套机制对应 F1 论文（kmap §17.1）的状态机 `absent → delete-only → write-only → public`
（Theorem 1），保证「at most two schema versions concurrently」。所以 `ADD COLUMN` / `ADD INDEX`
这类**新增型 DDL**，即使在 merge 后 master 处于新旧代码并存的滚动发布期间，也是安全的。

但 F1 论文 Claim 2 同时指出：直接 `DROP` 一个 public 的结构元素（列/索引）**不是
consistency-preserving 的**。如果未来的变更包含删列/改列语义，需要按 **expand-contract**
模式拆成多个 PR：

1. 先加新列（nullable / 带 default），新代码双写
2. 回填数据
3. 后续 PR 再 drop 旧列（此时新代码已经不依赖它了）

---

## 附：本次新增的基础设施

```
hello.go                     修改  ← 新增 MIGRATE_ONLY 模式：跑完 AutoMigrate 后直接退出，不启动 HTTP server
docker-compose.migration.yml 修改  ← 新增 tidb-schema-check 服务（一次性，对 TiDB 跑 AutoMigrate）
scripts/check-tidb-ddl.sh    新增  ← merge 前的 DDL 兼容性 gate 脚本
```

### `tidb-schema-check` 服务（[docker-compose.migration.yml](./docker-compose.migration.yml)）

复用 `backend` 镜像，`DB_DSN` 指向 `tidb:4000`，`MIGRATE_ONLY=true`，依赖 `tidb-init` 完成后一次性运行：

```yaml
tidb-schema-check:
  image: realworld-backend-backend
  environment:
    DB_DSN: "conduit:conduit@tcp(tidb:4000)/realworld?parseTime=true&charset=utf8mb4&loc=Local"
    MIGRATE_ONLY: "true"
  depends_on:
    tidb-init:
      condition: service_completed_successfully
  restart: "no"
```

### `scripts/check-tidb-ddl.sh`

```bash
./scripts/check-tidb-ddl.sh        # 构建 backend 镜像 + 对临时 TiDB 跑 AutoMigrate
./scripts/check-tidb-ddl.sh down   # 清理临时 TiDB 实例
```

`docker compose run --rm tidb-schema-check` 会按 `depends_on` 自动拉起 `tidb` → 等
`tidb-init` 跑完一次性建库建用户 → 再运行 schema check，退出码即代表 AutoMigrate 是否成功。
