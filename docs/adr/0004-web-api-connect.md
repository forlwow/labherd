# 0004. 前端 API 使用 connect-go，与 Agent 共用 proto

- 状态：提议
- 日期：2026-09-23

## 背景

Agent 与 Controller 之间使用 gRPC（见 0001）。前端（React + TypeScript）也需要调用 Controller 来查询节点、指标和进程。

问题在于：浏览器无法直接发起原生 gRPC 调用。前端需要一种它能用的协议，同时希望接口只定义一次，避免 proto 和前端类型两边分别维护、逐渐对不上。

## 备选方案

### A. 手写 REST（chi / echo）加 OpenAPI

- 优点：URL 风格直观，工具生态成熟。
- 缺点：接口在 proto 和 OpenAPI 中各定义一次，需要人工保持一致；前端类型要另外生成或手写。

### B. grpc-gateway

- 优点：从 proto 生成 REST 反向代理，URL 保持 RESTful。
- 缺点：需要在 proto 中编写 HTTP 映射注解；多一层代理和一套代码生成；错误映射需要额外处理。

### C. gRPC-Web 加 Envoy 代理

- 优点：前端直接使用 gRPC 语义。
- 缺点：需要额外部署 Envoy，运维负担大。

### D. connect-go（选定）

Connect 同一个 handler 可以同时支持 gRPC、gRPC-Web 和 Connect 协议（基于 HTTP 的 JSON 或二进制）。前端使用 `@connectrpc/connect-web` 调用，类型由 `protoc-gen-es` 从同一份 proto 生成。

## 决策

采用方案 D（待确认后改为"已接受"）：

- 面向前端的查询服务（如 `NodeService`）定义在 `api/proto/labherd/v1/` 中，由 connect-go 提供服务。
- 前端 TypeScript 客户端与类型由 `make proto` 统一生成，生成代码不手改。
- Agent 的双向流保持使用 gRPC 协议。Controller 需要启用 HTTP/2（h2c 或 TLS）以支持双向流。

## 后果

好处：

- 接口只在 proto 中定义一次，前后端类型由生成代码保证一致，`buf breaking` 同时保护两端。
- 不需要额外的代理进程。
- 可以直接用 curl 发送 JSON 进行调试。

代价：

- URL 形如 `POST /labherd.controller.v1.NodeService/ListNodes`，不是 RESTful 风格；如果将来要对外开放 API，可能需要另外提供 REST。
- 浏览器端不支持双向流，只支持一元调用和服务端流。M1 前端使用轮询，没有影响。
- 引入 buf 的 TypeScript 代码生成插件，需要写入 `buf.gen.yaml` 和 `make proto`。

待办：确认后更新本文状态，并在 `docs/design/api.md` 中说明服务划分。
