# 0005. 前端使用 Mantine + TanStack Query，构建产物嵌入 Controller

- 状态：已接受
- 日期：2026-11-09

## 背景

M1 需要一个监控页面：节点卡片、GPU 与磁盘占用、节点详情。语言已定为 React + TypeScript（见 0004 的背景）。还需要决定：

1. 用哪个组件库，以较少的代码做出流畅、美观的界面；
2. 怎样定时拉取数据，并在 Controller 不可达时保留旧数据；
3. 前端怎样部署到实验室服务器。

实验室服务器环境不统一，部署要尽量少依赖：最好不需要在服务器上安装 Node、nginx 或 Docker。

## 决策

- 构建：Vite。
- 组件库：Mantine，默认暗色主题。
- 数据请求：TanStack Query 定时轮询 `/api/v1/`，不引入 Redux 等全局状态库。
- 路由：React Router。
- 测试：Vitest + Testing Library。
- 部署：`vite build` 的产物通过 `go:embed` 打进 `labherd-controller`，与 `/api` 同一个端口提供；开发时用 Vite 开发服务器代理 `/api`。

## 理由

- Mantine 自带表格、进度条、徽章、折叠等后台常用组件和暗色主题，不需要手写 CSS；shadcn/ui + Tailwind 更灵活，但组件要自己组装，代码量更大。
- 监控页的数据几乎全部来自服务端，TanStack Query 提供轮询、缓存、重试，以及失败时保留上一次数据，正好对应校园网断开的场景。
- 嵌入 Controller 后，部署只有一个二进制文件；前后端版本天然一致，也没有跨域问题。Node 只在构建时需要。

## 放弃的方案

- **shadcn/ui + Tailwind**：见上，组装成本高。
- **单独的 nginx 容器提供静态文件**：服务器上需要 nginx 或 Docker，并且要配置反向代理；M1 只有一个 Controller，收益不大。
- **Next.js**：不需要服务端渲染，静态页面即可。
