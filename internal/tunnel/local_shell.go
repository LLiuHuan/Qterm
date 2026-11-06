package tunnel

import (
	"os"
	"os/exec"
	"strings"
)

type localShellSpec struct {
	Path string
	Args []string
}

const (
	LocalShellPathFallbackWarning = "session.local_shell_path_fallback"

	LocalShellDirectoryFallbackWarning = "session.local_shell_directory_fallback"
)

type LocalShellOptions struct {
	Path string

	WorkingDirectory string
}

type LocalShellPlan struct {
	Path string

	Args []string

	WorkingDirectory string

	Warnings []string
}

func ResolveLocalShellPlan(options LocalShellOptions) (LocalShellPlan, error) {
	plan := LocalShellPlan{Warnings: []string{}}
	if requested := strings.TrimSpace(options.Path); requested != "" {

		if resolved, err := exec.LookPath(requested); err == nil {
			plan.Path = resolved
			plan.Args = localShellArgs(resolved)
		} else {

			plan.Warnings = append(plan.Warnings, LocalShellPathFallbackWarning)
		}
	}
	if plan.Path == "" {
		spec, ok := defaultLocalShell()
		if !ok {
			return LocalShellPlan{}, ErrLocalShellUnavailable
		}
		plan.Path = spec.Path
		plan.Args = spec.Args
	}

	workingDirectory := strings.TrimSpace(options.WorkingDirectory)
	if workingDirectory == "" {
		plan.WorkingDirectory = localShellWorkDir()
		return plan, nil
	}

	if info, err := os.Stat(workingDirectory); err != nil || !info.IsDir() {
		plan.Warnings = append(plan.Warnings, LocalShellDirectoryFallbackWarning)
		plan.WorkingDirectory = localShellWorkDir()
		return plan, nil
	}
	plan.WorkingDirectory = workingDirectory
	return plan, nil
}

func resolveLocalShell(candidates []string) (localShellSpec, bool) {
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		resolved, err := exec.LookPath(candidate)
		if err != nil {
			continue
		}
		return localShellSpec{Path: resolved, Args: localShellArgs(resolved)}, true
	}
	return localShellSpec{}, false
}

func defaultLocalShell() (localShellSpec, bool) {
	return resolveLocalShell(defaultLocalShellCandidates())
}

func localShellEnv() []string {
	return mergeEnvironment(os.Environ(), map[string]string{
		"TERM":      "xterm-256color",
		"COLORTERM": "truecolor",
	})
}

func localShellWorkDir() string {
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return home
	}
	return ""
}

func mergeEnvironment(env []string, overrides map[string]string) []string {
	merged := make([]string, 0, len(env)+len(overrides))
	applied := make(map[string]struct{}, len(overrides))
	for _, entry := range env {
		key, _, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		value, override := overrides[key]
		if !override {
			merged = append(merged, entry)
			continue
		}
		if _, done := applied[key]; done {

			continue
		}
		applied[key] = struct{}{}
		merged = append(merged, key+"="+value)
	}
	for key, value := range overrides {
		if _, done := applied[key]; done {
			continue
		}
		merged = append(merged, key+"="+value)
	}
	return merged
}
