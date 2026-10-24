# 按操作系统隔离命令行工具

命令行能力使用独立的平台工具 ID。工具目录、默认选择和发给 LLM 的声明只包含当前操作系统对应的命令行工具；显式提交其他平台的工具 ID 不会启用它们。

| 操作系统 | 工具 ID | 程序探测顺序 | 命令语法 |
| --- | --- | --- | --- |
| Windows | `windows_shell` | PATH 中的 `pwsh.exe`、常见 PowerShell 7 安装目录、PATH 中的 `powershell.exe`、系统 Windows PowerShell 目录 | PowerShell |
| macOS | `macos_shell` | zsh、bash、sh | macOS 命令与 POSIX Shell 语法 |
| Linux | `linux_shell` | bash、sh | Linux 命令与 POSIX Shell 语法 |

Windows 不自动回退到 cmd.exe，避免同一工具的命令语法随安装环境改变。找不到对应程序时返回启动错误。

注册入口只装配当前平台工具。三个工具的执行入口还会检查 `runtime.GOOS`，因此直接构造其他平台工具也不能执行命令。普通管道、PTY 和共享终端统一使用上述程序探测逻辑。

旧配置及旧会话的 `shell` ID 在读取、比较和构建时映射到当前平台 ID；已保存的其他平台专用 ID 不会被映射到本机工具。工具描述明确命令语法，前端使用对应的平台名称。确认策略同时识别旧 ID 和三个新 ID。

实现见 `backend/pkg/agent/tools/platform_shell.go`、`shell.go` 和 `backend/pkg/agent/toolkit/platform.go`。测试覆盖程序优先级、安装目录探测、旧 ID 兼容、跨平台执行拒绝和最终 HTTP 请求的工具声明。
