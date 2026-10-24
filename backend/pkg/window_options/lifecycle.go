package window_options

import (
	"github.com/reatang/fire-poker/backend/pkg/id/window_id"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// NewHomeWindow creates the main window whose close button exits the application,
// even while the preloaded quick chat or a settings window is still alive.
func NewHomeWindow(app *application.App) *application.WebviewWindow {
	win := app.Window.NewWithOptions(DefaultHome())
	bindRootWindowClose(app, win)
	return win
}

// NewOnboardingWindow exits on close unless onboarding is handing off to home.
func NewOnboardingWindow(app *application.App) *application.WebviewWindow {
	win := app.Window.NewWithOptions(DefaultOnboarding())
	bindRootWindowClose(app, win)
	return win
}

func bindRootWindowClose(app *application.App, win *application.WebviewWindow) {
	win.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		_, hasHome := app.Window.GetByName(window_id.Home)
		closeRootWindow(event, win.Name(), hasHome, app.Quit)
	})
}

func closeRootWindow(event *application.WindowEvent, name string, hasHome bool, quit func()) {
	if name != window_id.Home && (name != window_id.Onboarding || hasHome) {
		return
	}
	// Quit owns window destruction and service shutdown. Cancel the ordinary
	// single-window close so it cannot race the application's teardown.
	event.Cancel()
	quit()
}
