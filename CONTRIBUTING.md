# 开发规范

## 分支
从 main 拉分支，命名为 `feat/xxx`、`fix/xxx`、`docs/xxx`。

## 提交信息
遵循 Conventional Commits，例如 `feat(agent): 采集 GPU 显存`。

## 合并流程
提 PR → CI 通过 → 自查清单 → squash 合并。提交前本地运行 `make all`。
