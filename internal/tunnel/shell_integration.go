package tunnel

import (
	"context"
	"errors"
	"path"
	"strings"
	"time"
	"unicode"
)

const (
	shellProbeCommand     = `printf '%s\n' "$SHELL"`
	shellProbeTimeout     = 2 * time.Second
	shellProbeOutputLimit = 256
)

type interactiveShell string

const (
	interactiveShellBash interactiveShell = "bash"
	interactiveShellZsh  interactiveShell = "zsh"
	interactiveShellFish interactiveShell = "fish"
)

func (c *Connection) terminalStartupCommand(ctx context.Context, directoryFollow, shellIntegration bool) (string, error) {
	if ctx == nil {
		return "", errors.New("shell integration context is nil")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	probeCtx, cancel := context.WithTimeout(ctx, shellProbeTimeout)
	defer cancel()
	output, err := c.runProbe(probeCtx, shellProbeCommand, shellProbeOutputLimit)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", nil
	}
	shellPath, shell := parseInteractiveShell(output)
	if shell == "" {
		return "", nil
	}
	return buildStartupCommand(shellPath, shell, directoryFollow, shellIntegration), nil
}

func parseInteractiveShell(output string) (string, interactiveShell) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || hasControlCharacter(line) {
			continue
		}
		if len(line) > 256 || strings.ContainsAny(line, " \t\r") {
			continue
		}
		if !strings.HasPrefix(line, "/") {
			continue
		}
		base := path.Base(line)
		switch base {
		case string(interactiveShellBash):
			return line, interactiveShellBash
		case string(interactiveShellZsh):
			return line, interactiveShellZsh
		case string(interactiveShellFish):
			return line, interactiveShellFish
		}
	}
	return "", ""
}

func buildStartupCommand(shellPath string, shell interactiveShell, directoryFollow, shellIntegration bool) string {
	shellArg := shellQuote(shellPath)
	switch shell {
	case interactiveShellBash:
		return buildBashStartupCommand(shellArg, directoryFollow, shellIntegration)
	case interactiveShellZsh:
		return buildZshStartupCommand(shellArg, directoryFollow, shellIntegration)
	case interactiveShellFish:
		return buildFishStartupCommand(shellArg, directoryFollow, shellIntegration)
	default:
		return ""
	}
}

func buildBashStartupCommand(shellArg string, directoryFollow, shellIntegration bool) string {
	init := bashRcPrefix
	if directoryFollow {
		init += bashDirectoryFollowInit
	}
	if shellIntegration {
		init += bashShellIntegrationInit
	}
	return "tmp=$(mktemp \"${TMPDIR:-/tmp}/qterm-bash.XXXXXX\" 2>/dev/null) || { " + shellArg + " -i; exit $?; }; " +
		"trap 'rm -f \"$tmp\"' 0 HUP INT TERM; " +
		"if ! printf '%s' " + shellQuote(init) + " >\"$tmp\"; then " +
		shellArg + " -i; qterm_status=$?; exit \"$qterm_status\"; fi; " +
		shellArg + " --rcfile \"$tmp\" -i; qterm_status=$?; exit \"$qterm_status\""
}

func buildZshStartupCommand(shellArg string, directoryFollow, shellIntegration bool) string {
	zshrc := zshRcPrefix
	if directoryFollow {
		zshrc += zshDirectoryFollowHook
	}
	if shellIntegration {
		zshrc += zshShellIntegrationHook
	}
	return "tmpdir=$(mktemp -d \"${TMPDIR:-/tmp}/qterm-zsh.XXXXXX\" 2>/dev/null || mktemp -d 2>/dev/null) || { " + shellArg + " -i; exit $?; }; " +
		"original_zdotdir=${ZDOTDIR:-$HOME}; " +
		"export QTERM_SHELL_INTEGRATION_ORIGINAL_ZDOTDIR=\"$original_zdotdir\"; " +
		"export QTERM_SHELL_INTEGRATION_TEMP_ZDOTDIR=\"$tmpdir\"; " +
		"trap 'rm -f \"$tmpdir/.zshenv\"; rm -f \"$tmpdir/.zprofile\"; rm -f \"$tmpdir/.zshrc\"; rm -f \"$tmpdir/.zlogin\"; rm -f \"$tmpdir/.zsh_history\"; rmdir \"$tmpdir\"' 0 HUP INT TERM; " +
		"if ! printf '%s' " + shellQuote(zshDirectoryFollowZshenv) + " >\"$tmpdir/.zshenv\" || " +
		"! printf '%s' " + shellQuote(zshDirectoryFollowZprofile) + " >\"$tmpdir/.zprofile\" || " +
		"! printf '%s' " + shellQuote(zshrc) + " >\"$tmpdir/.zshrc\" || " +
		"! printf '%s' " + shellQuote(zshDirectoryFollowZlogin) + " >\"$tmpdir/.zlogin\"; then " +
		shellArg + " -i; qterm_status=$?; exit \"$qterm_status\"; fi; " +
		"ZDOTDIR=\"$tmpdir\" " + shellArg + " -i; qterm_status=$?; exit \"$qterm_status\""
}

