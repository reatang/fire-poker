package main

import (
	"context"
	"embed"
	_ "embed"
	"io"
	"log"
	"log/slog"
	"os"

	"github.com/reatang/fire-poker/backend/pkg/dir"
	"github.com/reatang/fire-poker/backend/pkg/global_hotkey"
	"github.com/reatang/fire-poker/backend/pkg/i18n"
	"github.com/reatang/fire-poker/backend/pkg/id/window_id"
	"github.com/reatang/fire-poker/backend/pkg/logger"
	"github.com/reatang/fire-poker/backend/pkg/migration"
	"github.com/reatang/fire-poker/backend/pkg/prompt"
	"github.com/reatang/fire-poker/backend/pkg/skills"
	pkgterminal "github.com/reatang/fire-poker/backend/pkg/terminal"
	"github.com/reatang/fire-poker/backend/pkg/window_options"
	agentSvc "github.com/reatang/fire-poker/backend/service/agent"
	"github.com/reatang/fire-poker/backend/service/config"
	"github.com/reatang/fire-poker/backend/service/file"
	memorySvc "github.com/reatang/fire-poker/backend/service/memory"
	notificationSvc "github.com/reatang/fire-poker/backend/service/notification"
	onboardingSvc "github.com/reatang/fire-poker/backend/service/onboarding"
	"github.com/reatang/fire-poker/backend/service/onboarding/onboarding_dto"
	pluginSvc "github.com/reatang/fire-poker/backend/service/plugin"
	"github.com/reatang/fire-poker/backend/service/process"
	"github.com/reatang/fire-poker/backend/service/provider"
	runtimeSvc "github.com/reatang/fire-poker/backend/service/runtime"
	"github.com/reatang/fire-poker/backend/service/settings"
	skillsSvc "github.com/reatang/fire-poker/backend/service/skills"
	terminalSvc "github.com/reatang/fire-poker/backend/service/terminal"
	"github.com/reatang/fire-poker/backend/service/window"
	"github.com/reatang/fire-poker/backend/service/window/window_dto"
	"github.com/reatang/fire-poker/backend/storage"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var brandIcon []byte

func init() {
	application.RegisterEvent[string]("time")
}

func main() {
	dataDir, err := dir.GetDataDir()
	if err != nil {
		log.Fatal(err)
	}
	setupFileLogging(dataDir)

	istorage, err := storage.NewStorage()
	if err != nil {
		log.Fatal(err)
	}
	if err := istorage.PinLegacyDefaultPrompts(prompt.LegacyDefaultMainAgentPrompt); err != nil {
		log.Fatal(err)
	}

	if result, merr := migration.MigrateExtensionDirs(dataDir); merr != nil {
		log.Printf("extension dir migration failed: %v", merr)
	} else if len(result.Warnings) > 0 {
		for _, w := range result.Warnings {
			log.Printf("extension dir migration warning: %s", w)
		}
	}
	for _, w := range migration.MigrateCacheDirs(dataDir) {
		log.Printf("cache dir migration warning: %s", w)
	}

	providerService := provider.NewProvider(istorage)
	onboardingService := onboardingSvc.NewOnboarding(providerService)

	// Create skills manager (needs the skills root directory).
	skillsManager := skills.NewManager(dir.SkillsRoot(dataDir))
	if err := skillsManager.Refresh(nil); err != nil {
		// Log warning but don't fail startup — skills are optional.
		log.Printf("warning: failed to refresh skills: %v", err)
	}

	// Create the plugin and agent services — plugin must come first so its CLI bridge can be wired in.
	pluginService := pluginSvc.NewPluginWithStorage(istorage)
	memoryService := memorySvc.NewMemory(istorage)
	notificationService := notificationSvc.NewNotification(istorage)
	terminalManager := pkgterminal.NewManager(istorage, nil)
	terminalService := terminalSvc.NewTerminalWithManager(terminalManager)
	windowService := &window.Window{}
	quickChatShortcut, shortcutErr := settings.LoadQuickChatShortcutForStartup()
	if shortcutErr != nil {
		log.Printf("load quick chat shortcut failed, using default: %v", shortcutErr)
		quickChatShortcut = global_hotkey.DefaultQuickChatShortcut()
	}
	quickChatService := global_hotkey.NewQuickChatServiceWithShortcut(func() {
		log.Printf("quick chat global hotkey pressed")
		if _, err := windowService.ToggleQuickChat(context.Background(), window_dto.ToggleQuickChatInput{}); err != nil {
			log.Printf("quick chat toggle failed: %v", err)
		}
	}, quickChatShortcut)
	settingsService := settings.NewSettings(settings.Dependencies{QuickChatShortcuts: quickChatService, SessionToolDefaults: istorage})
	agentService := agentSvc.NewAgentWithDeps(istorage, agentSvc.Dependencies{
		SkillProvider:        skillsManager,
		AttentionRequester:   notificationService,
		SkillCreator:         skillsManager,
		CliInstaller:         pluginService,
		CliInstallProgress:   pluginService,
		CliManifestGenerator: pluginService,
		CliCommandRunner:     pluginService,
		TerminalRunner:       terminalManager,
	})

	app := application.New(application.Options{
		Name:        "Fire Poker",
		Description: "一个经典模式的智能体客户端",
		Icon:        brandIcon,
		// Wails logs every asset request at info level; keep only warnings so the daily log stays readable.
		Logger: slog.New(slog.NewTextHandler(log.Writer(), &slog.HandlerOptions{Level: slog.LevelWarn})),
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: dir.BundleIdentifier,
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				focusMainWindow()
			},
		},
		Services: []application.Service{
			application.NewService(settingsService),
			application.NewService(agentService),
			application.NewService(providerService),
			application.NewService(onboardingService),
			application.NewService(quickChatService),
			application.NewService(runtimeSvc.NewRuntime()),
			application.NewService(pluginService),
			application.NewService(terminalService),
			application.NewService(&process.Process{}),
			application.NewService(windowService),
			application.NewService(&file.File{}),
			application.NewService(&config.Config{}),
			application.NewService(skillsSvc.NewSkills(skillsManager)),
			application.NewService(memoryService),
			application.NewService(notificationService),
		},
		KeyBindings: map[string]func(window application.Window){
			quickChatShortcut: func(window application.Window) {
				log.Printf("quick chat app-level shortcut pressed")
				if _, err := windowService.ToggleQuickChat(context.Background(), window_dto.ToggleQuickChatInput{}); err != nil {
					log.Printf("quick chat toggle failed: %v", err)
				}
			},
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Windows: application.WindowsOptions{
			WebviewUserDataPath: migration.WebViewDataDir(dataDir),
		},
	})

	initState, err := onboardingService.IsInitialized(context.Background(), onboarding_dto.IsInitializedInput{})
	if err != nil {
		log.Fatal(err)
	}

	if initState != nil && initState.Initialized {
		win := window_options.NewHomeWindow(app)
		win.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
			application.Get().Event.Emit("files-dropped", map[string]any{
				"files": event.Context().DroppedFiles(),
			})
		})
	} else {
		window_options.NewOnboardingWindow(app)
	}
	quickChatOptions := window_options.DefaultQuickChat()
	quickChatOptions.Title = i18n.TCurrent("app.window.quick_chat_title", nil)
	app.Window.NewWithOptions(quickChatOptions)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// logRetentionDays is how many days of daily log files are kept in {dataDir}/logs.
const logRetentionDays = 14

// setupFileLogging mirrors the standard logger into daily files, since GUI launches discard stderr.
func setupFileLogging(dataDir string) {
	fileLogger, err := logger.New(logger.Options{DataDir: dataDir})
	if err != nil {
		log.Printf("file logging disabled: %v", err)
		return
	}
	log.SetOutput(io.MultiWriter(os.Stderr, fileLogger))
	if err := fileLogger.Prune(logRetentionDays); err != nil {
		log.Printf("prune old logs failed: %v", err)
	}
}

// focusMainWindow brings the existing home or onboarding window forward when the app is launched again.
func focusMainWindow() {
	app := application.Get()
	if app == nil {
		return
	}
	for _, name := range []string{window_id.Home, window_id.Onboarding} {
		if win, ok := app.Window.GetByName(name); ok {
			if win.IsMinimised() {
				win.UnMinimise()
			}
			win.Show()
			win.Focus()
			return
		}
	}
}
