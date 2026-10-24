# 桌面端打包

统一入口为 `uv run --locked tools/release.py package`，在目标系统的原生环境中运行。
本轮发布矩阵为 Windows x64、macOS Intel / Apple Silicon 和 Linux x64。

| 平台 | 产物 | 构建环境 |
| --- | --- | --- |
| Windows x64 | 便携 ZIP、NSIS 安装器 | Windows、Go、GCC、NSIS 3.13.0、WebView2 |
| macOS Intel / Apple Silicon | APP ZIP、DMG | 对应架构的 macOS、Xcode 命令行工具 |
| Linux x64 | tar.gz、DEB、RPM | Ubuntu 24.04、GCC、GTK4 / WebKitGTK 6.0 开发库 |

Windows / Linux arm64 可在对应原生机器上显式指定 `--target`；暂未纳入 CI 验证范围。
Android / iOS 模板保留，本轮不作为发布目标。
当前 Wails 版本的 AppImage helper 仍依赖旧的 WebKit 注入库名称并使用浮动下载工具，
因此本轮正式入口仅提供 tar.gz / DEB / RPM，待 GTK4 兼容性验证完成再接入 AppImage。

## 本地使用

安装 Go、Node.js 24、uv 和各平台原生依赖，再运行：

```text
uv run --locked tools/release.py install-cli
uv run --locked tools/release.py sync
uv run --locked tools/release.py doctor
uv run --locked tools/release.py package
```

Wails CLI 版本从 `go.mod` 读取，与应用依赖一致。Python 工具依赖锁定于
`tools/release.py.lock`，前端使用 `npm ci` 和现有 lock 文件。
Windows 设置 `CC` 为 GCC 可执行文件路径，并将其目录加入 PATH。

仅构建 Windows 便携包：

```text
uv run --locked tools/release.py package --target windows-amd64 --formats portable
```

`--skip-frontend` 仅供本地复用已构建的前端，CI 不使用该选项。
`wails3 task package` 同样转至统一入口，可传 `TARGET`、`FORMATS`。
在任意平台查看其他目标的产物计划无需交叉编译：

```text
uv run --locked tools/release.py plan --target darwin-arm64
```

## 版本与产物

修改 `build/release.json` 后运行 `sync`，同步应用版本、前端版本、Windows
资源与安装器品牌、macOS plist。发布标签必须等于该文件的 `vVERSION`。
像素图标从同一套网格生成 PNG / ICO / ICNS，避免平台脚本重新缩放出模糊轮廓。

产物位于 `release/vVERSION/系统-架构/`，包含版本和架构明确的文件名、
`manifest.json` 与 `SHA256SUMS.txt`。只在该目标全部格式构建成功后落入发布目录。
可用 `uv run --locked tools/release.py verify --target windows-amd64` 校验文件清单及内容，
其他目标替换对应 `--target`。
重复打包遇到已有目录时会停止，请先将旧产物移至其他目录，避免混入旧文件。

构建暂存于 `.release-work/`；程序、日志、密钥、聊天数据库和 WebView2 缓存
不会从当前 `bin/` 打入包内。便携包通过启动脚本将数据存到解压目录的 `data/`。
安装版使用正常用户数据目录；卸载只移除已安装的程序文件并保留用户数据。

## CI 与发布

Pull Request 仅自动运行测试检查，不执行安装包构建；仅推送 `v*` 版本标签触发打包与发布。
推送版本标签时，先通过测试和
标签一致性检查，再并行构建四个目标；全部成功后汇总校验和并上传安装包，
上传全部完成后自动发布 GitHub Release。上传期间暂存为草稿，避免发布缺少附件的版本。
带连字符的版本（例如 `v0.4.0-beta.1`）自动标记为预发布版本。

例如发布当前 `0.3.0` 版本：

```text
git tag -a v0.3.0 -m "发布 v0.3.0"
git push origin v0.3.0
```

发布其他版本前，先更新 `build/release.json` 的 `version`，运行
`uv run --locked tools/release.py sync` 并提交、推送，再创建对应的 `vVERSION` 标签。
工作流使用仓库自带的 `GITHUB_TOKEN` 发布，无需额外配置个人访问令牌。

macOS 分别使用原生 Intel 与 Apple Silicon runner，参见
[GitHub runner 平台与架构说明](https://docs.github.com/en/actions/reference/runners/github-hosted-runners)。
本地 Windows 验收不能替代其他系统的真实构建，macOS / Linux 应以 CI 成功产物为准。

## 签名

macOS 默认使用 ad-hoc 签名并验证包完整性。持有证书时可设置
`MACOS_SIGN_IDENTITY`，以及已配置到钥匙串的 `MACOS_NOTARY_PROFILE`；
流水线会执行 Developer ID 签名、公证与 APP stapling。
CI 默认没有注入发布证书，产物并不具备已公证发行身份。
Windows 当前生成未签名包，Authenticode 证书应在后续专用签名环境接入。

## 检查

```text
uv run --with pyyaml==6.0.3 python -m unittest discover -s tools/tests -v
```

测试覆盖版本同步、重复执行一致性、格式和平台限制、便携包内容边界、校验和与卸载规则。
