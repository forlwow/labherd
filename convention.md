# labherd 开发约定

本文件是项目结构、命名与开发流程的唯一依据。能由工具自动检查的规则写在配置文件里（见第 9 节），本文件只写工具查不到的部分。修改约定需要单独提 PR（标题如 `docs: update conventions`），并在描述中说明原因。

## 1. 目录职责

| 目录 | 放什么 | 不放什么 |
|---|---|---|
| `api/proto/labherd/<服务>/v1/` | 接口定义（.proto）及其注释 | 任何实现代码 |
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
| `docs/` | 项目级文档、ADR、组件运行说明、测试报告（见第 8 节） | 接口字段说明（写在 .proto 注释中） |
| `.github/` | CI 工作流、Issue 模板、PR 模板 | — |

依赖方向规则：

- `internal/agent` 与 `internal/controller` **互相不得导入**，二者只通过 `gen/` 中的接口通信。
- `internal/common` 不得导入 `agent` 或 `controller`。
- 以上由 golangci-lint 的 depguard 检查。

## 2. Go 命名

### 2.1 标识符

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

### 2.2 变量与常量

总原则：**作用域越小，名字越短；作用域越大，名字越具体。** 名字表达"是什么"，不重复类型。

| 场景 | 规则 | 示例 |
|---|---|---|
| 作用域 | 几行内用完的变量用短名；包级或跨函数的用完整名 | 循环 `i`；包级 `defaultReportInterval` |
| 惯用名 | 以下固定用法不另起名 | `ctx context.Context`、`err error`、`cfg Config`、`t *testing.T`、`b *testing.B`、`mu sync.Mutex`、`wg sync.WaitGroup`、`ok`（comma-ok） |
| 集合 | 切片用复数；map 用 `<值>By<键>` | `nodes`、`nodeByID`、`userByUID` |
| 不带类型后缀 | 名字中不出现类型 | `nodes`，不写 `nodeList`、`nodeSlice`、`nameStr` |
| 布尔 | 读起来是一个判断 | `online`、`isLost`、`hasGPU`、`enabled` |
| 时间 | 时长用 `time.Duration`、时刻用 `time.Time`，名字不带单位 | `timeout`、`lastSeen`，不写 `timeoutSec int` |
| 其他数值 | 名字带单位，与指标、proto 字段一致 | `usedBytes`、`utilRatio`、`tempCelsius` |
| 常量 | 与变量规则相同，不用全大写下划线 | `maxRetries`、`DefaultPort`，不写 `MAX_RETRIES` |
| 枚举常量 | 类型名作前缀 + `iota`，零值表示未知 | `StateUnknown`、`StateOnline` |
| 通道 | 以传递的内容命名；只作信号用时叫 `done` 或 `stop` | `reports`、`done` |
| 错误 | 局部用 `err`；需要同时保留多个时加前缀 | `readErr`、`closeErr` |
| 不遮蔽包名 | 变量不与导入的包同名 | `reg := registry.New()`，不写 `registry := registry.New()` |
| Getter | 不加 `Get` 前缀 | `Name()`，不写 `GetName()` |

生成代码（`gen/`）中的命名（如 `GpuSnapshot`、`GetMemoryUsedBytes()`）由生成器决定，不受本节约束。

## 3. Proto 命名

| 对象 | 规则 | 示例 |
|---|---|---|
| package | `labherd.<服务>.v<版本>` | `labherd.agent.v1` |
| service | 大驼峰，以 `Service` 结尾 | `AgentService` |
| message | 大驼峰 | `GpuSnapshot` |
| 字段 | 小写下划线，数值字段带单位 | `memory_used_bytes` |
| enum 值 | 全大写下划线，带类型名前缀，0 值为 UNSPECIFIED | `NODE_STATE_UNSPECIFIED = 0; NODE_STATE_ONLINE = 1;` |
| rpc | 动词 + 名词 | `RegisterNode`、`ListNodes` |
| 请求/响应 | `<Rpc名>Request` / `<Rpc名>Response` | `ListNodesRequest` |

proto 文件所在目录必须与 package 一致：`labherd.agent.v1` 放在 `api/proto/labherd/agent/v1/`。由 buf lint 检查。

兼容性规则：已发布的字段编号不得修改或复用；删除字段时用 `reserved` 占住编号和名字；准备淘汰的字段先标记 `[deprecated = true]` 并注释原因。不兼容的修改新开 `v2` 包。由 `buf breaking` 检查。

