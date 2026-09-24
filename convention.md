# labherd 开发约定

本文件是项目结构与命名的唯一依据。能由工具自动检查的规则写在配置文件里（见文末），本文件只写工具查不到的部分。修改约定需要单独提 PR（标题如 `docs: update conventions`），并在描述中说明原因。

## 1. 目录职责

| 目录 | 放什么 | 不放什么 |
|---|---|---|
| `api/proto/labherd/<服务>/v1/` | 接口定义（.proto） | 任何实现代码 |
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

proto 文件所在目录必须与 package 一致：`labherd.agent.v1` 放在 `api/proto/labherd/agent/v1/`。由 buf lint 检查。

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

## 7. 开发流程

### 7.1 总览

每项工作都走同一个循环，不存在例外：

```
Issue → 开分支 → 小步提交 → 推送 → 创建 PR → CI 全绿 → 自查 → Squash 合并 → 同步本地
```

main 分支受保护：禁止直接推送、禁止强制推送、禁止删除，只能通过 PR 合并。

### 7.2 分支

- 命名：`<类型>/<简短描述>`，类型与提交类型一致，描述用小写短横线。
  - 示例：`feat/agent-gpu-collector`、`fix/controller-reconnect-storm`、`ci/fix-tidy-check`
- 一个分支只做一件事，对应一个 Issue。
- 分支从最新的 main 开出，合并后即删除，不长期保留。

```bash
git switch main && git pull
git switch -c feat/agent-cpu-collector
```

### 7.3 提交

**格式**：Conventional Commits，`<类型>(<范围>): <描述>`，描述用英文小写开头、祈使句、不加句号。

- 类型：`feat` 新功能、`fix` 修复、`refactor` 重构、`perf` 性能、`test` 测试、`docs` 文档、`build` 构建、`ci` 流水线、`chore` 杂项
- 范围：`agent` `controller` `web` `api` `db` `deploy` `ci` `docs`，跨多个范围时可省略
- 示例：`feat(agent): report per-process gpu memory`

**时机**：每完成一个能用一句话说清的小步骤就提交，如"定义接口""补充测试""测试通过"。提交前代码必须能编译。提交信息里需要用"和"字时，说明应该拆成两次提交。

**做法**：

```bash
make all             # 格式化、检查、测试、构建
git add -p           # 逐块确认，避免带入调试代码和无关改动
git commit -m "feat(agent): add cpu collector interface"
```

新建的文件需要先 `git add <文件>`，`git commit -a` 不会包含未跟踪的文件。

功能分支内的提交会在合并时被压缩，格式可以适当宽松；**PR 标题必须严格符合格式**，因为它会成为 main 上的最终提交信息。

### 7.4 推送

以下任一情况都应推送：

- 每天结束工作前（至少一次，作为备份）；
- 希望 CI 运行时（可先创建草稿 PR）；
- 准备提交审查时。

首次推送需要关联远程分支，之后直接 `git push`：

```bash
git push -u origin feat/agent-cpu-collector
```

### 7.5 创建 PR

```bash
gh pr create --title "feat(agent): add cpu collector" --web
```

`--web` 会在浏览器中打开创建页面，描述框已填入模板。不使用 `--fill`，它会跳过模板。需要提前运行 CI 时，在页面上选择创建草稿（Draft）PR。

要求：

- 标题符合 7.3 的格式，由 `pr-title` 检查。
- 按模板填写"做了什么、为什么、怎么验证的"，对应 Issue 写 `Closes #编号`，合并后 Issue 自动关闭。
- 自查清单逐项确认后打勾（`- [ ]` 改为 `- [x]`）。
- 改动控制在约 400 行以内，超出时拆分成多个 PR。生成代码和迁移脚本不计入。

### 7.6 CI 检查

合并前必须通过以下四项检查：

| 检查 | 来源 | 内容 |
|---|---|---|
| `lint` | `ci.yml` | golangci-lint 代码检查与格式检查 |
| `test` | `ci.yml` | go mod tidy 检查、带竞争检测的单元测试、覆盖率 |
| `build` | `ci.yml` | 构建全部程序，lint 与 test 通过后才执行 |
| `check` | `pr-title.yml` | PR 标题格式 |

检查失败时，在 PR 页面点击 Details 查看日志，在本地用相同命令复现（`make lint`、`make test`），修复后推送到**同一分支**，PR 与 CI 会自动更新，不要重新创建 PR。

```bash
gh pr checks --watch      # 在终端查看检查进度
```

### 7.7 合并

仅允许 **Squash and merge**。合并前确认 CI 全绿、自查清单已完成、合并框中的提交标题正确。

**方式一：命令行（推荐）**，会自动切回 main、拉取最新代码并删除本地和远程分支：

```bash
gh pr merge --squash --delete-branch
```

**方式二：网页点击合并**，之后需要手动清理：

```bash
git switch main && git pull
git branch -D feat/agent-cpu-collector
```

squash 合并后 Git 无法识别分支已被合并，因此删除本地分支需要用 `-D`。

### 7.8 同步与冲突

**分支落后于 main 时**，用 rebase 保持线性历史：

```bash
git fetch origin
git rebase origin/main
# 如有冲突：修改文件 → git add <文件> → git rebase --continue
git push --force-with-lease
```

强制推送只允许用 `--force-with-lease`，且只在自己的功能分支上使用。

**在 GitHub 网页上修改过分支后**（网页编辑文件、采纳审查建议、点击 Update branch），远程会多出提交，回到本地继续工作前先执行 `git pull`。

### 7.9 禁止事项

- 直接推送或强制推送到 main。
- 提交构建产物（`bin/`）、覆盖率文件、`.env`、密钥与密码。
- 手动修改 `gen/` 下的生成代码，或修改已合入 main 的数据库迁移文件。
- 在一个 PR 中混入不相关的改动。
- 为了让 CI 通过而关闭检查规则或删除测试。确需调整规则时单独提 PR 并说明原因。

### 7.10 发版

- 版本号采用语义化版本 `vMAJOR.MINOR.PATCH`，1.0 之前接口可能不兼容。
- 在 main 的最新提交上打 tag 并推送：

```bash
git switch main && git pull
git tag v0.1.0
git push origin v0.1.0
```

- 镜像命名：`ghcr.io/forlwow/labherd-<组件>:<版本>`，如 `labherd-controller:v0.1.0`。

## 8. 文档命名

- ADR：`docs/adr/<4位序号>-<短横线描述>.md`，如 `0001-agent-push-model.md`。
- 测试报告：`docs/testing/reports/<年-月>-<短横线描述>.md`，如 `2026-10-network-outage.md`。
- 其余文档文件名一律小写短横线。

## 9. 由工具强制执行的规则

| 规则 | 配置文件 | 执行时机 |
|---|---|---|
| 缩进、换行、编码 | `.editorconfig` | 编辑器保存时 |
| Go 格式 | `.golangci.yml`（gofumpt、goimports） | `make fmt`、CI |
| Go 静态检查与依赖方向 | `.golangci.yml` | `make lint`、CI |
| Proto 风格与兼容性 | `buf.yaml` | `make proto-lint`、CI |
| 前端格式与检查 | `web/eslint.config.js`、Prettier | `npm run lint`、CI |
| PR 标题 | `.github/workflows/pr-title.yml` | 创建或编辑 PR 时 |
| 合并前检查 | `.github/workflows/ci.yml` + 仓库规则集 | 每次推送到 PR 时 |
