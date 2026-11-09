# 0006. 发布与部署：tag 触发发布，Controller 用 Compose，Agent 用 systemd + Ansible

- 状态：已接受
- 日期：2026-11-09

## 背景

- 部署对象：1 个 Controller，约 20 台以内的 GPU 服务器各一个 Agent（ADR 0003）。
- 手动拷贝二进制只适合首次试跑，每次升级都要逐台操作，容易遗漏、版本不一致。
- Agent 需要读取宿主机的 `/proc`、磁盘挂载和 `nvidia-smi`，M4 还要操作宿主机进程。
- Controller 以后会依赖 PostgreSQL 与 VictoriaMetrics（ADR 0002）。
- 校园网访问 GitHub、ghcr 不稳定，GPU 服务器不一定能访问外网。
- GPU 服务器是公用的，不宜在上面安装额外的运行环境。

## 备选方案

### A. 手动拷贝 + nohup/tmux

- 优点：零准备。
- 缺点：不能开机自启、崩溃不重启；升级要逐台操作，无法知道各台的版本。

### B. Agent 也放进容器

- 优点：与 Controller 统一。
- 缺点：需要 `--pid=host`、挂载 `/proc`、所有磁盘挂载点和 GPU 设备，基本等于特权容器；容器内看到的进程用户、挂载路径要额外换算。

### C. Agent 自更新

- 优点：不需要 SSH 到每台机器。
- 缺点：要实现下载、校验、替换、回滚，且依赖服务器访问外网；当前规模用不上。

### D. Kubernetes DaemonSet

- 与 ADR 0003 冲突。

### E. tag 发布 + Controller 用 Compose + Agent 用 systemd/Ansible（选定）

## 决策

采用方案 E：

- **发布**：推送 `v*` tag 触发 `release` 工作流，构建静态链接的二进制（`CGO_ENABLED=0`），上传到 GitHub Releases 并附 `sha256sums.txt`；Controller 镜像推送到 `ghcr.io/forlwow/labherd-controller:<版本>`。
- **Controller**：`deploy/compose/compose.yaml`，宿主机网络（保留 Agent 源 IP），版本由 `.env` 中的 `LABHERD_VERSION` 指定。
- **Agent**：二进制加 `deploy/systemd/labherd-agent.service`，以系统用户 `labherd` 运行，`Restart=always`。
- **批量部署**：`deploy/ansible/agent.yml`，只需 SSH。发布包在控制机下载并校验后再复制到服务器，服务器无需访问外网。

## 理由

- 升级只需改一个版本号再运行一条命令，所有服务器版本一致，可回滚。
- Ansible 只依赖 SSH 与 sudo，不在服务器上安装常驻组件。
- Agent 以普通系统用户运行即可读取 M1 所需的信息；systemd 负责开机自启与崩溃重启，在 Agent 内置退避重连之前也承担断线后的重启。
- Compose 文件以后直接加入 PostgreSQL 与 VictoriaMetrics，与 ADR 0003 中"一条 compose 命令"的预期一致。

## 后果

- 需要在首次发版后把 ghcr 镜像包设为公开。
- 只发布 linux/amd64。如需 arm64（如机器人上的 Jetson），在 `release` 工作流中增加一组构建即可。
- M4 若需要以 root 操作其他用户的进程，届时调整 unit 的运行用户或增加 capability，并更新本 ADR。
