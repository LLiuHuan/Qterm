package tunnel

import (
	"context"
	"errors"
	"strings"
)

const directoryFollowBlock = `case "$PROMPT_COMMAND" in
    '') ;;
    *';'|*'; ') ;;
    *) PROMPT_COMMAND="$PROMPT_COMMAND; ";;
esac
PROMPT_COMMAND="${PROMPT_COMMAND}(_qterm_status=\$?; printf \"\\033]1337;CurrentDir=%s\\007\" \"\$PWD\"; exit \"\$_qterm_status\")"
`

func (c *Connection) DirectoryFollow(ctx context.Context, configure bool) (string, error) {
	output, err := c.runProbe(ctx, directoryFollowCommand(configure), 4096)
	if err != nil {
		return "", err
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	switch strings.TrimSpace(lines[len(lines)-1]) {
	case "QTERM_FOLLOW:present":
		return "present", nil
	case "QTERM_FOLLOW:missing":
		return "missing", nil
	case "QTERM_FOLLOW:unsupported":
		return "unsupported", nil
	default:
		return "", errors.New("directory follow probe failed")
	}
}

func directoryFollowCommand(configure bool) string {
	action := "printf '\\nQTERM_FOLLOW:missing\\n'"
	if configure {
		block := "'" + strings.ReplaceAll(directoryFollowBlock, "'", "'\\''") + "'"
		action = "if (umask 077; printf '\\n%s' " + block + ") >> \"$HOME/.bashrc\"; then printf '\\nQTERM_FOLLOW:present\\n'; else exit 1; fi"
	}

	return `if [ -z "${BASH_VERSION-}" ]; then printf '\nQTERM_FOLLOW:unsupported\n';
elif [ -e "$HOME/.bashrc" ] && { [ ! -f "$HOME/.bashrc" ] || [ ! -r "$HOME/.bashrc" ]; }; then exit 1;
elif grep -Eq '1337;CurrentDir' "$HOME/.bashrc" 2>/dev/null; then printf '\nQTERM_FOLLOW:present\n';
else ` + action + `; fi`
}