接口文档即 proto 注释：每个 service、rpc、message 和字段都写注释，说明代码表达不了的内容——单位与范围、谁在什么时候发送、字段缺失时的含义、废弃原因。不在其他文档中重复列出字段。

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

### 6.1 文件与目录

| 对象 | 规则 | 示例 |
|---|---|---|
| 组件文件与组件名 | 大驼峰 | `NodeCard.tsx` |
| Hook | `use` 开头 | `useNodeList.ts` |
| 其他 TS 文件 | 小驼峰 | `formatBytes.ts` |
| 测试 | 与被测文件同名 | `NodeCard.test.tsx` |
| 页面目录 | 小写短横线 | `pages/node-detail/` |

### 6.2 代码中的名字

| 对象 | 规则 | 示例 |
|---|---|---|
| 变量、函数 | 小驼峰 | `nodes`、`formatBytes` |
| 类型、接口 | 大驼峰，不加 `I` 前缀 | `NodeStatus`，不写 `INodeStatus` |
| 模块级常量 | 全大写下划线 | `REFRESH_INTERVAL_MS` |
| 布尔 | `is` / `has` / `can` 开头 | `isLoading`、`hasGpu` |
| 数值 | 名字带单位（TS 没有时长类型，时间也要带） | `timeoutMs`、`usedBytes` |
| 事件 | props 用 `on<事件>`，处理函数用 `handle<事件>` | `onSelect`、`handleSelect` |

## 7. 开发流程

### 7.1 总览

每项工作都走同一个循环，不存在例外：

```
Issue（挂到里程碑）→ 开分支 → 小步提交 → 推送 → 创建 PR → CI 全绿 → 自查 → Squash 合并（关闭 Issue）→ 同步本地
```

- **Issue** 描述要做什么、为什么、做到什么程度；不含代码。
- **PR** 交付代码，合并时通过 `Closes #编号` 关闭 Issue。
- **里程碑**（M1～M4）把 Issue 分组，GitHub 自动统计完成比例。

main 分支受保护：禁止直接推送、禁止强制推送、禁止删除，只能通过 PR 合并。

### 7.2 里程碑与 Issue

#### 里程碑

路线图文档只写规划、目标与完成标准，**进度以 GitHub Milestone 为准**，文档中链接到对应里程碑，不在文档里维护勾选状态。

```bash
# 创建（描述中写完成标准）
gh api repos/forlwow/labherd/milestones -f title="M1 监控" -f description="完成标准：..."

# 查看进度
gh issue list --milestone "M1 监控"
gh issue list --milestone "M1 监控" --state closed
```

判断里程碑是否完成只看完成标准是否全部满足，不看 Issue 数量。完成后打版本 tag（见 7.11）并关闭里程碑。

#### 标签

| 标签 | 用途 |
|---|---|
| `feature` | 新功能 |
| `bug` | 已有功能出错 |
| `task` | 不直接增加功能的工程工作：CI、重构、部署脚本 |
| `docs` | 文档 |
| `question` | 需要讨论、尚未决定的事 |
| `agent` `controller` `web` `api` `db` `deploy` | 所属组件，与提交范围一致 |

```bash
gh label create feature --color 0E8A16 --description "新功能"
```

#### 创建

- 每个 Issue 的粒度为 1～3 天能完成、对应 PR 在约 400 行以内；更大的拆成子 Issue（在网页 Issue 右侧的 Sub-issues 中创建）。
- 按模板（`.github/ISSUE_TEMPLATE/`）填写三栏：**要做什么**、**完成标准**、**不做什么**。完成标准会成为 PR 中"怎么验证的"。
- 必须带类型标签和组件标签，并挂到里程碑。
- 开发中发现的额外工作，**新建 Issue 记录**，不要顺手加进当前分支。

```bash
gh issue create --title "Agent: CPU/内存/磁盘采集器" \
  --label feature,agent --milestone "M1 监控"
# 不带参数时交互式填写；加 --web 在浏览器中按模板填写
```

#### 开发中

```bash
gh issue edit 5 --add-assignee @me          # 认领
gh issue develop 5 --name feat/agent-host-collector --checkout   # 开分支并关联 Issue（先切回最新的 main）
gh issue comment 5 --body "..."             # 记录进展
```

Issue 评论区作为工作日志：记录查到的关键信息、遇到的阻碍、计划的调整及原因。需要讨论的决定先在 Issue 中讨论，结论写入 ADR（见 8.3）。

