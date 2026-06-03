# 🙌 RealWorld example app 🍰 Feature-Sliced Design

A modern implementation of the [RealWorld](https://github.com/gothinkster/realworld) app built with React, TypeScript, React Router, React Query, and Zod.

![Realworld example app](./logo.gif)

## About the Project

This project is an educational and demonstration Medium-clone built with the Feature-Sliced Design (FSD) architectural approach and modern frontend tools. It is suitable for learning, experimentation, and as a template for large-scale applications.

![Preview][preview-domain]

## Tech Stack

- **React 19**
- **TypeScript**
- **React Router 7**
- **TanStack React Query 5**
- **Zod 4**
- **Webpack 5**
- **Jest**
- **Testing Library**
- **MSW**
- **ESLint**
- **Prettier**
- **Sass**
- **Orval**

## Project Structure

- `src/app` — application shell, root router, layout, providers, root error handling
- `src/pages` — page-scoped route modules and page UI
- `src/shared` — reusable API layer, utilities, router helpers, and shared UI

## Architecture Notes

The codebase uses a page-scoped structure rather than a full multi-layer FSD tree.

- The root router is defined in `src/app/browser-router.tsx`.
- Pages expose route objects from `src/pages/*/*.route.ts`.
- Route modules are lazy-loaded to keep the initial bundle smaller.
- Data fetching is handled in route loaders with React Query.
- Mutations are handled in route actions and usually validate `formData` with Zod before calling the API.
- Shared routing helpers, API utilities, and common UI live in `src/shared`.

## Runtime Patterns

- **Lazy route modules**: page components, loaders, and actions are loaded on demand.
- **Route loaders**: async page data is prepared through React Router loaders backed by React Query.
- **Route actions**: form submissions and mutations are handled declaratively through React Router actions.
- **Auth middleware**: route middleware restores the current user and protects auth-only flows.
- **Validation before API calls**: form data is validated with Zod-based helpers before mutations run.
- **Optimistic UI**: interactive actions such as follow and favorite use fetcher-driven optimistic state.
- **Error boundaries**: the root router renders a dedicated fallback for route and auth failures.

## Development Workflow

- Webpack Dev Server is used for local development.
- Husky hooks are configured for pre-commit and pre-push checks.
- Generated API code is produced from OpenAPI through Orval and then normalized with a local Zod conversion step.
- Root `Dockerfile` and `nginx.conf` are used for the containerized frontend build.

### Dependency Graph

![Dependency Graph][dependency-graph-domain]

### Bundle Analyze

![Bundle Analyze][bundle-analyze-domain]

## E2E 测试（Playwright）

端到端测试位于 `e2e/` 目录，使用 [Playwright](https://playwright.dev/) + Chromium 对完整的前后端栈进行真实浏览器测试。

### 前提

后端服务（`:8080`）和前端服务（`:4100`）均需已启动：

```bash
# 启动后端（realworld-backend 目录）
docker compose up -d

# 构建并启动前端（本目录，API_URL 须指向已运行的后端）
API_URL=http://localhost:8080 WEB_SERVER_PORT_EXTERNAL=4100 WEB_SERVER_PORT_INTERNAL=80 \
  docker compose up -d
```

### 安装与运行

```bash
cd e2e
npm install          # 安装 @playwright/test
npx playwright install chromium   # 首次使用需下载浏览器

# 运行全部测试（headless，约 26 秒）
npx playwright test

# 带 UI 模式（可视化调试）
npx playwright test --ui

# 有头模式（可看到浏览器窗口）
npx playwright test --headed
```

### 测试覆盖范围（29 个测试，100% 通过）

| 文件 | 测试数 | 场景 |
|------|--------|------|
| `tests/auth.spec.ts` | 9 | 注册、重复用户报错、登录、密码错误报错、登出 |
| `tests/articles.spec.ts` | 7 | 创建 / 查看 / 编辑 / 删除文章，删除后 404 |
| `tests/comments.spec.ts` | 4 | 发评论、未登录提示、删自己的评论、权限隔离 |
| `tests/profile.spec.ts` | 9 | 查看资料、关注 / 取关、个人 Feed、收藏、设置更新 |
| **合计** | **29** | **29/29 全部通过** |

测试报告（HTML）生成后可通过 `npx playwright show-report` 查看。

---

## Scripts

- `yarn start` — starts the development server.
- `yarn build:dev` — builds the app in development mode.
- `yarn build:prod` — builds the production bundle.
- `yarn analyze:prod` — builds the production bundle with bundle analyzer enabled.
- `yarn test` — runs Jest tests.
- `yarn eslint` — lints and auto-fixes files under `src`.
- `yarn prettier` — formats the repository with Prettier.
- `yarn graph` — generates a dependency graph preview for `src`.[^1]
- `yarn generate` — regenerates API artifacts from the OpenAPI schema.
- `yarn zod:mini` — post-processes generated Zod artifacts.
- `yarn format:generated` — formats generated API files.
- `yarn prepare` — installs Husky hooks.

## Run

Install dependencies:

```bash
yarn install
```

Run locally:

```bash
yarn start
```

- Frontend: `http://localhost:30401`
- Backend API: `http://localhost:30400/api`

[^1]:
    This assumes the GraphViz `dot` command is available - on most linux and
    comparable systems this will be. In case it's not, see
    [GraphViz' download page](https://www.graphviz.org/download/) for instructions
    on how to get it on your machine.

## Docker Compose

Start locally with Docker:

```bash
docker compose --env-file .env.compose up -d --build
```

- Frontend: <http://localhost:30401>
- Backend API is expected separately at <http://localhost:30400>

For backend integration, see [node-express-realworld-example-app](https://github.com/yurisldk/node-express-realworld-example-app).

Stop:

```bash
docker compose --env-file .env.compose down
```

## Deployment

Build image:

```bash
docker build --build-arg API_URL=http://localhost:30400/api -t realworld-frontend .
```

Released images are published to GitHub Container Registry:

```bash
docker pull ghcr.io/yurisldk/realworld:2.0.0
docker pull ghcr.io/yurisldk/realworld:latest
```

Release images are built with `API_URL=http://localhost:30400/api` by default.

[dependency-graph-domain]: ./dependency-graph-preview.svg
[preview-domain]: ./preview.gif
[bundle-analyze-domain]: ./bundle-analyze.png
