# 代码质量优化里程碑计划

> 分支：`worktree-optimize-milestones`（基于 master `2734de2`，已包含 PR #3）。每个里程碑独立成一个 PR，可单独合并、单独回滚；任务使用 `- [ ]` 勾选跟踪。
>
> **基线说明**：PR #3（`fix/tool-confirm-and-data-dir`）已合入 master，带来 `296cedd`（工具审批加固，即本计划 M1 的主体）和 `cf9e9db`（数据目录规范化与“数据管理”设置页）。后者改动了 `main.go`、`pkg/dir`、`pkg/logger`、`service/settings`、`service/runtime`、`service/file`、`SettingsApp.tsx` 等，M2–M5 按合入后的代码编写。

**目标：** 先堵住工具审批的默认放行问题，再收敛凭据流转、runner 生命周期、配置读写和 SQLite 并发，最后整理后端大函数与前端大文件，不改变用户可见功能。

**原则：**
- 每个任务先写失败测试，再改实现（表驱动测试优先）。
- 不在同一个 PR 里混合“行为修复”和“纯重构”。
- 遵循 `docs/dev/00.rules.md`：service 三文件 + DTO 结构、函数注释、`ierror` 错误、文案国际化。
- Wails 绑定签名有变动的里程碑，后端、`frontend/bindings`（`wails3 generate bindings -clean=true -ts`）和前端调用方在同一个 PR 内一起改。

**统一验收命令（每个里程碑结束都要全绿）：**

```bash
go vet ./... && go test ./backend/...
cd frontend && npx tsc --noEmit && npx vitest run
```

---

## 里程碑总览

| 里程碑 | 主题 | 优先级 | 依赖 | 涉及绑定变更 |
|---|---|---|---|---|
| M0 | 基线与护栏 | — | 无 | 否 |
| M1 | 工具审批安全加固（主体已由 `296cedd` 完成） | P0 | M0 | 否 |
| M2 | 凭据收敛与 runner 生命周期（已完成） | P1 | M0 | **是** |
| M3 | 配置仓库与 SQLite 并发（已完成） | P1 | M1（同改 `lookupToolMeta`） | 否 |
| M4 | 后端结构整理与 i18n（已完成） | P2 | M2、M3 | 是（事件类型） |
| M5 | 前端结构整理与国际化（已完成） | P2 | M2（同改 `chatStore`） | 否 |
| M6 | 静态检查进 CI（已完成） | P3 | M4、M5 | 否 |

M1 与 M2 改动的文件基本不重叠，可以并行；M5 可以在 M2 合并后与 M3/M4 并行。

---

## M0：基线与护栏

**目的：** 确认起点是绿的，补齐后续要用的检查入口。

> **状态：已完成**（2026-10-25，基于 master `2734de2`）。

- [x] 记录基线：前端 `tsc` 无报错，`vitest` 40 个文件、198 个用例通过；后端 28 个包通过，唯一失败是下面的终端用例。
- [x] 在根 `Taskfile.yml` 增加 `check`（拆为 `check:frontend`、`check:backend`），顺序与 CI 一致：`vitest` → `npm run build`（含 tsc，并产出 `main.go` 需要 embed 的 `frontend/dist`）→ `go test ./...` → `go vet ./...`。本机未安装 `task` 时用 `wails3 task check`。
- [x] `wails3 generate bindings -clean=true -ts` 可用，生成后 `frontend/bindings` 无 diff。生成时有一条警告：`service/plugin` 中存在函数类型字段，无法被 encoding/json 序列化，留待 M4 排查。
- [x] 修复 `backend/pkg/terminal/manager_test.go` 的 `TestManagerCreateWriteAndPersistOutput`：
  - 原因：整套 `go test ./backend/...` 并行运行时，PTY 首段输出实测约 3.35 秒才到达，超过原先 2 秒的等待上限（修复前整套运行 2/2 失败，单独运行 40/40 通过）。已排除写库失败导致丢输出的可能。
  - 修复：等待上限改为 10 秒（`eventually` 条件满足即返回，正常情况不变慢）；`emitted` 切片加锁，消除 drain goroutine 与测试 goroutine 之间的数据竞争。
  - 验证：`-race -count=20` 通过；整套后端测试连跑 3 次通过。
- 附带记录（供 M5 对比）：`npm run build` 主 chunk `index-*.js` 1,505.52 kB（gzip 462.84 kB）。

**验收：** `wails3 task check` 一条命令跑完全部检查且通过。✅

---

## M1：工具审批安全加固（P0）