#### 关闭

- **完成**：PR 描述写 `Closes #编号`（修复 bug 时写 `Fixes #编号`），合并时自动关闭。一个 Issue 由多个 PR 完成时，前面的 PR 写 `Refs #编号`，最后一个写 `Closes`。
- **不做了 / 重复 / 已由其他 PR 完成**：手动关闭，必须写明原因。

```bash
gh issue close 5 --reason "not planned" --comment "原因……"
gh issue close 6 --reason "completed" --comment "已在 #4 中完成"
gh issue reopen 5                           # 关错时重新打开
```

#### 日常查看

```bash
gh issue list --assignee @me     # 手上的工作，同时进行的不超过 2 个
gh issue view 5                  # 详情与评论
gh issue view 5 --web
```

### 7.3 分支

- 命名：`<类型>/<简短描述>`，类型与提交类型一致，描述用小写短横线。
  - 示例：`feat/agent-gpu-collector`、`fix/controller-reconnect-storm`、`ci/fix-tidy-check`
- 一个分支只做一件事，对应一个 Issue。
- 分支从最新的 main 开出，合并后即删除，不长期保留。

```bash
git switch main && git pull
git switch -c feat/agent-cpu-collector
# 或者：gh issue develop <编号> --name feat/agent-cpu-collector --checkout
```

### 7.4 提交

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

### 7.5 推送

以下任一情况都应推送：

- 每天结束工作前（至少一次，作为备份）；
- 希望 CI 运行时（可先创建草稿 PR）；
- 准备提交审查时。

首次推送需要关联远程分支，之后直接 `git push`：

```bash
git push -u origin feat/agent-cpu-collector
```

### 7.6 创建 PR

PR 是把分支合并进 main 的申请，每个分支只创建一次；之后的推送会自动更新同一个 PR。

```bash
gh pr create --title "feat(agent): add cpu collector" --web
gh pr ready          # 草稿 PR 完成后标记为可审查
```

`--web` 会在浏览器中打开创建页面，描述框已填入模板。不使用 `--fill`，它会跳过模板。需要提前运行 CI 时，在页面上选择创建草稿（Draft）PR。

要求：

- 标题符合 7.4 的格式，由 `pr-title` 检查。
- 按模板填写"做了什么、为什么、怎么验证的"；关联 Issue（`Closes #编号` 或 `Refs #编号`，见 7.2）；涉及设计决策的链接对应 ADR。
- 改动使文档过时的，在**同一个 PR** 中修改文档（见 8.2）。
- 自查清单逐项确认后打勾（`- [ ]` 改为 `- [x]`）。
- 改动控制在约 400 行以内，超出时拆分成多个 PR。生成代码和迁移脚本不计入。

### 7.7 CI 检查

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

### 7.8 合并

仅允许 **Squash and merge**。合并前确认 CI 全绿、自查清单已完成、合并框中的提交标题正确。GitHub 会在提交标题末尾自动附加 `(#PR编号)`，据此可从 main 上的任意提交追溯到 PR、Issue 与 ADR。

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

功能未全部完成也可以合并，前提是 main 始终能编译、能运行，已有功能不受影响。

### 7.9 同步与冲突

**分支落后于 main 时**，用 rebase 保持线性历史：

```bash
git fetch origin
git rebase origin/main
# 如有冲突：修改文件 → git add <文件> → git rebase --continue
git push --force-with-lease
```

强制推送只允许用 `--force-with-lease`，且只在自己的功能分支上使用。

**在 GitHub 网页上修改过分支后**（网页编辑文件、采纳审查建议、点击 Update branch），远程会多出提交，回到本地继续工作前先执行 `git pull`。

### 7.10 禁止事项

- 直接推送或强制推送到 main。
- 提交构建产物（`bin/`）、覆盖率文件、`.env`、密钥与密码。
- 手动修改 `gen/` 下的生成代码，或修改已合入 main 的数据库迁移文件。
- 在一个 PR 中混入不相关的改动。
- 合并使文档过时、却未同步修改文档的 PR。
- 为了让 CI 通过而关闭检查规则或删除测试。确需调整规则时单独提 PR 并说明原因。

### 7.11 发版

- 版本号采用语义化版本 `vMAJOR.MINOR.PATCH`，1.0 之前接口可能不兼容。版本号只来源于 git tag，构建时由 Makefile 注入，代码中不写死。
- 在 main 的最新提交上打 tag 并推送，然后生成发版说明：

