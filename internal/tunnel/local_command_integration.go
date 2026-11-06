package tunnel

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type localCommandIntegration struct {
	enabled bool
	args    []string
	env     map[string]string

	cleanupFiles []string
	cleanupDirs  []string
}

func prepareLocalCommandIntegration(plan LocalShellPlan) localCommandIntegration {
	if runtime.GOOS == "windows" {

		return localCommandIntegration{}
	}

	shell := interactiveShell(filepath.Base(plan.Path))
	switch shell {
	case interactiveShellBash:
		return prepareBashLocalCommandIntegration(plan)
	case interactiveShellZsh:
		return prepareZshLocalCommandIntegration(plan)
	case interactiveShellFish:
		return localCommandIntegration{
			enabled: true,
			args:    buildFishLocalCommandArgs(plan.Args),
			env:     map[string]string{},
		}
	default:
		return localCommandIntegration{}
	}
}

func (i localCommandIntegration) commandArgs(original []string) []string {
	if !i.enabled {
		return append([]string(nil), original...)
	}
	return append([]string(nil), i.args...)
}

func (i localCommandIntegration) applyEnvironment(environment []string) []string {
	if !i.enabled || len(i.env) == 0 {
		return environment
	}
	return mergeEnvironment(environment, i.env)
}

func (i *localCommandIntegration) cleanup() error {
	if i == nil {
		return nil
	}
	var firstErr error
	for _, path := range i.cleanupFiles {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) && firstErr == nil {
			firstErr = err
		}
	}
	for _, path := range i.cleanupDirs {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) && firstErr == nil {
			firstErr = err
		}
	}
	i.cleanupFiles = nil
	i.cleanupDirs = nil
	return firstErr
}

func prepareBashLocalCommandIntegration(plan LocalShellPlan) localCommandIntegration {
	loginShell := hasLocalShellLoginArg(plan.Args)
	path, err := createLocalIntegrationFile("qterm-bash-", buildBashLocalCommandScript(loginShell))
	if err != nil {

		return localCommandIntegration{}
	}

	args := removeLocalShellLoginArgs(plan.Args)
	args = append(args, "--rcfile", path, "-i")
	return localCommandIntegration{
		enabled:      true,
		args:         args,
		env:          map[string]string{},
		cleanupFiles: []string{path},
	}
}

func prepareZshLocalCommandIntegration(plan LocalShellPlan) localCommandIntegration {
	directory, err := os.MkdirTemp("", "qterm-zsh-")
	if err != nil {
		return localCommandIntegration{}
	}

	files := []struct {
		name    string
		content string
	}{
		{name: ".zshenv", content: zshLocalCommandZshenv},
		{name: ".zprofile", content: zshLocalCommandZprofile},
		{name: ".zshrc", content: zshLocalCommandZshrc},
		{name: ".zlogin", content: zshLocalCommandZlogin},
	}
	created := make([]string, 0, len(files))
	for _, file := range files {
		path := filepath.Join(directory, file.name)

		created = append(created, path)
		if err := writeLocalIntegrationFile(path, file.content); err != nil {
			partial := localCommandIntegration{cleanupFiles: created, cleanupDirs: []string{directory}}
			_ = partial.cleanup()
			return localCommandIntegration{}
		}
	}

	originalZdotdir := os.Getenv("ZDOTDIR")
	if strings.TrimSpace(originalZdotdir) == "" {
		originalZdotdir = os.Getenv("HOME")
	}
	environment := map[string]string{
		"QTERM_LOCAL_ORIGINAL_ZDOTDIR": originalZdotdir,
		"QTERM_LOCAL_TEMP_ZDOTDIR":     directory,
		"ZDOTDIR":                      directory,
	}
	if strings.TrimSpace(os.Getenv("HISTFILE")) == "" && strings.TrimSpace(originalZdotdir) != "" {

		environment["QTERM_LOCAL_DEFAULT_HISTFILE"] = filepath.Join(originalZdotdir, ".zsh_history")
	}
	return localCommandIntegration{
		enabled:      true,
		args:         append([]string(nil), plan.Args...),
		env:          environment,
		cleanupFiles: created,
		cleanupDirs:  []string{directory},
	}
}

func buildFishLocalCommandArgs(original []string) []string {
	args := append([]string(nil), original...)
	if !containsLocalShellArg(args, "--interactive") && !containsLocalShellArg(args, "-i") {
		args = append(args, "--interactive")
	}
	return append(args, "--init-command", fishLocalCommandInit)
}

func buildBashLocalCommandScript(loginShell bool) string {
	startup := bashLocalInteractiveStartup
	if loginShell {
		startup = bashLocalLoginStartup
	}
	return startup + bashLocalCommandHooks
}

