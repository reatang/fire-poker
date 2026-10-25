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
| M4 | 后端结构整理与 i18n | P2 | M2、M3 | 是（事件类型） |
| M5 | 前端结构整理 | P2 | M2（同改 `chatStore`） | 否 |
| M6 | 静态检查进 CI | P3 | M4、M5 | 否 |

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

纯重构为主，行为保持不变。

### Task 4.1 Manager 依赖改为构造注入

- [ ] 用一个 `ManagerDeps` 结构体在 `NewManager` 一次性注入 8 个依赖，删除对应 setter 及 `toolDependenciesVersion` 的递增逻辑（`SetApp` 视 Wails 生命周期保留）。
- [ ] `agentSvc.NewAgent(stor, deps ...Dependencies)` 改为普通参数。
- [ ] 测试辅助函数同步调整（`newTestXxx` 模式）。

### Task 4.2 拆分 `ChatHandler.SendMessage`

- [ ] 拆成 `prepareTurn`（重新生成、历史加载）、`resolveSystemPrompt`（锁定与默认值）、`persistUserMessage`、`startRun`（runner、事件消费、清理）。
- [ ] 合并重复的 `CountMessagesForSession` 查询。
- [ ] 现有 `chat_handler_test.go`、`regenerate_test.go`、`resume_test.go` 不改断言即可通过。

### Task 4.3 硬编码中文改为错误码/i18n

- [ ] `storage/session.go:58`、`storage/session_tools.go:13` 改为 `ierror` 错误码，前端按码翻译。
- [ ] `storage/fork.go` 的“ · 分叉”、`storage/default_prompt.go` 与 `chat_handler.go` 的“默认系统提示词”/“自定义系统提示词”改为通过 `i18n.TCurrent` 取值，或改存稳定 key、前端展示时翻译。
- [ ] `i18n/resources_*` 与前端 `locales` 同步补充文案。

### Task 4.4 事件类型化

- [ ] 对 `agent:stream:*`、`agent:session:*` 等事件使用 `application.RegisterEvent[T]` 注册载荷类型。
- [ ] 重新生成绑定，前端改为引用 `bindings/events` 导出的事件名常量与类型，去掉字符串字面量。

**验收：** `main.go` 中 agent 相关组装更短；后端无面向用户的硬编码中文（注释除外）。

---

## M5：前端结构整理（P2）

### Task 5.1 拆分 `chatStore.ts`（1094 行）

- [ ] 按 zustand slice 拆为 `sessionSlice`（会话增删改查）、`streamSlice`（流式缓冲与逐帧合并）、`titleSlice`（标题生成），事件订阅移到 `store/chatEvents.ts`。
- [ ] `chatStore.test.ts`、`chatPerformance.test.tsx` 保持通过。

### Task 5.2 多窗口入口

- [ ] `App.tsx` 改为 `entry → Component` 映射表，配合 `React.lazy` + `Suspense` 按窗口拆包。
- [ ] 对比改造前后 `vite build` 主 chunk 体积，写进 PR 描述。

### Task 5.3 大组件拆分

- [ ] `ChatInput.tsx`（801 行）：编辑器初始化、附件、工具选择、快捷键提取为 hooks/子组件。
- [ ] `SettingsApp.tsx`（717 行）：各设置面板的状态与路由拆分。
- [ ] 对应组件测试保持通过。

**验收：** 单文件不超过约 400 行；行为测试无断言修改。

---

## M6：静态检查进 CI（P3）

- [ ] 前端加 ESLint（`@typescript-eslint`，开启 `no-explicit-any`），先修存量问题再进 CI。
- [ ] 后端加 `golangci-lint`（先启用 `govet`、`staticcheck`、`errcheck`、`ineffassign`），配置写进 `.golangci.yml`。
- [ ] `.github/workflows/release-build.yml` 的测试 job 增加两项检查；`Taskfile.yml` 的 `check` 任务同步加入。

**验收：** PR 上 lint 检查通过；本地 `task check` 与 CI 一致。

---

## 风险与注意事项

- **与 `cf9e9db` 的重叠**：M2（`main.go`、`logger`）、M3（`pkg/dir`、`service/settings`、`configfile` 读取方）、M5（`SettingsApp.tsx`、`settingsStore`）都会碰到该提交改过的文件。该提交已移除 `ApplyFileSettings` 和“通用 → 文件”子页，并让测试进程默认使用临时数据目录（`FIRE_POKER_DATA_DIR`），编写新测试时沿用这一约定。

- **M2 是破坏性的绑定变更**：后端 DTO、生成的绑定、前端调用方必须在同一个 PR 里一起合并，否则应用无法发送消息。
- **M1 可能增加确认弹窗**：合并前用日常任务手动回归一遍，确认读类操作没有被误伤；必要时补充安全名单。
- **M3 的 WAL 模式**会在数据目录生成 `-wal`/`-shm` 文件，需确认数据目录迁移、备份、重置逻辑能正确处理。
- **M4、M5 是纯重构**：如果发现需要改行为，单独拆成修复 PR，不要混在重构里。
