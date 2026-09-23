# M1 范围说明：节点与 GPU 监控

| 项 | 内容 |
|---|---|
| 里程碑 | M1（GitHub Milestone `M1`） |
| 目标版本 | `v0.1.0` |
| 状态 | 草案 |
| 相关文档 | `docs/adr/0001-agent-push-model.md`、`docs/design/api.md`、`docs/design/data-model.md` |

## 1. 背景与目标

实验室的 GPU 分布在多台服务器上。目前想知道"哪张卡空着、谁在用、磁盘还剩多少"，只能逐台登录执行 `nvidia-smi` 和 `df`。

M1 要实现的是：**在一个网页上实时看到所有服务器的资源状态，以及 GPU 被谁占用。** 服务器离线或网络中断时，系统能正确显示离线状态，并在恢复后自动补齐数据。

M1 同时是后续里程碑的地基：Agent 与 Controller 的连接、节点身份、指标管道会被 M2（告警/预约）到 M4（调度）直接复用。

## 2. 使用场景

1. 组员准备跑实验，打开页面，找到显存空闲的卡，并确认没有别人正在用。
2. 某张卡被长期占用，组员在页面上看到占用者的用户名和命令，可以直接去沟通。
3. 管理员查看各服务器的磁盘使用率，提前处理快满的盘。
4. 校园网中断后恢复，管理员查看断网期间各节点的历史曲线。

## 3. 范围内

### 3.1 节点接入

- 每台服务器运行一个 `labherd-agent`，由 Agent 主动与 `labherd-controller` 建立 gRPC 双向流。
- 节点身份以 `/etc/machine-id` 为准，主机名只用于显示。主机名变化不产生新节点。
- Agent 连接时上报静态信息：主机名、操作系统、内核版本、CPU 型号与核数、内存总量、GPU 列表（UUID、序号、型号、显存总量、驱动版本）、Agent 版本。
- Agent 按固定间隔发送心跳和指标。

### 3.2 节点状态

| 状态 | 含义 |
|---|---|
| `NODE_STATE_ONLINE` | 流已建立，心跳正常 |
| `NODE_STATE_LOST` | 超过心跳超时仍未收到心跳 |

M1 不做"手动下线"和"维护中"状态，留到 M2。

### 3.3 采集指标

指标名遵循 `conventions.md` 第 5 节的规范。

| 指标 | 标签 | 说明 |
|---|---|---|
| `labherd_node_cpu_usage_ratio` | `node_id` | 整机 CPU 使用率，0～1 |
| `labherd_node_load1` | `node_id` | 1 分钟负载 |
| `labherd_node_memory_used_bytes` / `_total_bytes` | `node_id` | 内存 |
| `labherd_node_disk_used_bytes` / `_total_bytes` | `node_id`, `mountpoint` | 各挂载点容量，排除 tmpfs、overlay 等虚拟文件系统 |
| `labherd_node_network_receive_bytes_total` / `_transmit_bytes_total` | `node_id`, `device` | 网卡流量 |
| `labherd_gpu_utilization_ratio` | `node_id`, `gpu_uuid` | GPU 利用率，0～1 |
| `labherd_gpu_memory_used_bytes` / `_total_bytes` | `node_id`, `gpu_uuid` | 显存 |
| `labherd_gpu_temperature_celsius` | `node_id`, `gpu_uuid` | 温度 |
| `labherd_gpu_power_watts` | `node_id`, `gpu_uuid` | 功耗 |
| `labherd_gpu_user_memory_used_bytes` | `node_id`, `gpu_uuid`, `user` | 按用户聚合的显存占用 |

**进程级信息不作为时序指标存储**：PID 基数高且不断变化，写入 VictoriaMetrics 会导致序列数量膨胀。进程列表作为节点的"当前快照"保存在 Controller 内存中，由 API 直接返回；需要历史数据时，使用上表中按用户聚合的指标。

### 3.4 GPU 进程归属

- 通过 NVML 获取每张卡上的进程 PID 和显存占用，再读取 `/proc/<pid>/status` 得到 uid 并换算为用户名，读取 `/proc/<pid>/cmdline` 得到命令（截断至 256 字符）。
- **必须覆盖不经过平台启动的进程**，例如直接 `python train.py`、桌面程序等。M1 中所有进程都属于这一类。
- 取不到信息的进程（权限不足、进程已退出）照常显示 PID 和显存，用户名显示为 `unknown`，不能因此让采集整体失败。

