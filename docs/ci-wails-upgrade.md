# Wails 3 升级与 Linux CI 修复

本项目使用 Wails `v3.0.0-beta.34`，Go 最低版本为 1.26，前端
`@wailsio/runtime` 固定为 `3.0.0-beta.34`。CLI 继续从 `go.mod` 读取版本安装。

## 失败原因与修复

[v0.3.0 的发布构建](https://github.com/reatang/fire-poker/actions/runs/40073132801)
在 Linux 安装旧版 `v3.0.0-alpha.79` CLI 时失败：旧版默认使用 GTK3，
需要 `webkit2gtk-4.1`，而 runner 安装的是 GTK4 / WebKitGTK 6.0 开发包。

新版 Wails 默认使用 GTK4，与现有 Linux 打包依赖一致，无需额外安装旧版 WebKit。
工作流在安装 CLI 前执行 `pkg-config --exists gtk4 webkitgtk-6.0`，
并在 Pull Request 中执行四个平台的原生打包和校验，提前发现发布链路问题。
PR 仅上传构建产物；发布 Release 仍由版本标签触发。

| 范围 | 要求与边界 |
| --- | --- |
| Go 与 CLI | Go 1.26+；使用 `uv run --locked tools/release.py install-cli` 安装匹配版本 |
| 前端 | 使用 `npm ci` 安装锁定的 runtime 与其他依赖 |
| Linux CI | Ubuntu 24.04、GTK4 / WebKitGTK 6.0；先检查 pkg-config，再安装 CLI |
| Docker 交叉构建模板 | 使用 Go 1.26 / Debian Trixie，提供新版 Go 和 GTK4 开发包 |
| 验证边界 | Windows 可本地验证；macOS、Linux 的原生打包以 CI 结果为准 |

## 版本依据

- [Wails v3.0.0-beta.34 发布说明](https://github.com/wailsapp/wails/releases/tag/v3.0.0-beta.34)
- [Wails 模块的 Go 版本要求](https://github.com/wailsapp/wails/blob/v3.0.0-beta.34/v3/go.mod)
- [Linux 默认 GTK4 / WebKitGTK 6.0 编译配置](https://github.com/wailsapp/wails/blob/v3.0.0-beta.34/v3/internal/operatingsystem/webkit_linux.go)

## 后续执行

| 行动项 | 完成标准 |
| --- | --- |
| 合并前检查 PR | 测试和四个平台的打包、校验均通过 |
| 重新发布 | 将修复纳入新的版本提交并推送匹配的版本标签；旧标签重跑仍使用旧代码 |