const bashRcPrefix = `if [ -r "$HOME/.bashrc" ]; then
    source "$HOME/.bashrc"
fi
`

const bashDirectoryFollowInit = `if [[ "$(declare -p PROMPT_COMMAND 2>/dev/null)" == "declare -a"* ]]; then
    PROMPT_COMMAND+=( '(_qterm_status=$?; case "$PWD" in *[[:cntrl:]]*) ;; *) printf "\033]1337;CurrentDir=%s\007" "$PWD";; esac; exit "$_qterm_status")' )
else
    case "$PROMPT_COMMAND" in
        '') ;;
        *';'|*'; ') ;;
        *) PROMPT_COMMAND="$PROMPT_COMMAND; ";;
    esac
    PROMPT_COMMAND="${PROMPT_COMMAND}(_qterm_status=\$?; case \"\$PWD\" in *[[:cntrl:]]*) ;; *) printf \"\\033]1337;CurrentDir=%s\\007\" \"\$PWD\";; esac; exit \"\$_qterm_status\")"
fi
`

const bashShellIntegrationInit = `__qterm_osc133_preexec() {
    case "$BASH_COMMAND" in
        __qterm_osc133_*) return ;;
    esac
    printf '\033]133;C\007'
    if [[ -n "${__qterm_osc133_previous_debug-}" ]]; then
        eval "$__qterm_osc133_previous_debug"
    fi
}
__qterm_osc133_precmd() {
    local __qterm_status=$?
    printf '\033]133;D;%s\007' "$__qterm_status"
    printf '\033]133;A\007'
    return "$__qterm_status"
}
case "${PS1-}" in
    *']133;B'*) ;;
    *) PS1="$PS1\[\033]133;B\007\]" ;;
esac
__qterm_osc133_previous_debug="$(trap -p DEBUG)"
if [[ -n "$__qterm_osc133_previous_debug" ]]; then
    __qterm_osc133_previous_debug="${__qterm_osc133_previous_debug#trap -- }"
    __qterm_osc133_previous_debug="${__qterm_osc133_previous_debug% DEBUG}"
fi
trap '__qterm_osc133_preexec' DEBUG
if [[ "$(declare -p PROMPT_COMMAND 2>/dev/null)" == "declare -a"* ]]; then
    PROMPT_COMMAND=( '__qterm_osc133_precmd' "${PROMPT_COMMAND[@]}" )
else
    case "$PROMPT_COMMAND" in
        '') PROMPT_COMMAND='__qterm_osc133_precmd' ;;
        *';'|*'; ') PROMPT_COMMAND="__qterm_osc133_precmd;${PROMPT_COMMAND}" ;;
        *) PROMPT_COMMAND="__qterm_osc133_precmd;${PROMPT_COMMAND}" ;;
    esac
fi
`

const zshDirectoryFollowZshenv = `if [[ -r "$QTERM_SHELL_INTEGRATION_ORIGINAL_ZDOTDIR/.zshenv" ]]; then
    ZDOTDIR="$QTERM_SHELL_INTEGRATION_ORIGINAL_ZDOTDIR"
    source "$QTERM_SHELL_INTEGRATION_ORIGINAL_ZDOTDIR/.zshenv"
fi
ZDOTDIR="$QTERM_SHELL_INTEGRATION_TEMP_ZDOTDIR"
`

