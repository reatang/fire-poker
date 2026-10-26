//go:build windows

package global_hotkey

import "golang.design/x/hotkey"

// quickChatModifierForToken maps a shortcut token to the Windows hotkey modifier.
func quickChatModifierForToken(token string) (hotkey.Modifier, bool) {
	switch token {
	case "CmdOrCtrl", "Ctrl":
		return hotkey.ModCtrl, true
	case "Cmd":
		return hotkey.ModWin, true
	case "Alt":
		return hotkey.ModAlt, true
	case "Shift":
		return hotkey.ModShift, true
	default:
		return 0, false
	}
}