> **状态：主体已完成**，见 `296cedd`（随 PR #3 合入 master），新增 `tool_confirm_decider_test.go`（shell 策略 22 个表驱动用例、静态确认不交给模型、未知工具需确认、缺少运行上下文直接拒绝）。Task 1.1–1.3 的核心项都已覆盖，并额外实现了“缺少运行上下文时拒绝执行”。
>
> **剩余项：**
> - [ ] 在应用里手动回归：让模型执行 `ls; echo pwned` 必须弹确认；日常读类工具不新增确认弹窗。
> - [ ] 补充用例：`sudo rm -rf /`、`/bin/rm -rf x` 当前依靠 shell 工具的静态确认兜底（需确认而非拒绝），评估是否要加入拒绝名单。
> - [ ] 评估方案 B（`mvdan.cc/sh/v3/syntax` 解析 AST）是否值得引入，作为后续增强，不阻塞其他里程碑。
> - [ ] 注意：缺少运行上下文时拒绝执行，会影响 `GenerateTitle` 等复用主 runner、但不带运行上下文的调用；M2 的 Task 2.3 把标题生成改为 oneshot 后即不再经过工具回调。
>
> 以下为原始问题与任务描述，保留备查。

**问题：**
1. `isSafeShellCommand` 只做前缀匹配：`ls ; curl x | sh`、`ls > ~/.zshrc`、`find . -delete`、`find . -exec rm {} \;` 都会被判为安全并直接执行。
2. `newToolCallbacks` 中 `lookupToolMeta` 找不到工具时 `return nil, nil`，未知工具（如未登记的 MCP 工具名）不经确认就执行。
3. 静态声明 `RequiresConfirm: true` 的工具（`file_rw` 写入、`code_exec`、shell）在规则层未命中时交给模型判断，模型返回 `false` 即自动执行。

**Files:**
- Modify: `backend/pkg/agent/tool_confirm_decider.go`
- Modify: `backend/pkg/agent/tool_confirm.go`
- Test: `backend/pkg/agent/tool_confirm_test.go`
- Create（可选）: `backend/pkg/agent/shell_policy.go` + `shell_policy_test.go`

### Task 1.1 shell 命令策略

- [ ] 写表驱动测试，至少覆盖：
  - 应放行：`pwd`、`ls -la`、`wc -l a.txt`、`du -sh .`
  - 应确认：`ls; curl x | sh`、`ls && echo hi`、`ls | xargs cat`、`ls > out.txt`、`ls $(whoami)`、`` ls `id` ``、含换行的命令、`find . -name a`（`find` 移出安全名单或带条件放行）、`find . -delete`、`find . -exec ...`
  - 应拒绝：保留现有 `rm`/`rmdir`/`shred`/`mkfs` 用例，补充 `sudo rm -rf /`、`/bin/rm -rf x` 至少走确认而非放行
