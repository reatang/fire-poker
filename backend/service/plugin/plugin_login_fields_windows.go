//go:build windows

package plugin

// loginFields is an empty stub on Windows. CLI login sessions use a PTY which is
// not supported on Windows; the feature is gated by //go:build !windows on
// plugin_login.go so these fields are never accessed at runtime on this platform.
type loginFields struct{} //nolint:unused // embedded in Plugin so both platforms share one struct layout

// initLoginFields is a no-op on Windows.
func initLoginFields(p *Plugin) {}
