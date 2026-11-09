# web 组件说明

LabHerd 的前端：React + TypeScript，用 Vite 构建，组件库为 Mantine，数据请求用 TanStack Query。选型理由见 [ADR 0005](../adr/0005-web-stack-and-embedding.md)，源码在 `web/`。

## 环境

Node.js 22 及以上（推荐用 fnm 安装 LTS 版本）。依赖全部写在 `package.json` 中：

```bash
cd web
npm ci
```

## 开发

先在本地启动 Controller（默认 HTTP 端口 8080），再启动开发服务器：

```bash
npm run dev    # http://localhost:5173，/api 自动转发给 Controller
```

Controller 不在本机或不是默认端口时，用环境变量指定：

```bash
LABHERD_CONTROLLER_HTTP=http://10.0.0.5:8080 npm run dev
```

## 常用命令

| 命令             | 作用                            |
| ---------------- | ------------------------------- |
| `npm run lint`   | ESLint 检查与 Prettier 格式检查 |
| `npm run format` | 用 Prettier 格式化全部文件      |
| `npm test`       | 运行单元测试（Vitest）          |
| `npm run build`  | 类型检查并构建到 `dist/`        |

在仓库根目录也可以用 `make web-lint`、`make web-test`、`make web-build`。

## 目录

```
web/src/
  components/   通用组件（大驼峰文件名）
  pages/        页面，每个页面一个小写短横线目录
  test/         测试工具与全局配置
  routes.tsx    路由表
  theme.ts      Mantine 主题
```

## 打包进 Controller

前端不单独部署，构建产物嵌入 `labherd-controller`，与 `/api` 使用同一个端口（`-http-listen`，默认 `:8080`）：

```bash
make release    # npm ci → vite build → go build（带 webui 构建标签）
./bin/labherd-controller
# 浏览器打开 http://<controller-ip>:8080/
```

- 嵌入代码在 `web/embed.go`（`webui` 构建标签）与 `web/noembed.go`；`make build` 发现 `web/dist/index.html` 存在时自动加上该标签。
- 没有先构建前端时，`make build` 仍能成功，访问页面只会看到一张说明页，接口不受影响。只改后端时不需要安装 Node。
- 路由回退由 `internal/controller/webui` 处理：`/nodes/xxx` 这类前端路由返回 `index.html`，`assets/` 下的文件长期缓存。
