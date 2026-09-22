# labherd 开发约定

本文件是项目结构与命名的唯一依据。能由工具自动检查的规则写在配置文件里（见文末），本文件只写工具查不到的部分。修改约定需要单独提 PR，并在 PR 描述中说明原因。

## 1. 目录职责

| 目录 | 放什么 | 不放什么 |
|---|---|---|
| `api/proto/labherd/v1/` | 接口定义（.proto） | 任何实现代码 |
| `cmd/<程序名>/` | 只有 `main.go`：读配置、组装模块、启动 | 业务逻辑 |
| `internal/agent/` | Agent 专属逻辑 | — |
| `internal/controller/` | Controller 专属逻辑 | — |
| `internal/common/` | 两端共用的基础能力：日志、配置、版本 | 业务逻辑 |
| `gen/` | 由 `make proto` 生成的代码 | 手写代码，禁止手改 |
| `migrations/` | 数据库迁移脚本 | — |
| `test/` | 跨组件的集成测试、压测工具 | 单元测试（放在源码旁） |
| `web/` | 前端 | — |
| `build/` | Dockerfile | — |
| `deploy/` | compose、systemd、ansible | — |
| `docs/` | 项目级文档 | 组件的运行说明（写在组件 README） |

依赖方向规则：

- `internal/agent` 与 `internal/controller` **互相不得导入**，二者只通过 `gen/` 中的接口通信。
- `internal/common` 不得导入 `agent` 或 `controller`。
- 以上由 golangci-lint 的 depguard 检查。

## 2. Go 命名

| 对象 | 规则 | 示例 |
|---|---|---|
| 包名 | 全小写、单个单词、不用下划线和复数 | `collector`、`registry` |
| 文件名 | 小写 + 下划线 | `gpu_collector.go`、`gpu_collector_test.go` |
| 导出标识符 | 大驼峰 | `NodeStatus` |
| 内部标识符 | 小驼峰 | `lastSeen` |
| 缩写词 | 整体大写或整体小写 | `GPUID`、`gpuID`、`HTTPServer`，不写 `GpuId` |
| 接口 | 单方法接口用 -er 结尾 | `Collector`、`Reporter` |
| 错误变量 | `Err` 开头 | `ErrNodeNotFound` |
| 错误信息 | 小写开头、不加句号 | `fmt.Errorf("read meminfo: %w", err)` |
| 接收者 | 1～2 个字母，同一类型保持一致 | `func (r *Registry) ...` |
| 测试函数 | `Test<被测对象>_<场景>` | `TestRegistry_MarkLost` |
| 构造函数 | `New<类型>` | `NewRegistry(...)` |

避免包名与内容重复：写 `registry.New()`，不写 `registry.NewRegistry()`（单类型包时）。

## 3. Proto 命名

| 对象 | 规则 | 示例 |
|---|---|---|
| package | `labherd.<服务>.v<版本>` | `labherd.agent.v1` |
| message | 大驼峰 | `GpuSnapshot` |
| 字段 | 小写下划线 | `memory_used_bytes` |
| enum 值 | 全大写下划线，带类型名前缀，0 值为 UNSPECIFIED | `NODE_STATE_UNSPECIFIED = 0; NODE_STATE_ONLINE = 1;` |
| rpc | 动词 + 名词 | `RegisterNode`、`ListNodes` |
| 请求/响应 | `<Rpc名>Request` / `<Rpc名>Response` | `ListNodesRequest` |

兼容性规则：已发布的字段编号不得修改或复用；删除字段时用 `reserved` 占住编号。由 `buf breaking` 检查。

## 4. 数据库命名

- 表名：小写下划线、复数，如 `nodes`、`gpu_reservations`。
- 列名：小写下划线；主键 `id`；外键 `<表单数>_id`，如 `node_id`。
- 每张表带 `created_at`、`updated_at`（`timestamptz`）。
- 索引：`idx_<表>_<列>`；唯一约束：`uq_<表>_<列>`。
- 迁移文件：`<4位序号>_<描述>.up.sql` / `.down.sql`，如 `0003_add_gpu_owner.up.sql`。已合入 main 的迁移文件不得修改，只能新增。

## 5. 指标、日志、配置

- **指标名**遵循 Prometheus 规范：`labherd_<子系统>_<名称>_<单位>`，单位用基本单位。
  - `labherd_gpu_memory_used_bytes`、`labherd_gpu_utilization_ratio`（0～1）、`labherd_agent_reconnects_total`
- **日志字段**小写下划线：`node_id`、`gpu_index`、`trace_id`。日志消息用英文小写短句。
- **环境变量**统一前缀 `LABHERD_`：`LABHERD_CONTROLLER_ADDR`、`LABHERD_LOG_LEVEL`。

## 6. 前端命名

| 对象 | 规则 | 示例 |
|---|---|---|
| 组件文件与组件名 | 大驼峰 | `NodeCard.tsx` |
| Hook | `use` 开头 | `useNodeList.ts` |
| 其他 TS 文件 | 小驼峰 | `formatBytes.ts` |
| 测试 | 与被测文件同名 | `NodeCard.test.tsx` |
| 页面目录 | 小写短横线 | `pages/node-detail/` |

## 7. Git 与发布

- **分支**：`<类型>/<简短描述>`，如 `feat/agent-gpu-collector`、`fix/controller-reconnect-storm`。
- **提交**：Conventional Commits，`<类型>(<范围>): <描述>`。
  - 类型：`feat` `fix` `refactor` `perf` `test` `docs` `build` `ci` `chore`
  - 范围：`agent` `controller` `web` `api` `db` `deploy` `ci` `docs`
  - 示例：`feat(agent): report per-process gpu memory`
- **版本号**：语义化版本 `vMAJOR.MINOR.PATCH`；1.0 之前接口可能不兼容。
- **镜像**：`ghcr.io/forlwow/labherd-<组件>:<版本>`，如 `labherd-controller:v0.1.0`。

## 8. 文档命名

- ADR：`docs/adr/<4位序号>-<短横线描述>.md`，如 `0001-agent-push-model.md`。
- 测试报告：`docs/testing/reports/<年-月>-<短横线描述>.md`，如 `2026-10-network-outage.md`。
- 其余文档文件名一律小写短横线。

## 9. 由工具强制执行的规则

| 规则 | 配置文件 | 执行时机 |
|---|---|---|
| 缩进、换行、编码 | `.editorconfig` | 编辑器保存时 |
| Go 格式 | gofumpt | `make fmt`、CI |
| Go 静态检查与依赖方向 | `.golangci.yml` | `make lint`、CI |
| Proto 风格与兼容性 | `buf.yaml` | `make proto-lint`、CI |
| 前端格式与检查 | `web/eslint.config.js`、Prettier | `npm run lint`、CI |
| 提交信息 | commitlint | CI 检查 PR 标题 |
| 代码归属 | `.github/CODEOWNERS` | PR |
