package app

import (
	"fmt"
	"strings"
)

var wailsModifiers = map[string]string{
	"mod":   "CmdOrCtrl",
	"ctrl":  "Ctrl",
	"alt":   "Alt",
	"shift": "Shift",

	"cmdorctrl":   "CmdOrCtrl",
	"cmd":         "Cmd",
	"command":     "Cmd",
	"super":       "Super",
	"win":         "Super",
	"option":      "Alt",
	"optionoralt": "Alt",
}

var wailsNamedKeys = map[string]string{
	"space":          "space",
	"escape":         "escape",
	"esc":            "escape",
	"enter":          "enter",
	"return":         "enter",
	"backspace":      "backspace",
	"tab":            "tab",
	"delete":         "delete",
	"arrowup":        "up",
	"arrowdown":      "down",
	"arrowleft":      "left",
	"arrowright":     "right",
	"home":           "home",
	"end":            "end",
	"pageup":         "page up",
	"page up":        "page up",
	"pagedown":       "page down",
	"page down":      "page down",
	"forwarddelete":  "forward delete",
	"forward delete": "forward delete",
	"numlock":        "numlock",
	"plus":           "plus",

	"equal": "=",
	"minus": "-",
}

func toWailsAccelerator(binding string) (string, error) {
	binding = strings.TrimSpace(binding)
	if binding == "" {
		return "", nil
	}

	if strings.HasSuffix(binding, "++") {
		binding = strings.TrimSuffix(binding, "+") + "Plus"
	}

	parts := strings.Split(binding, "+")
	if len(parts) < 2 {
		return "", invalidAcceleratorError(binding, "缺少修饰键")
	}

	key := parts[len(parts)-1]
	modifiers := make([]string, 0, len(parts)-1)
	seen := make(map[string]struct{}, len(parts)-1)
	for _, raw := range parts[:len(parts)-1] {
		modifier, ok := wailsModifiers[strings.ToLower(strings.TrimSpace(raw))]
		if !ok {
			return "", invalidAcceleratorError(binding, fmt.Sprintf("未知修饰键 %q", raw))
		}
		if _, duplicated := seen[modifier]; duplicated {
			continue
		}
		seen[modifier] = struct{}{}
		modifiers = append(modifiers, modifier)
	}
	if len(modifiers) == 0 {
		return "", invalidAcceleratorError(binding, "缺少修饰键")
	}

	translated, err := translateAcceleratorKey(key)
	if err != nil {
		return "", invalidAcceleratorError(binding, err.Error())
	}
	return strings.Join(append(modifiers, translated), "+"), nil
}

func translateAcceleratorKey(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("缺少主键")
	}
	if named, ok := wailsNamedKeys[strings.ToLower(key)]; ok {
		return named, nil
	}

	if key == "TabNumber" || key == "Arrows" {
		return "", fmt.Errorf("全局热键不支持通配主键 %q", key)
	}

	if len(key) >= 2 && (key[0] == 'F' || key[0] == 'f') {
		if number, err := parseFunctionKeyNumber(key[1:]); err == nil && number >= 1 && number <= 35 {
			return "f" + key[1:], nil
		}
	}
	runes := []rune(key)
	if len(runes) != 1 {
		return "", fmt.Errorf("不支持的主键 %q", key)
	}
	if runes[0] == '+' {
		return "plus", nil
	}
	return strings.ToLower(key), nil
}

func parseFunctionKeyNumber(digits string) (int, error) {
	if digits == "" {
		return 0, fmt.Errorf("缺少功能键序号")
	}
	number := 0
	for _, char := range digits {
		if char < '0' || char > '9' {
			return 0, fmt.Errorf("功能键序号不是数字")
		}
		number = number*10 + int(char-'0')
	}
	return number, nil
}

func invalidAcceleratorError(binding, reason string) error {
	return fmt.Errorf("invalid global shortcut %q: %s", binding, reason)
}