### 3.5 断网与恢复

- Agent 断线后按指数退避重连，初始 1 秒、上限 60 秒、带随机抖动，避免所有 Agent 同时重连造成冲击。
- 断线期间 Agent 继续采集，数据写入本地环形缓冲；重连后先补发缓冲中的数据，再发送实时数据。补发数据保留原始时间戳。
- 缓冲写满时丢弃最旧的数据，并通过 `labherd_agent_buffer_dropped_total` 计数。
- 每条上报数据带单调递增的序号。Controller 用序号去重，保证补发幂等。
- Controller 重启后，所有 Agent 自动重连，节点列表从 PostgreSQL 与重连信息恢复。
- 心跳超时后，Controller 将节点标为 `LOST`，并记录状态变更时间。

### 3.6 存储

- **PostgreSQL**：存节点与 GPU 的静态信息、节点状态及最后在线时间。表结构见 `docs/design/data-model.md`，以 `migrations/` 为准。
- **VictoriaMetrics**：存 3.3 节中的全部时序指标。
- **Controller 内存**：存各节点的最新快照和 GPU 进程列表。

### 3.7 前端

- **节点列表页**：显示每个节点的状态、CPU/内存/磁盘概览、每张 GPU 的显存条与利用率，以及当前使用者。支持按"有空闲 GPU"筛选。
- **节点详情页** `pages/node-detail/`：显示静态信息、各项指标的历史曲线（可选 1 小时、6 小时、24 小时、7 天）和当前 GPU 进程表。
- 页面数据自动刷新。

### 3.8 部署与发版

- Controller、PostgreSQL、VictoriaMetrics、Web 使用 `deploy/` 下的 docker compose 一键启动。
- Agent 以单个二进制加 systemd unit 的方式安装，提供安装说明。
- 在 main 上打 tag `v0.1.0`，并将镜像推送到 `ghcr.io/forlwow/labherd-*`。

## 4. 不在范围内

以下内容明确不在 M1 中处理，如有需要会在后续里程碑中评估：

- 告警与通知（M2）
- GPU 预约、排队、调度（M2、M4）
- 代码与数据集迁移（M3）
- 登录、权限、多租户（M1 仅部署在实验室内网，风险见第 8 节）
- 按用户或目录统计磁盘占用：在共享大盘上执行 `du` 代价很高，需要单独设计
- 容器内进程与容器的对应关系：M1 只显示宿主机视角的 PID 与用户
- Controller 高可用、多副本
- 移动端适配

## 5. 验收标准

全部通过后，M1 才算完成。每一条都应能用自动化测试或明确的手工步骤验证。

| 编号 | 标准 | 验证方式 |
|---|---|---|
| AC-01 | 新服务器安装并启动 Agent 后，30 秒内出现在节点列表中，状态为 ONLINE | 手工 |
| AC-02 | 页面上的 GPU 显存与利用率，与同一时刻 `nvidia-smi` 的输出一致，延迟不超过 2 个采集周期 | 手工对比 |
| AC-03 | 在服务器上直接运行 `python` 训练脚本占用 GPU，页面能显示其 PID、用户名、命令和显存 | 手工 |
| AC-04 | 在无 GPU 或未装驱动的机器上，Agent 正常运行，只上报主机指标，不崩溃、不刷错误日志 | 集成测试 |
| AC-05 | 断开某节点的网络，在心跳超时后节点显示为 LOST | 集成测试 |
| AC-06 | 断网 10 分钟后恢复，1 分钟内节点回到 ONLINE，断网期间的指标曲线无缺口 | 集成测试并出具测试报告 |
| AC-07 | 重启 Controller 后，所有 Agent 在 2 分钟内自动重连，无需人工操作 | 集成测试 |
| AC-08 | 重复补发同一批数据不产生重复样本 | 单元测试 |
| AC-09 | 可以查询最近 7 天的历史曲线 | 手工 |
| AC-10 | Agent 稳态资源占用：CPU 平均低于单核的 2%，常驻内存低于 100 MB | 压测工具（`test/`） |
| AC-11 | 在一台干净的机器上，按文档从零部署完成时间不超过 30 分钟 | 手工 |
| AC-12 | CI 四项检查全绿；核心包（采集、缓冲、注册表）单元测试覆盖率不低于 60% | CI |

