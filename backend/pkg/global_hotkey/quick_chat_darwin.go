//go:build darwin

package global_hotkey

import "golang.design/x/hotkey"

// quickChatModifierForToken maps a shortcut token to the macOS hotkey modifier.
func quickChatModifierForToken(token string) (hotkey.Modifier, bool) {
	switch token {
	case "CmdOrCtrl", "Cmd":
		return hotkey.ModCmd, true
	case "Ctrl":
		return hotkey.ModCtrl, true
	case "Alt":
		return hotkey.ModOption, true
	case "Shift":
		return hotkey.ModShift, true
	default:
		return 0, false
	}
}