```bash
git switch main && git pull
git tag v0.1.0
git push origin v0.1.0
gh release create v0.1.0 --generate-notes
```

- 镜像命名：`ghcr.io/forlwow/labherd-<组件>:<版本>`，如 `labherd-controller:v0.1.0`。

## 8. 文档

### 8.1 放在哪里

| 文档 | 位置 | 何时编写 |
|---|---|---|
| 项目简介、快速开始 | 根目录 `README.md` | 仓库骨架 PR；安装或启动方式变化时更新 |
| 路线图 | `docs/roadmap.md` | 规划变化时更新；进度见 GitHub Milestone |
| 整体架构 | `docs/architecture.md` | 编码前；组件或数据流变化时更新。不写具体字段 |
| 设计决策 | `docs/adr/` | 做出重要决定时，在实现之前或与实现同一个 PR |
| 组件运行说明 | `docs/components/<组件>.md` | 组件能运行的 PR；配置项、启动参数变化时更新 |
| 接口说明 | `.proto` 文件注释 | 定义或修改接口时 |
| 代码说明 | 导出标识符的 godoc 注释 | 编写或修改代码时 |
| 测试报告 | `docs/testing/reports/` | 完成断网演练、压测等之后；不修改，每次写新的 |
| 开发约定 | 本文件 | 规则变化时，单独提 PR |

原则：经常变化的内容写在离代码最近的地方，只写一份，不在多处复述。

### 8.2 同步规则

**哪个 PR 让某份文档过时，就由这个 PR 负责修改它。** 提 PR 前依次确认：

1. 对应哪个 Issue？没有就先建一个。
2. 有没有"选 A 还是 B"的决定？有则需要 ADR。
3. 这个改动让哪些文档说错了？（可用 `git grep <旧名字>` 查找残留）在本 PR 中修改。
4. Issue 是否已全部完成？是则 `Closes`，否则 `Refs`，剩余部分建新 Issue。

只修正文档错误时，单独提 `docs:` PR，不混入功能分支。

### 8.3 ADR

以下情况需要写 ADR：存在多个可行方案、需要从中选择，且以后可能被问"为什么不用 X"。

每篇 ADR 包含：状态、日期、相关 Issue、背景、决定、理由（考虑过的方案）、后果。

状态流转：`提议中` → `已接受` → `已废弃` 或 `已被 <编号> 取代`。

| 情况 | 处理 |
|---|---|
| PR 中、状态为提议中 | 可自由修改 |
| 已接受，修正错别字、失效链接 | 直接修改 |
| 决定被推翻或需要补充 | 写新 ADR，注明"取代 / 补充 <编号>"；旧 ADR 只修改状态行，正文不动。两处在同一个 PR 中修改 |
| 不再适用且无替代 | 状态改为已废弃，附一句原因 |

决定较小时，ADR 可与实现放在同一个 PR；否则先单独合并 ADR（`Refs #编号`），再提实现 PR（`Closes #编号`）。

### 8.4 文件命名

- ADR：`docs/adr/<4位序号>-<短横线描述>.md`，如 `0001-agent-push-model.md`。
- 测试报告：`docs/testing/reports/<年-月>-<短横线描述>.md`，如 `2026-10-network-outage.md`。
- 组件说明：`docs/components/<组件>.md`，如 `agent.md`。
- 其余文档文件名一律小写短横线。

## 9. 由工具强制执行的规则

| 规则 | 配置文件 | 执行时机 |
|---|---|---|
| 缩进、换行、编码 | `.editorconfig` | 编辑器保存时 |
| Go 格式 | `.golangci.yml`（gofumpt、goimports） | `make fmt`、CI |
| Go 静态检查与依赖方向 | `.golangci.yml` | `make lint`、CI |
| Proto 风格、目录与兼容性 | `buf.yaml` | `make proto-lint`、CI |
| 前端格式与检查 | `web/eslint.config.js`、Prettier | `npm run lint`、CI |
| Issue 必填项 | `.github/ISSUE_TEMPLATE/` | 创建 Issue 时 |
| PR 描述与自查清单 | `.github/pull_request_template.md` | 创建 PR 时 |
| PR 标题 | `.github/workflows/pr-title.yml` | 创建或编辑 PR 时 |
| 合并前检查 | `.github/workflows/ci.yml` + 仓库规则集 | 每次推送到 PR 时 |