const zshDirectoryFollowZprofile = `if [[ -r "$QTERM_SHELL_INTEGRATION_ORIGINAL_ZDOTDIR/.zprofile" ]]; then
    ZDOTDIR="$QTERM_SHELL_INTEGRATION_ORIGINAL_ZDOTDIR"
    source "$QTERM_SHELL_INTEGRATION_ORIGINAL_ZDOTDIR/.zprofile"
fi
ZDOTDIR="$QTERM_SHELL_INTEGRATION_TEMP_ZDOTDIR"
`

const zshRcPrefix = `if [[ -r "$QTERM_SHELL_INTEGRATION_ORIGINAL_ZDOTDIR/.zshrc" ]]; then
    ZDOTDIR="$QTERM_SHELL_INTEGRATION_ORIGINAL_ZDOTDIR"
    source "$QTERM_SHELL_INTEGRATION_ORIGINAL_ZDOTDIR/.zshrc"
fi
ZDOTDIR="$QTERM_SHELL_INTEGRATION_ORIGINAL_ZDOTDIR"
`

const zshDirectoryFollowHook = `__qterm_current_dir_precmd() {
    local qterm_status=$?
    case "$PWD" in
        *[[:cntrl:]]*) return "$qterm_status" ;;
        *) printf '\033]1337;CurrentDir=%s\007' "$PWD" ;;
    esac
    return "$qterm_status"
}
autoload -Uz add-zsh-hook 2>/dev/null && add-zsh-hook precmd __qterm_current_dir_precmd
`

const zshShellIntegrationHook = `__qterm_osc133_precmd() {
    local qterm_status=$?
    printf '\033]133;D;%s\007' "$qterm_status"
    printf '\033]133;A\007'
    return "$qterm_status"
}
__qterm_osc133_preexec() {
    printf '\033]133;C\007'
}
case "${PROMPT-}" in
    *']133;B'*) ;;
    *) PROMPT="${PROMPT}%{"$'\033'"]133;B"$'\007'"%}" ;;
esac
autoload -Uz add-zsh-hook 2>/dev/null && add-zsh-hook precmd __qterm_osc133_precmd
autoload -Uz add-zsh-hook 2>/dev/null && add-zsh-hook preexec __qterm_osc133_preexec
`

const zshDirectoryFollowZlogin = `if [[ -r "$QTERM_SHELL_INTEGRATION_ORIGINAL_ZDOTDIR/.zlogin" ]]; then
    ZDOTDIR="$QTERM_SHELL_INTEGRATION_ORIGINAL_ZDOTDIR"
    source "$QTERM_SHELL_INTEGRATION_ORIGINAL_ZDOTDIR/.zlogin"
fi
ZDOTDIR="$QTERM_SHELL_INTEGRATION_ORIGINAL_ZDOTDIR"
`

const fishDirectoryFollowInit = `function __qterm_current_dir_prompt --on-event fish_prompt
    if string match --regex -q '[[:cntrl:]]' -- "$PWD"
        return
    end
    printf '\033]1337;CurrentDir=%s\007' "$PWD"
end
`

const fishShellIntegrationInit = `function __qterm_osc133_prompt --on-event fish_prompt
    printf '\033]133;A\007'
    printf '\033]133;B\007'
end
function __qterm_osc133_preexec --on-event fish_preexec
    printf '\033]133;C\007'
end
function __qterm_osc133_postexec --on-event fish_postexec
    printf '\033]133;D;%s\007' "$status"
end
`

func buildFishStartupCommand(shellArg string, directoryFollow, shellIntegration bool) string {
	init := ""
	if directoryFollow {
		init += fishDirectoryFollowInit
	}
	if shellIntegration {
		init += fishShellIntegrationInit
	}
	if init == "" {
		return shellArg + " --interactive"
	}
	return shellArg + " --interactive --init-command " + shellQuote(init)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func hasControlCharacter(value string) bool {
	for _, char := range value {
		if char == unicode.ReplacementChar || char < 0x20 || char == 0x7f {
			return true
		}
	}
	return false
}