func createLocalIntegrationFile(prefix string, content string) (string, error) {
	file, err := os.CreateTemp("", prefix)
	if err != nil {
		return "", err
	}
	path := file.Name()
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func writeLocalIntegrationFile(path string, content string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func hasLocalShellLoginArg(args []string) bool {
	return containsLocalShellArg(args, "-l") || containsLocalShellArg(args, "--login")
}

func containsLocalShellArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func removeLocalShellLoginArgs(args []string) []string {
	result := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "-l" || arg == "--login" {
			continue
		}
		result = append(result, arg)
	}
	return result
}

const bashLocalLoginStartup = `# qterm local OSC 133 integration: preserve normal login startup files.
if [[ -r /etc/profile ]]; then
    source /etc/profile
fi
if [[ -r "$HOME/.bash_profile" ]]; then
    source "$HOME/.bash_profile"
elif [[ -r "$HOME/.bash_login" ]]; then
    source "$HOME/.bash_login"
elif [[ -r "$HOME/.profile" ]]; then
    source "$HOME/.profile"
fi
`

const bashLocalInteractiveStartup = `# qterm local OSC 133 integration: preserve normal interactive startup.
if [[ -r "$HOME/.bashrc" ]]; then
    source "$HOME/.bashrc"
fi
`

const bashLocalCommandHooks = `
__qterm_local_osc133_prompt_command() {
    local qterm_status=$?
    __qterm_local_osc133_in_prompt=1
    __qterm_local_osc133_in_command=0
    printf '\033]133;D;%s\007' "$qterm_status"
    printf '\033]133;A\007'
    return "$qterm_status"
}

__qterm_local_osc133_wrap_prompt() {
    if [[ "$PS1" != "$__qterm_local_osc133_last_ps1" ]]; then
        # B is the prompt end. The nonprinting brackets keep the OSC out of
        # Bash's prompt width calculation while preserving the user's PS1.
        PS1="${PS1}"'\[\e]133;B\a\]'
        __qterm_local_osc133_last_ps1="$PS1"
    fi
    __qterm_local_osc133_in_prompt=0
}

__qterm_local_osc133_preexec() {
    if [[ "$__qterm_local_osc133_in_prompt" == "1" ]]; then
        return
    fi
    if [[ "$__qterm_local_osc133_in_command" == "0" ]]; then
        __qterm_local_osc133_in_command=1
        # DEBUG is available on the Bash versions shipped by macOS and Linux;
        # it supplies C where newer Bash versions do not expose PS0.
        printf '\033]133;C\007'
    fi
}

__qterm_local_osc133_restore_status() {
    return "$1"
}

__qterm_local_osc133_get_debug_trap() {
    local -a qterm_terms
    eval "qterm_terms=( $(trap -p DEBUG) )"
    printf '%s' "${qterm_terms[2]-}"
}

__qterm_local_osc133_debug() {
    local qterm_status=$?
    local qterm_frame qterm_index
    case "$BASH_COMMAND" in
        __qterm_local_osc133_*) return ;;
    esac
    for ((qterm_index = 1; qterm_index < ${#FUNCNAME[@]}; qterm_index++)); do
        qterm_frame="${FUNCNAME[$qterm_index]}"
        case "$qterm_frame" in
            __qterm_local_osc133_*) return ;;
        esac
    done
    __qterm_local_osc133_preexec
    if [[ -n "$__qterm_local_osc133_previous_debug_trap" ]]; then
        # C 的 printf 会把 $? 改成 0；恢复原值后再执行旧 trap，保持它的观察语义。
        __qterm_local_osc133_restore_status "$qterm_status"
        eval "$__qterm_local_osc133_previous_debug_trap"
    fi
}

# Put our prompt hook first so it observes the command's real exit status;
# returning that status lets existing user hooks observe the same value. The
# final wrapper runs after user hooks so a dynamically changed PS1 is covered.
if [[ "$(declare -p PROMPT_COMMAND 2>/dev/null)" == "declare -a"* ]]; then
    PROMPT_COMMAND=(__qterm_local_osc133_prompt_command "${PROMPT_COMMAND[@]}" __qterm_local_osc133_wrap_prompt)
else
    if [[ -n "${PROMPT_COMMAND:-}" ]]; then
        PROMPT_COMMAND="__qterm_local_osc133_prompt_command;${PROMPT_COMMAND};__qterm_local_osc133_wrap_prompt"
    else
        PROMPT_COMMAND="__qterm_local_osc133_prompt_command;__qterm_local_osc133_wrap_prompt"
    fi
fi

__qterm_local_osc133_in_prompt=0
__qterm_local_osc133_in_command=0
__qterm_local_osc133_last_ps1=''
__qterm_local_osc133_previous_debug_trap="$(__qterm_local_osc133_get_debug_trap)"
trap '__qterm_local_osc133_debug "$_"' DEBUG
`

const zshLocalCommandZshenv = `if [[ -z "${QTERM_LOCAL_ORIGINAL_ZDOTDIR:-}" ]]; then
    QTERM_LOCAL_ORIGINAL_ZDOTDIR="${HOME:-}"
fi
if [[ -r "$QTERM_LOCAL_ORIGINAL_ZDOTDIR/.zshenv" ]]; then
    ZDOTDIR="$QTERM_LOCAL_ORIGINAL_ZDOTDIR"
    source "$QTERM_LOCAL_ORIGINAL_ZDOTDIR/.zshenv"
fi
ZDOTDIR="$QTERM_LOCAL_TEMP_ZDOTDIR"
`

const zshLocalCommandZprofile = `if [[ -r "$QTERM_LOCAL_ORIGINAL_ZDOTDIR/.zprofile" ]]; then
    ZDOTDIR="$QTERM_LOCAL_ORIGINAL_ZDOTDIR"
    source "$QTERM_LOCAL_ORIGINAL_ZDOTDIR/.zprofile"
fi
ZDOTDIR="$QTERM_LOCAL_TEMP_ZDOTDIR"
`

const zshLocalCommandZshrc = `if [[ "${HISTFILE:-}" == "$QTERM_LOCAL_TEMP_ZDOTDIR/.zsh_history" && -n "${QTERM_LOCAL_DEFAULT_HISTFILE:-}" ]]; then
    HISTFILE="$QTERM_LOCAL_DEFAULT_HISTFILE"
fi
unset QTERM_LOCAL_DEFAULT_HISTFILE
if [[ -r "$QTERM_LOCAL_ORIGINAL_ZDOTDIR/.zshrc" ]]; then
    ZDOTDIR="$QTERM_LOCAL_ORIGINAL_ZDOTDIR"
    source "$QTERM_LOCAL_ORIGINAL_ZDOTDIR/.zshrc"
fi
ZDOTDIR="$QTERM_LOCAL_ORIGINAL_ZDOTDIR"
` + zshLocalCommandHooks

const zshLocalCommandZlogin = `if [[ -r "$QTERM_LOCAL_ORIGINAL_ZDOTDIR/.zlogin" ]]; then
    ZDOTDIR="$QTERM_LOCAL_ORIGINAL_ZDOTDIR"
    source "$QTERM_LOCAL_ORIGINAL_ZDOTDIR/.zlogin"
fi
ZDOTDIR="$QTERM_LOCAL_ORIGINAL_ZDOTDIR"
` + zshLocalCommandHooks

const zshLocalCommandHooks = `
__qterm_local_osc133_wrap_prompt() {
    if [[ "$PS1" != "$__qterm_local_osc133_last_ps1" ]]; then
        PS1=$'%{\033]133;A\007%}'"${PS1}"$'%{\033]133;B\007%}'
        __qterm_local_osc133_last_ps1="$PS1"
    fi
}

__qterm_local_osc133_precmd_start() {
    local qterm_status=$?
    __qterm_local_osc133_last_status=$qterm_status
    printf '\033]133;D;%s\007' "$qterm_status"
}

__qterm_local_osc133_preexec() {
    # A/B are wrapped around the rendered prompt; C starts command output.
    printf '\033]133;C\007'
}

__qterm_local_osc133_precmd_end() {
    __qterm_local_osc133_wrap_prompt
    return "$__qterm_local_osc133_last_status"
}

__qterm_local_osc133_last_ps1=''
__qterm_local_osc133_last_status=0
typeset -ga precmd_functions
typeset -ga preexec_functions
precmd_functions=(${precmd_functions:#__qterm_local_osc133_precmd_start})
precmd_functions=(${precmd_functions:#__qterm_local_osc133_precmd_end})
precmd_functions=(__qterm_local_osc133_precmd_start $precmd_functions __qterm_local_osc133_precmd_end)
preexec_functions=(__qterm_local_osc133_preexec ${preexec_functions:#__qterm_local_osc133_preexec})
`

const fishLocalCommandInit = `function __qterm_local_osc133_wrap_prompt --on-event fish_prompt
    if functions --query fish_prompt
        functions --copy fish_prompt __qterm_local_osc133_user_prompt
        functions --erase fish_prompt
        functions --erase __qterm_local_osc133_wrap_prompt
        function fish_prompt
            printf '\033]133;A\007'
            __qterm_local_osc133_user_prompt
            printf '\033]133;B\007'
        end
    end
end
function __qterm_local_osc133_preexec --on-event fish_preexec
    printf '\033]133;C\007'
end
function __qterm_local_osc133_postexec --on-event fish_postexec
    printf '\033]133;D;%s\007' "$status"
end
`