## 6. 非功能要求

| 项 | 要求 |
|---|---|
| 采集间隔 | 默认 10 秒，可通过 `LABHERD_COLLECT_INTERVAL` 配置 |
| 心跳超时 | 默认 30 秒 |
| 本地缓冲 | 至少容纳 1 小时的数据 |
| 指标保留 | 30 天 |
| 规模假设 | 不超过 20 个节点，每节点不超过 8 张 GPU（待确认） |
| 兼容性 | Agent 只依赖 glibc 与 NVIDIA 驱动，不要求安装 Docker |

## 7. 待确认

- [ ] 实验室的服务器数量、每台的 GPU 数量、操作系统与驱动版本。这些信息决定第 6 节的规模假设是否成立。
- [ ] Controller 部署在哪台机器上。它需要一台所有服务器都能访问、并且尽量不关机的机器。
- [ ] Agent 以什么用户运行。读取其他用户进程的 `cmdline` 可能需要 root 权限或额外的 capability。
- [ ] 是否有服务器上运行着 Docker 容器训练任务。这决定第 4 节中容器进程的限制是否会造成明显困扰。
- [ ] 前端 API 的实现方式（ADR 0004）。

## 8. 风险

| 风险 | 影响 | 应对 |
|---|---|---|
| 页面无登录，内网任何人都能看到进程命令行 | 命令行参数中可能包含路径或令牌 | M1 只在内网部署；命令行显示时截断；M2 评估加认证 |
| 部分服务器的驱动版本过旧，NVML 接口不全 | 部分 GPU 指标缺失 | 不支持的指标跳过并记录一次日志，不影响其余指标 |
| Controller 所在机器宕机 | 整体不可用，但 Agent 在本地继续缓冲 | 可以接受（ADR 0003）；缓冲容量覆盖一般的停机时长 |
| 磁盘紧张的服务器上缓冲写满 | 丢失最旧的数据 | 缓冲以内存为主并限制容量，丢弃有计数可查 |

## 9. 任务拆分

每项对应一个 Issue 和一个 PR，括号中为 PR 标题。

**阶段 A：骨架**

1. 在 `conventions.md` 中增加 `docs/design/` 与 `docs/milestones/` 目录（`docs: update conventions`）
2. 编写 ADR 0001～0004（`docs(docs): add initial adrs`）
3. 定义 Agent 流接口与节点消息（`feat(api): define agent stream proto`）
4. Controller 接受连接，并在内存中维护节点表（`feat(controller): track connected nodes in memory`）
5. Agent 连接 Controller 并发送心跳（`feat(agent): connect and send heartbeat`）
6. 编写端到端集成测试（`test: add agent-controller integration test`）

**阶段 B：采集**

7. 采集主机指标：CPU、内存、磁盘、网络（`feat(agent): add host collector`）
8. 通过 NVML 采集 GPU 指标（`feat(agent): add gpu collector`）
9. GPU 进程归属（`feat(agent): resolve gpu process owner`）

**阶段 C：可靠性**

10. 指数退避重连（`feat(agent): reconnect with backoff`）
11. 本地环形缓冲与按序号补发（`feat(agent): buffer and replay metrics`）
12. 心跳超时后标记 LOST（`feat(controller): mark lost nodes on heartbeat timeout`）
13. 断网测试报告（`docs(docs): add network outage test report`）

**阶段 D：存储与 API**

14. 建立节点与 GPU 表（`feat(db): create nodes and gpus tables`）
15. 指标写入 VictoriaMetrics（`feat(controller): write metrics to victoriametrics`）
16. 面向前端的查询接口（`feat(controller): add node query api`）

**阶段 E：前端与交付**

17. 节点列表页（`feat(web): add node list page`）
18. 节点详情页（`feat(web): add node detail page`）
19. compose 与 systemd 部署文件（`build(deploy): add compose and systemd units`）
20. 发布 v0.1.0
