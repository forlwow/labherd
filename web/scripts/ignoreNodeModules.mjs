// 有的 npm 包自带 .go 文件（如 flatted），会被 `go test ./...`、golangci-lint 当成本仓库的包。
// 在 node_modules 里放一个 go.mod，使它成为独立模块，`./...` 就会跳过整个目录。
// 由 npm 的 postinstall 自动执行。
import { writeFileSync } from 'node:fs';

writeFileSync(
  new URL('../node_modules/go.mod', import.meta.url),
  '// 由 web/scripts/ignoreNodeModules.mjs 生成，让 Go 工具跳过 node_modules。\nmodule labherd.invalid/node_modules\n',
);
