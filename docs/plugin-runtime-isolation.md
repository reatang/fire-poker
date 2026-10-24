# 插件运行时的目录隔离

保留按需下载 Node.js 与 npm 的方案。CLI 插件默认使用应用管理的目录存放配置、缓存、日志和临时文件，不修改宿主系统的 PATH 或全局安装 Node.js。

## 写入位置

下表中的路径相对于应用数据目录。默认数据目录为用户目录下的 `.fire-poker`，兼容旧 `.lemontea`，也支持自定义数据目录。

| 内容 | 位置 |
| --- | --- |
| Node.js 与 npm | `runtime/node/` |
| CLI 插件程序 | `plugins/cli/<name>/` |
| HOME、USERPROFILE | `plugins/cli_data/<name>/home/` |
| XDG 配置、数据、缓存 | `plugins/cli_data/<name>/{config,data,cache}/` |
| APPDATA、LOCALAPPDATA | `plugins/cli_data/<name>/appdata/{roaming,local}/` |
| TEMP、TMP、TMPDIR | `plugins/cli_data/<name>/tmp/` |
| npm 缓存与日志 | `plugins/cli_data/<name>/cache/npm/`、其下的 `_logs/` |
| npm 用户与全局配置路径 | `plugins/cli_data/<name>/config/npmrc`、`npmrc-global` |

安装过程始终使用私有环境；插件执行、登录和帮助探测遵循 manifest 的 `isolation` 设置，未设置时默认隔离。私有目录会在启动子进程前创建。环境变量仅传给子进程，代理、证书及 registry 等其他环境设置仍可继承；隔离安装不再默认读取宿主用户的 `.npmrc`，私有仓库配置可写入表中的私有 npm 配置文件。

## 边界与清理

| 情况 | 行为 |
| --- | --- |
| 显式选择 `shared` | 插件执行、登录和帮助探测保留用户目录；安装过程仍使用私有目录 |
| 本地 MCP | 沿用 MCP 自身的命令与环境配置，不自动套用 CLI 隔离策略 |
| 删除插件 | 保留 `cli_data` 中的配置、登录状态、缓存和临时文件；重置插件数据可清除这些内容 |
| 历史文件 | 不迁移或删除此前留在宿主用户目录里的文件 |
| 第三方程序自行写入绝对路径 | 环境隔离不能阻止；当前没有文件系统沙箱 |

实现依据：`backend/pkg/cli/env.go`、`backend/pkg/cli/npm.go`、`backend/pkg/cli/manager.go`、`backend/service/plugin/plugin_cli.go`。