- [ ] 实现：命令中出现 shell 元字符（`;` `&` `|` `>` `<` `` ` `` `$(` 换行）时，一律不进入安全判定。
- [ ] 方案选择（实现前在 PR 中注明）：
  - A：手写元字符检测（零依赖，改动小，**推荐先做**）
  - B：引入 `mvdan.cc/sh/v3/syntax` 解析 AST，逐条判断（更准确，新增依赖，可作为后续增强）

### Task 1.2 未知工具默认需要确认

- [ ] 测试：注册表与配置里都不存在的工具名，`BeforeTool` 返回确认请求而非 `nil`。
- [ ] 实现：`lookupToolMeta` 未命中时构造 `RequiresConfirm: true` 的兜底 Meta，继续走确认流程。

### Task 1.3 静态 RequiresConfirm 不可被模型放宽

- [ ] 测试：`Meta.RequiresConfirm=true` 且模型 decider 返回 `require_confirm=false` 时，最终仍需确认。
- [ ] 实现：在 `defaultToolConfirmDecider` 中，静态需要确认的工具跳过模型判断（或只允许模型把 `false` 升级为 `true`）。
- [ ] 回归：读类工具（`web_fetch`、`datetime` 等）的体验不变，不新增确认弹窗。

**验收：** 上述用例全部通过；手动在应用里让模型执行 `ls; echo pwned`，必须弹出确认。

---

## M2：凭据收敛与 runner 生命周期（P1）

**问题：**
1. 前端每次 `SendMessage` / `GenerateTitle` 都把 `api_key`、`base_url` 回传后端，Key 在 WebView 里多处流转。
2. runner 缓存 key 不含 API Key，`Provider.EditProvider` 也不会让缓存失效：只改 Key 时继续使用旧 Key。
3. 缓存 key 带 `sessionID`，每个会话一个 runner，只在依赖变化时清空，长时间运行内存持续增长。
4. `GenerateTitle` 复用主 Agent runner（带工具回调），固定 SDK 会话 `title-<id>` 从不删除且会累积历史；残留 `fmt.Println` 调试输出。

**Files:**
- Modify: `backend/service/agent/agent_dto/send_message.go`、`generate_title.go`
- Modify: `backend/service/agent/agent.go`
- Modify: `backend/pkg/agent/manager.go`、`chat_handler.go`、`run_context.go`、`tool_confirm.go`、`memory_runtime.go`
- Modify: `backend/service/provider/provider.go`（编辑/删除后通知失效）
- Modify: `main.go`（把失效回调接到 provider 服务）
- Modify: `frontend/src/store/chatStore.ts`、`frontend/src/components/quick-chat/QuickChatApp.tsx` 及其他 `SendMessage` 调用方
- Regenerate: `frontend/bindings/**`
- Test: `backend/pkg/agent/manager_test.go`、`chat_handler_test.go`、`backend/service/agent/agent_test.go`、`frontend/src/__tests__/chatStore.test.ts`

> **状态：已完成**（2026-10-25）。实际实现与原计划的差异见各任务下的“实现说明”。

### Task 2.1 后端按 provider 解析凭据

- [x] `SendMessageInput`、`GenerateTitleInput`、`SpawnTaskSessionInput` 改为 `ProviderID uint` + `ModelName string`，移除 `BaseURL` / `ApiKey` / `ProviderType`。
- [x] 新增 `backend/pkg/agent/model_config.go`：`ModelConfig` 与 `Manager.ResolveModelConfig(providerID, modelName)`，配合新增的 `storage.GetProvider`；区分 `ErrProviderRequired`、`ErrModelRequired`、`ErrProviderNotFound`（含已删除）、`ErrProviderDisabled`。
- [x] `ChatHandler.SendMessage` 在 `TryStart` 之后、任何读写之前解析凭据；`runContext`、记忆编码共用解析结果。
- [x] 测试：正常路径凭据来自数据库；4 种无效选择各自返回对应错误；provider 无效时不写入用户消息、释放流占用。
- 实现说明：没有单独定义 `ProviderResolver` 接口——Manager 本来就持有 storage，直接查询即可。`runContext` 保持原有扁平字段，由 `ModelConfig` 填充，避免改动审批相关测试。错误在 service 层统一包装为 `ErrAgentSendMessage`，暂无“provider 已禁用”这类细分的多语言提示，留给 M4 的 i18n 任务。

### Task 2.2 runner 缓存失效与淘汰

- [x] 保留按会话区分 runner：shell、task_state 等工具在构建时就绑定了 `sessionID`，不能跨会话共享。
- [x] 新增 `backend/pkg/agent/runner_cache.go`：并发安全的 LRU，容量 `maxCachedRunners = 32`，替换原来只增不减的 map；去掉了已经没有调用方的 `legacyKey` 查找。
- [x] 缓存 key 改为 `provider ID + 凭据指纹（baseURL、apiKey、类型的 sha256 前 8 字节）+ 模型 + 工具 + 会话 + 版本号`：改了 Key 或 Base URL 会自然不再命中旧 runner，key 中不含明文 Key。
- [x] 测试：改 Key、改 Base URL 后都会得到新 runner，配置不变时复用；key 不含明文 Key；LRU 淘汰顺序、并发写入时保留先写入者、clear、`-race` 下的并发访问。
- 实现说明：没有做 `InvalidateProvider(id)` 以及 provider 服务、`main.go` 之间的接线。凭据指纹已经保证不会用到旧 Key；已删除或禁用的 provider 在解析阶段就会被拒，旧 runner 由 LRU 自然淘汰。

### Task 2.3 标题生成走 oneshot

- [x] `GenerateTitle` 改用 `OneshotComplete`（最多 64 tokens、30 秒超时）：不绑定工具，也不创建 SDK 会话，原先那个从不删除的固定会话 `title-<id>` 不复存在。
- [x] 删除 `fmt.Println`，改用 `log.Printf`（`main.go` 已把标准库 log 接到文件日志）；删除不再使用的 `collectTitleFromEvents` 及其两个测试。
- [x] 测试：标题请求使用库中存储的 Key、请求体不带 `tools`、标题会去除首尾空白并写入数据库；provider 未知时回退为 "New Chat"，不报错（与原行为一致）。

### Task 2.4 前端调用方

- [x] `chatStore` 新增 `ModelRunConfig = { providerId, modelName }`；`sendMessage`、`generateTitle`、`sessionRunConfigs`、`pendingTitleGenerations` 只保存这两个字段；`generateTitle` 改为 `(sessionId, config)`。
- [x] `ChatInput`、`QuickChatApp`、`ChatHeader`、`SettingsApp`（从文档安装 CLI 的任务）同步修改；重新生成时改为按 `providerId` 找回原 provider，不再比对 `base_url`。
- [x] 绑定已重新生成。新增前端测试：多个 provider 提供同名模型时，重新生成会选回原 provider（已通过临时改坏匹配逻辑确认该测试能拦住回归）。
- [x] `grep -rn api_key frontend/src/store` 无结果；聊天相关代码不再读取 Key。

**验收：** ✅ 后端凭据只来自数据库，改 Key 后下一轮对话就会用新 Key（由单元测试覆盖，尚未在打包后的应用里手动验证）；`wails3 task check` 通过（vitest 40 个文件、199 个用例；Go 30 个包）。

**后续项（未包含在 M2 内）：**
- [ ] 列表接口 `ListProviders` / `ProviderAndModelList` 仍会把 `api_key` 返回给前端，主窗口的设置引导（`useSettingsBootstrap`）和 `ChatHeader` 都会拿到 Key。要彻底收敛，需要让列表接口不再返回 Key，并给设置页的编辑视图单独提供读取 Key 的接口。
- [ ] `service/plugin` 的 `loadDefaultChatModel` 每次调用都会 `storage.NewStorage()` 新开一次数据库连接，应改为注入共享的 storage；放到 M3 与 SQLite 连接配置一起处理。
- [ ] 手动回归：在应用里修改 provider 的 Key 后立即发送消息，确认请求使用新 Key；分别验证快速对话、重新生成和从文档安装 CLI 的任务流程。

---

## M3：配置仓库与 SQLite 并发（P1）

**问题：**
1. `config.json` 在 agent、memory、plugin、provider、settings、skills 等 8 处直接读取；`loadAgentConfig` 在每次工具调用、每轮对话都读盘并解析。
2. 用 `strings.Contains(bytes, "\"memory\"")` 判断字段是否存在，比较脆弱。
3. SQLite 未开启 WAL、未设置 `busy_timeout`，流式写消息、记忆编码、终端输出分块并发写入时可能出现 `database is locked`。

**Files:**
- Create: `backend/pkg/configfile/repository.go` + `repository_test.go`
- Modify: `backend/pkg/agent/manager.go`、`tool_confirm.go`、`tool_providers.go`、`tool_settings.go`、`memory_runtime.go`
- Modify: `backend/service/{memory,plugin,provider,settings,skills}/*_internal.go`、`backend/service/plugin/plugin.go`
- Modify: `backend/models/data_models/config.go`（`Memory.Enabled` 改为可区分“未设置”的形式）
- Modify: `backend/storage/storage.go`
- Modify: `main.go`
- Test: `backend/storage/*_test.go`

> **状态：已完成**（2026-10-25）。实际实现与原计划的差异见各任务下的“实现说明”。

### Task 3.1 统一的配置读取

- [x] 新增 `backend/pkg/configfile/load.go`：
  - `Load(dataDir)` 是 config.json 唯一的读取入口，不会创建文件。它统一做三项归一化：`DataDir` 以实际解析出的目录为准；`Extensions` 为 nil 时转为空列表；只有用户显式保存过 `memory.enabled` 时才采用该值，否则记忆默认开启。
  - `LoadCurrent()` 读取应用当前数据目录下的配置。
  - `LoadOrInit(dataDir, defaults)` 在文件不存在时先写入默认配置再读取，写入复用 `Ensure`，不会覆盖已有数据。
  - 文件存在但无法解析时返回 `ErrInvalidConfig`。
- [x] 修复存量 bug：原来用 `strings.Contains(bytes, "\"memory\"")` 判断用户是否设置过记忆开关，只要配置里任何地方出现 `"memory"`（比如某个扩展名叫 memory），就会被当成“已设置”，导致记忆被默认关闭。现在改为检查 JSON 结构。
- [x] 测试：文件缺失时返回默认值且不创建文件；6 种 memory 配置形态（含上述误判场景）；`DataDir` 覆盖与字段保留；非法 JSON 返回 `ErrInvalidConfig`；`LoadOrInit` 只在缺失时写入默认值。
- 实现说明：**没有做内存缓存和 Subscribe**。配置文件只有几 KB，读取加解析是微秒级；而按文件修改时间判断缓存是否失效，在时间戳精度粗的文件系统上，同一秒内写入且大小不变的修改会被漏掉，反而有读到旧配置的风险。Subscribe 目前没有任何使用方。写入仍走原有的 `Update` / `SaveFields` / `Ensure`（带文件锁）。

### Task 3.2 迁移读取方

- [x] `pkg/agent` 删除 `loadAgentConfig`，4 处调用（工具审批、扩展工具集、工具设置、记忆开关）改为 `configfile.LoadCurrent()`。
- [x] `service/settings`、`service/plugin` 改用 `LoadOrInit`，各自保留首次运行写入的默认值；settings 用 `ErrInvalidConfig` 继续区分“解析失败”和“读取失败”两种错误码。
- [x] `service/memory`、`service/skills`、`service/provider` 改用 `LoadCurrent`。
- [x] 验收：`dir.ConfigFileName` 只剩 `configfile` 包、`migration/extension_dirs.go`，以及数据管理页列出文件路径的那一处（只用于展示路径，不读取）。
- 实现说明：各服务继续调用包级函数，没有改成构造函数注入，原因同上：没有缓存可共享，也就没有需要注入的实例。所有改动在同一个提交里。

### Task 3.3 SQLite 连接配置

- [x] `NewStorage` 的 DSN 加上 `_journal_mode=WAL&_busy_timeout=5000&_txlock=immediate`（mattn/go-sqlite3 的参数写法）。
  - `_txlock=immediate` 是必需的：`AppendTerminalOutput` 这类“先读后写”的事务在默认 deferred 模式下，读锁升级为写锁时会直接返回 `SQLITE_BUSY`，不受 busy_timeout 影响。已验证：去掉该参数后，并发测试 20 次全部失败（193 次 `database is locked`）。
  - 没有设置 `SetMaxOpenConns`：WAL 下读不阻塞，写由锁加 busy_timeout 串行化，目前没有限制连接数的必要。
- [x] 测试（新增 `backend/storage/storage_test.go`）：真实文件数据库的 `journal_mode` 为 `wal`、`busy_timeout` 至少 1 秒；8 个 goroutine 并发执行 `CreateMessage`、`AppendTerminalOutput`、`CreateMemory` 共 200 次写入且无报错，每个终端的输出分块序号连续、游标正确。修复前该测试稳定复现 `database is locked`，修复后 `-count=20` 稳定通过。
- [x] `-wal` / `-shm` 文件：代码中没有复制或清理 `data.db` 的逻辑（只有数据管理页展示 `data.db` 路径），因此不受影响。旧数据库在首次打开时由 SQLite 自动切换为 WAL 模式。
- [x] 附带修复：`backend/storage` 测试辅助函数 `newTestStorage` 从不关闭共享缓存的内存数据库，导致 `-count` 大于 1 时上一轮的数据残留、多个用例失败（与本次改动无关的存量问题）。现在在 cleanup 中关闭连接，`-race -count=5` 稳定通过。

### Task 3.4 plugin 复用共享 storage（M2 后续项）

- [x] `loadDefaultChatModel` 原来每次调用都 `storage.NewStorage()` 新开数据库，而且从不关闭。现在新增 `NewPluginWithStorage(stor)`（沿用 `NewTerminalWithManager` 的写法），`main.go` 注入共享 storage；`NewPlugin()` 不带 storage，解析默认模型时返回 `errPluginStorageUnavailable`，不再私自开库。
- [x] 测试：通过注入的 storage 解析出默认 provider 及其默认模型；不带 storage 时返回明确错误。

**验收：** ✅ `storage.NewStorage()` 只在 `main.go` 调用一次；并发写测试 `-count=20` 稳定通过；`wails3 task check` 通过（vitest 40 个文件、199 个用例；Go 30 个包）。

**后续项（未包含在 M3 内）：**
- [ ] 手动验证：用旧版本留下的 `data.db` 启动新版本，确认能自动切到 WAL 并正常读写；在应用里同时进行流式对话和终端命令，确认终端输出没有丢失。
- [ ] `pkg/terminal` 的 `drain` 在 `AppendTerminalOutput` 出错时直接 `continue`，静默丢弃输出。锁冲突修复后这种情况应该很少出现，但仍建议至少记录日志。

---

## M4：后端结构整理与 i18n（P2）

> **状态：已完成**（2026-10-25，开始前已合并 master 的 PR #4）。实际实现与原计划的差异见各任务下的“实现说明”。

### Task 4.1 Manager 依赖改为构造注入

- [x] 新增 `ManagerDeps`，通过 `NewManagerWithDeps(stor, deps)` 在构造时一次性注入 8 个依赖，删除 8 个 setter 和没有调用方的 `SkillProvider()` getter。依赖构造后不再变化，工具工厂读取时无需加锁。
- [x] `toolDependenciesVersion` 改名为 `appVersion`，只在 `SetApp` 时递增（Wails 应用句柄仍在 `ServiceStartup` 时注入）。
- [x] `agentSvc.Dependencies` 改为 `pkgAgent.ManagerDeps` 的类型别名，`NewAgent(stor, deps ...)` 拆成 `NewAgent(stor)` 和 `NewAgentWithDeps(stor, deps)`，原来 30 行逐个判空调用 setter 的代码随之删除。
- [x] 测试：原“替换服务后 runner 失效”的测试改为验证 `SetApp`；新增“注入依赖后对应工具可用”的测试。
- 实现说明：保留 `NewManager(stor)` 作为不带依赖的构造函数（沿用 M3 里 `NewPluginWithStorage` 的写法），约 70 处测试调用无需改动。

### Task 4.2 拆分 `ChatHandler.SendMessage`

- [x] 拆成 `prepareTurn`（解析凭据、重新生成、加载历史，全程只读）、`resolveSessionPrompt`（工具选择和系统提示词的锁定与默认值）、`persistUserMessage`、`startRun`（runner、事件消费、清理）。主函数只负责编排顺序和释放流占用。
- [x] 两次 `CountMessagesForSession` 合并为一次，并且在“没有指定提示词且会话已有提示词”的常见路径上直接跳过查询。已逐个分支核对与原逻辑等价：两次计数之间没有任何写消息的操作。
- [x] 现有测试一个断言都没改，全部通过（`-count=3` 和 `-race` 均通过）。

### Task 4.3 硬编码中文改为错误码/i18n

- [x] storage / pkg 层改为英文哨兵错误：`storage.ErrSystemPromptLocked`（新增）、`storage.ErrSessionToolsLocked`、`agent.ErrSessionBusy`（新增，`TryStart` 和 `WhileInactive` 共用）、`systemprompt.ErrInvalidName` / `ErrInvalidContent`。
- [x] service 层新增 `agentError(fallback, err)`：用 `errors.Is` 把已知原因映射为专门的错误码，这样提示文字本身就是本地化后的具体原因，而不是“发送消息失败”再附一句英文。顺带补上了 M2 留下的“provider 不存在或已停用”提示。
- [x] `system_prompt.go`、`fork.go` 的公开方法改为全部返回 `ierror`（原来直接返回中文 `fmt.Errorf`，违反项目规则）。
- [x] 写入数据库的标签（`prompt_name` 默认值和自定义值、分叉会话标题的后缀）改为按当前语言调用 `i18n.TCurrent` 取值，key 定义为 `storage.PromptNameDefaultKey` / `PromptNameCustomKey`。历史数据不受影响。
- [x] 新增 15 个错误码和 3 个标签的中英文文案；新增 `i18n_test.go`，检查两种语言的 key 集合完全一致。
- [x] 测试：错误映射；无效 provider 时提示具体原因；提示词为空、名称过长、分叉目标无效时返回本地化提示；英文环境下默认提示词名称和分叉后缀为英文。
- 实现说明：
  - `chat_handler.go` 中“工具执行已中断……”这句是写进对话历史、发给模型看的，不是给用户看的，保持不动。
  - `service/config` 里的语言名称（如“简体中文”）本来就该显示原文，保持不动。
  - 这次没有改前端 `locales`：后端文案在后端翻译，前端直接显示 `msg`。

### Task 4.4 事件类型化

- [x] 新增 `backend/pkg/agent/events.go`：8 个对话事件（`agent:session:status/spawned`、`agent:stream:chunk/tool_call/tool_result/confirm_request/done/error`）的载荷结构体、`RegisterEvent` 注册，以及每个事件一个发送函数。11 个原来直接发送 `map` 的地方全部改为调用这些函数；`ChatHandler` 和 `Manager` 里重复的 `updateSessionStatus` 合并为一个。
- [x] **风险控制**：Wails 遇到类型不匹配的载荷时会直接取消这次事件，漏改一处就会导致对话界面收不到消息。为此新增两个测试：
  - 结构性测试：扫描后端源码，只要在 `events.go` 之外直接引用这 8 个事件名就失败（迁移前它正确列出了全部 11 处）；
  - 格式测试：锁定每个载荷序列化后的 JSON，与原来 map 的格式逐字一致，前端解析逻辑不受影响。
- [x] 重新生成绑定；`chatStore` 的 8 个监听改用 `bindings/events` 导出的常量。生成的类型声明让 TS 在编译期就知道每个事件的数据类型；`contentType`、`status` 这两个字段在 Go 里是普通 string，在回调边界处收窄为前端的联合类型。
- [x] 3 个前端测试的 `@wailsio/runtime` mock 改为 `importOriginal` 加覆盖 `Events`（项目其他测试已在用这种写法），因为生成的事件绑定会在导入时调用真实的 `Create` 工具函数。
- 实现说明：只做了计划里的 8 个对话事件。`cli.install.*`、`cli.login.*`、`runtime.node.progress`、`terminal.*`、`notification.*`、`quick_chat:*` 等约 12 个事件仍未注册，列为后续项。

**验收：** ✅ `main.go` 中 agent 的组装由逐个调用 setter 改为一次构造；后端已没有面向用户的硬编码中文（发给模型的提示和语言名称除外）；`wails3 task check` 通过（vitest 40 个文件、199 个用例；Go 31 个包）。

**后续项（未包含在 M4 内）：**
- [ ] 手动验证：在打包后的应用里完整跑一轮对话（流式输出、工具调用和审批、停止、重新生成、生成标题、后台任务会话），确认 8 个事件都能正常到达前端；切换到英文后，分别触发系统提示词已锁定、provider 已停用、分叉这几种情况，确认提示文字和标签都是英文。
- [ ] 其余约 12 个事件按同样方式注册类型。
- [ ] **前端还有大量硬编码中文**（`SystemPromptSettings.tsx`、`MessageActions.tsx` 里的 `zh ? … : …` 内联三元、`chatStore.ts` 里抛出的错误文本等），没有走 i18n，建议并入 M5。

---

> **状态：已完成**（2026-10-26），并入了 M4 记录的“前端硬编码中文国际化”。三个大文件的拆分都是用脚本按行号原样搬移代码，并且先断言边界行的内容，任何一处不符就中止、不写文件；拆完后再逐行比对，确认除有意的改动外原代码都保留了下来。

### Task 5.0 前端硬编码中文改为 i18n（M4 后续项）

- [x] `SystemPromptSettings.tsx`（28 处）、`MessageActions.tsx`（原来是 `zh ? … : …` 内联三元）、`chatStore` 抛出的 6 个错误、`MessageItem` 的中断/失败提示，以及侧边栏折叠/展开、设置页“返回”、模型选择按钮的 title，全部改为 i18n key。中文文案与原来逐字一致，现有测试里断言中文的地方不受影响。
- [x] `utils.ts` 的日期：“10月25日 / Oct 25”改用 `Intl.DateTimeFormat` 生成（已核对与原来手写的格式逐字一致），“今天 / 昨天”改为带插值的 key，会话分组标题复用 `sidebar.*` 里已有的 key。
- [x] 附带修复：`SystemPromptSettings` 和 `MessageActions` 原来直接显示 `e.message`，后端返回的错误会显示成一段原始 JSON；现在改用 `parseUnknownIError` 只显示本地化后的 `msg`。
- [x] 删除没有任何引用的 `src/mock/data.ts`。
- [x] 新增 `localeKeys.test.ts`，检查中英文 locale 的 key 集合完全一致；给 `formatTimestamp`、`groupConversationsByDate` 补测试，覆盖两种语言下的今天、昨天、同年、跨年四种情况。
- 保留的中文：`ErrorMessageBubble` 里用来匹配后端两种语言返回值的字符串；`useSettingsBootstrap` 里后端数据加载前的占位值（语言名“简体中文”、地区名）。

### Task 5.1 拆分 `chatStore.ts`（1076 行）

- [x] 拆为 `store/chat/` 下的 6 个文件：`types.ts`、`helpers.ts`、`streamingBuffer.ts`（流式缓冲与逐帧合并，做成工厂函数供各模块共享）、`sessionSlice.ts`（会话列表、工具选择、消息加载）、`turnSlice.ts`（发送、重新生成、停止、审批、标题）、`chatEvents.ts`（8 个事件监听）。`store/chatStore.ts` 只剩 19 行的组装代码，`useChatStore` 的导入路径不变。
- [x] 各模块的返回类型用 `Pick` / `Omit` 从 `ChatStore` 精确推出，组装时 TS 会检查字段是否齐全。
- [x] 逐行比对：原文件中只有 17 行不是原样保留，全部是有意改动（加 `export`、替换外层 `create(...)` 包装、事件方法头改为工厂函数）。
- [x] `chatStore.test.ts`（29 个用例）、`chatPerformance.test.tsx` 断言未改，全部通过。
- 实现说明：没有按原计划拆出单独的 `titleSlice`。标题生成和发送共用 `pendingTitleGenerations` / `sessionRunConfigs`，并入 `turnSlice` 更内聚。

### Task 5.2 多窗口入口

- [x] `App.tsx` 改为 `ENTRIES` 映射表，每个窗口入口（含引导页和主界面）都用 `React.lazy` 按需加载，外层包一个 `Suspense`。
- [x] 体积对比：主 chunk `index-*.js` 从 **1,505 KB（gzip 463 KB）降到 291 KB（gzip 96 KB）**。各窗口现在只加载自己的 chunk：快速对话 5 KB、添加 provider 5 KB、设置页 107 KB、主界面 45 KB 加对话区。原来构建时报的 `INEFFECTIVE_DYNAMIC_IMPORT` 警告也消失了。
- [x] `onboardingApp.test.tsx` 的首次断言从 `getByText` 改为 `await findByText`：入口改为按需加载后，首帧渲染本来就是异步的。这是本里程碑唯一一处测试断言的改动。

### Task 5.3 大组件拆分

- [x] `ChatInput.tsx` 从 801 行降到 457 行。拆出 `chat/input/editorExtensions.ts`（两个 tiptap 扩展和代码块转换，原样搬移）、`ToolSelector.tsx`、`ModelSelector.tsx`（纯展示组件，状态和回调由 `ChatInput` 传入，处理逻辑不变）、`useComposerAttachments.ts`（附件状态、选择文件、粘贴、浏览器拖放和 macOS 原生拖放）；`blobToBase64` 移到 `lib/attachments.ts`。
- [x] `SettingsApp.tsx` 从 717 行降到 227 行。拆出：
  - `usePluginSettings`：插件页的状态和 12 个处理函数。它在 `SettingsApp` 顶层**无条件调用**，所以切换标签页时状态不丢失，窗口卸载时仍会取消进行中的 CLI 登录（如果把状态移进插件页组件，切换标签就会误取消登录）。
  - `PluginsSettingsSection`：插件页的 JSX。
  - `useRuntimeDownloadReminder(active)`：运行时下载提醒、进度提示的两个 effect 和相关格式化函数。
- [x] 逐行比对两个文件的拆分结果，未原样保留的行全部是 props 改名、回调转移、import 调整和 `export`。
- [x] 对应组件测试（`chatInputTools`、`chatInputAttachments`、`pluginSettings`、`settingsApp`）断言未改，全部通过。

**验收：** ✅ `wails3 task check` 通过（vitest 41 个文件、202 个用例；Go 31 个包）。拆分过的文件都降到 400 行左右以内，只有 `ChatInput.tsx` 还有 457 行，剩下的主要是编辑器初始化和数据加载的 effect。除 Task 5.2 说明的那一处外，测试断言均未修改。

**后续项（未包含在 M5 内）：**
- [ ] 手动验证：在打包后的应用里分别打开主界面、设置、快速对话、添加 provider / Skill / 记忆这几个窗口，确认按需加载都正常；切到英文后检查系统提示词设置弹窗、消息操作按钮、时间戳和会话分组标题；在设置页插件标签发起 CLI 登录后切换到其他标签再切回，确认登录没有被中断。
- [ ] 对话区 chunk `ChatArea-*.js` 仍有 937 KB（gzip 303 KB），主要来自 `rehype-highlight` 默认引入的 highlight.js 全量语言包，以及 tiptap 编辑器。可以改为只注册常用语言，或把代码高亮再拆成异步加载的 chunk。
- [ ] `useSettingsBootstrap` 里的地区名（“中国 / 美国”）是后端 `service/config` 返回值加载前的占位，后端返回的地区列表本身也只有中文，需要前后端一起本地化。

---

## M6：静态检查进 CI（P3）

> **状态：已完成**（2026-10-26，分支 `optimize-m6-lint`，基于 M5 单独开 PR）。

- [x] 前端接入 ESLint 10 flat config（`frontend/eslint.config.js`）：`@eslint/js` 与 `typescript-eslint` 的 recommended 规则、`no-explicit-any` 设为 error，`react-hooks` 只启用经典的 `rules-of-hooks` / `exhaustive-deps` 两条。`npm run lint` 带 `--max-warnings 0`，警告同样会导致失败。依赖版本固定为发布满两周的版本：`eslint@10.12.0`、`@eslint/js@10.0.1`、`typescript-eslint@8.71.1`、`eslint-plugin-react-hooks@7.1.1`、`globals@17.13.0`。
- [x] 修完前端全部 19 处存量问题：
  - 6 处 `any`：两处绑定调用上的 `as any` 本来就是多余的；tiptap Suggestion 的参数补上 `Editor` / `Range` 类型；测试里的 markdown 渲染函数给出精确签名。
  - `SystemPromptSettings` 的 `promptLabel` 改为模块级函数，effect 的依赖如实声明；`ChatInput` 补上稳定的 `setAttachments` 依赖。
  - 有意为之的写法加 disable 注释并写明原因：两处 ANSI 转义正则；`useAutoScroll` 由调用方传入依赖数组；`ShortcutSettingsView` 只在开始/结束录制时重新订阅键盘事件。
  - 删除 3 条已经不起作用的 disable 注释；`vite.config.ts` 不在 tsc 检查范围内，单独关闭 `ban-ts-comment`。
- [x] 后端接入 golangci-lint v2.14.0（`.golangci.yml`），启用 `govet`、`staticcheck`、`errcheck`、`ineffassign`、`unused`。存量 65 处（macOS）加上 5 处只在 Windows 下出现：
  - **2 处真问题**：Node 运行时下载写入后的 `Close`、MCP 导入复制文件后的 `Close` 原来都被忽略。前者的 SHA256 是对数据流算的，写盘失败也能通过校验；后者会悄悄留下被截断的文件。现在都显式检查错误。
  - errcheck 统一豁免只读或清理性质的 `Close` / `Remove`（含 `CloseHandle`），测试文件不做 errcheck；配置注释里写明，写入后的 `Close` 仍必须显式检查。
  - 删除 3 个死代码函数（两个平台的 `quickChatModifiers` 和 `formatMemoryContext`）；`staticcheck` 的 8 处写法建议照改；测试里永远不成立的 `runtime.GOOS == "windows"` 分支删除；Windows 下空实现的登录字段用带原因说明的 `//nolint:unused` 标注。
  - macOS 和 `GOOS=windows` 交叉检查均为 0 个问题；Linux 因 Wails 依赖 cgo / GTK 无法在本机交叉检查，CI 也只跑 Windows。
- [x] CI 测试 job：Frontend checks 在 `npm ci` 后运行 `npm run lint`；新增 Backend lint 步骤（`golangci/golangci-lint-action@v9`，版本 v2.14.0，参数 `./backend/... .`）；`pull_request.paths` 加上 `.golangci.yml`。
- [x] `Taskfile.yml`：`check:frontend` 加入 `npm run lint`，新增 `check:lint`（说明里写明安装命令和固定版本），`check` 依次运行三个子任务。

**验收：** ✅ 本地 `wails3 task check` 通过（ESLint 0 个问题；vitest 41 个文件、202 个用例；Go 31 个包；golangci-lint 0 个问题），流程与 CI 一致。PR 上的检查结果以 CI 为准。

**后续项：**
- [ ] 仓库里有不少存量 Go 文件没有经过 gofmt，这次没有启用 `gofmt` / `goimports` 格式化检查，以免产生大量与本次无关的改动。可以单独提一个只做格式化的 PR，再在 `.golangci.yml` 里开启 `formatters`。
- [ ] `eslint-plugin-react-hooks` 7 新增的 React Compiler 相关规则暂未启用，可以在以后单独评估。

---

## 风险与注意事项

- **与 `cf9e9db` 的重叠**：M2（`main.go`、`logger`）、M3（`pkg/dir`、`service/settings`、`configfile` 读取方）、M5（`SettingsApp.tsx`、`settingsStore`）都会碰到该提交改过的文件。该提交已移除 `ApplyFileSettings` 和“通用 → 文件”子页，并让测试进程默认使用临时数据目录（`FIRE_POKER_DATA_DIR`），编写新测试时沿用这一约定。

- **M2 是破坏性的绑定变更**：后端 DTO、生成的绑定、前端调用方必须在同一个 PR 里一起合并，否则应用无法发送消息。
- **M1 可能增加确认弹窗**：合并前用日常任务手动回归一遍，确认读类操作没有被误伤；必要时补充安全名单。
- **M3 的 WAL 模式**会在数据目录生成 `-wal`/`-shm` 文件，需确认数据目录迁移、备份、重置逻辑能正确处理。
- **M4、M5 是纯重构**：如果发现需要改行为，单独拆成修复 PR，不要混在重构里。
