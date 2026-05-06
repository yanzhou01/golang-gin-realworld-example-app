# ![RealWorld Example App](logo.png)


[![CI](https://github.com/gothinkster/golang-gin-realworld-example-app/actions/workflows/ci.yml/badge.svg)](https://github.com/gothinkster/golang-gin-realworld-example-app/actions/workflows/ci.yml)
[![Coverage Status](https://coveralls.io/repos/github/gothinkster/golang-gin-realworld-example-app/badge.svg?branch=main)](https://coveralls.io/github/gothinkster/golang-gin-realworld-example-app?branch=main)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://github.com/gothinkster/golang-gin-realworld-example-app/blob/main/LICENSE)
[![GoDoc](https://godoc.org/github.com/gothinkster/golang-gin-realworld-example-app?status.svg)](https://godoc.org/github.com/gothinkster/golang-gin-realworld-example-app)

> ### Golang/Gin codebase containing real world examples (CRUD, auth, advanced patterns, etc) that adheres to the [RealWorld](https://github.com/gothinkster/realworld) spec and API.


This codebase was created to demonstrate a fully fledged fullstack application built with **Golang/Gin** including CRUD operations, authentication, routing, pagination, and more.

## Recent Updates

This project has been modernized with the following updates:
- **Go 1.21+**: Updated from Go 1.15 to require Go 1.21 or higher
- **GORM v2**: Migrated from deprecated jinzhu/gorm v1 to gorm.io/gorm v2
- **JWT v5**: Updated from deprecated dgrijalva/jwt-go to golang-jwt/jwt/v5 (fixes CVE-2020-26160)
- **Validator v10**: Updated validator tags and package to match gin v1.10.0
- **Latest Dependencies**: All dependencies updated to their 2025 production-stable versions
- **RealWorld API Spec Compliance**:
  - `GET /profiles/:username` now supports optional authentication (anonymous access allowed)
  - `POST /users/login` returns 401 Unauthorized on failure (previously 403)
  - `GET /articles/feed` registered as dedicated authenticated route
  - `DELETE /articles/:slug` and `DELETE /articles/:slug/comments/:id` return empty response body

## Test Coverage

The project maintains high test coverage across all core packages:

| Package | Coverage |
|---------|----------|
| `articles` | 93.4% |
| `users` | 99.5% |
| `common` | 85.7% |
| **Total** | **90.0%** |

To generate a coverage report locally, run:
```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

## Dependencies (2025 Stable Versions)

| Package | Version | Release Date | Known Issues |
|---------|---------|--------------|--------------|
| [gin-gonic/gin](https://github.com/gin-gonic/gin) | v1.10.0 | 2024-05 | None; v1.11 has experimental HTTP/3 support |
| [gorm.io/gorm](https://gorm.io/) | v1.25.12 | 2024-08 | None; v1.30+ has breaking changes |
| [golang-jwt/jwt/v5](https://github.com/golang-jwt/jwt) | v5.2.1 | 2024-06 | None; v5.3 only bumps Go version requirement |
| [go-playground/validator/v10](https://github.com/go-playground/validator) | v10.24.0 | 2024-12 | None; v10.30+ requires Go 1.24 |
| [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto) | v0.32.0 | 2025-01 | None; keep updated for security fixes |
| [gorm.io/driver/sqlite](https://github.com/go-gorm/sqlite) | v1.5.7 | 2024-09 | None; requires cgo; use glebarez/sqlite for pure Go |
| [gosimple/slug](https://github.com/gosimple/slug) | v1.15.0 | 2024-12 | None |
| [stretchr/testify](https://github.com/stretchr/testify) | v1.10.0 | 2024-10 | None; v2 still in development |


# Directory structure
```
.
├── gorm.db
├── hello.go
├── common
│   ├── utils.go        //small tools function
│   └── database.go     //DB connect manager
├── users
|   ├── models.go       //data models define & DB operation
|   ├── serializers.go  //response computing & format
|   ├── routers.go      //business logic & router binding
|   ├── middlewares.go  //put the before & after logic of handle request
|   └── validators.go   //form/json checker
├── ...
...
```

# Getting started

## 快速启动（推荐）：Docker Compose 全栈

> 前提：安装 [Docker Desktop](https://www.docker.com/products/docker-desktop/)，无需本地 Go 环境。

```bash
# 1. 启动 MySQL + 后端 API + 前端 + 自动写入 demo 数据（一条命令）
docker compose up --build -d

# 2. 打开浏览器
open http://localhost:3000        # 前端界面
# 后端 API: http://localhost:8080/api
```

启动顺序由 Docker Compose 自动编排：
1. **MySQL 8.0** 启动并通过健康检查
2. **backend**（Go/Gin）连接 MySQL，GORM AutoMigrate 建表
3. **seed**（一次性容器）调用 API 写入 demo 数据后自动退出
4. **frontend**（React/nginx）提供 Web 界面

### Demo 数据

| 数据类型 | 数量 |
|---------|------|
| 用户 | 8 |
| 文章 | 30 |
| 标签 | 42 |
| 评论 | 20 |
| 收藏 | 29 |
| 关注关系 | 15 |

登录账号：`alice@example.com` / `bobby@example.com` 等，密码统一 `password123`

### Docker Compose 常用命令

```bash
# 查看容器状态
docker compose ps

# 查看 seed 执行日志
docker compose logs seed

# 查看后端日志
docker compose logs -f backend

# 停止并清除数据卷（重置数据）
docker compose down -v

# 停止但保留数据
docker compose stop
```

---

## 本地开发（需要 Go 1.25+）

### 安装依赖

```bash
go mod download
```

### 方式一：SQLite（零依赖，适合快速调试）

```bash
go run hello.go
# 服务启动在 http://localhost:8080/api
# 数据库文件：./data/gorm.db（自动创建）
```

### 方式二：连接 MySQL（需先启动 mysql 容器）

```bash
# 只启动 MySQL
docker compose up -d mysql

# 设置 DB_DSN 环境变量后启动服务
DB_DSN="conduit:conduit@tcp(localhost:3306)/realworld?parseTime=true&charset=utf8mb4&loc=Local" \
  go run hello.go
```

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | `8080` | 监听端口 |
| `GIN_MODE` | `debug` | `debug` 或 `release` |
| `DB_DSN` | 空 | MySQL DSN，设置后使用 MySQL，否则使用 SQLite |
| `DB_PATH` | `./data/gorm.db` | SQLite 文件路径（仅 `DB_DSN` 未设置时生效） |

### 写入 demo 数据（本地后端运行时）

```bash
go run ./seed/
# 默认连接 http://localhost:8080/api
# 自定义地址：go run ./seed/ http://your-api-host/api
```

---

## 单元测试

```bash
# 运行所有测试（使用 SQLite，无需外部依赖）
go test ./...

# 带覆盖率
go test ./... -cover

# 生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

### 测试覆盖率

| Package | Coverage |
|---------|----------|
| `articles` | 93.4% |
| `users` | 99.5% |
| `common` | 85.7% |
| **Total** | **90.0%** |

---

## MySQL → TiDB 迁移练习

本项目设计为 MySQL → TiDB 迁移的练手 baseline：

```bash
# 修改 docker-compose.yml 中 backend 的 DB_DSN，
# 将 mysql:3306 改为 TiDB 地址，重启 backend 即可：
DB_DSN="user:pass@tcp(tidb-host:4000)/realworld?parseTime=true&charset=utf8mb4&loc=Local"

# 用单元测试验证功能正确性
go test ./...

# 重新运行 seed 验证端到端流程
docker compose restart seed
docker compose logs seed
```

---

## API 测试（Postman / Newman）

```bash
# 运行官方 RealWorld API 测试集合
bash scripts/run-api-tests.sh
```
