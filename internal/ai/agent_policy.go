package ai

import (
	"regexp"
	"strings"
)

type ApprovalMode string

const (
	ApprovalStrict    ApprovalMode = "strict"
	ApprovalBalanced  ApprovalMode = "balanced"
	ApprovalReview    ApprovalMode = "review"
	ApprovalAutomatic ApprovalMode = "automatic"
)

func ParseApprovalMode(value string) ApprovalMode {
	switch ApprovalMode(strings.TrimSpace(value)) {
	case ApprovalStrict:
		return ApprovalStrict
	case ApprovalBalanced:
		return ApprovalBalanced
	case ApprovalReview:
		return ApprovalReview
	case ApprovalAutomatic:
		return ApprovalAutomatic
	default:
		return ApprovalStrict
	}
}

var destructivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\brm\s+(-[a-z]*[rf][a-z]*\s+)+(/|/\*|~|\$HOME)(\s|$)`),
	regexp.MustCompile(`(?i)\bmkfs(\.[a-z0-9]+)?\b`),
	regexp.MustCompile(`(?i)\bdd\b[^\n]*\bof=/dev/`),
	regexp.MustCompile(`(?i)\b(shutdown|reboot|poweroff|halt)\b`),
	regexp.MustCompile(`(?i)\binit\s+0\b`),
	regexp.MustCompile(`(?i)\bwipefs\b|\bfdisk\b|\bparted\b|\bsgdisk\b`),
	regexp.MustCompile(`(?i)\bzpool\s+destroy\b|\bmdadm\b[^\n]*--zero-superblock`),
	regexp.MustCompile(`(?i)\bchmod\s+-R\s+777\s+/(\s|$)`),
	regexp.MustCompile(`(?i)\bchown\s+-R\s+[^\s]+\s+/(\s|$)`),
	regexp.MustCompile(`(?i)>\s*/dev/(sd|nvme|disk)`),
	regexp.MustCompile(`:\(\)\s*\{\s*:\|:&\s*\};:`),
	regexp.MustCompile(`(?i)\b(drop\s+database|truncate\s+table)\b`),
}

var readOnlyPrefixes = map[string]bool{
	"ls": true, "pwd": true, "cat": true, "head": true, "tail": true,
	"df": true, "du": true, "free": true, "ps": true, "uptime": true,
	"whoami": true, "id": true, "uname": true, "hostname": true, "date": true,
	"wc": true, "stat": true, "file": true, "which": true, "type": true,
	"echo": true, "printf": true, "env": true, "printenv": true,
	"nproc": true, "lscpu": true, "lsblk": true, "lsof": true,
	"systemctl status": true, "journalctl": true,
	"git status": true, "git log": true, "git diff": true, "git show": true,
	"docker ps": true, "docker images": true, "docker version": true,
	"ip addr": true, "ip route": true, "ss": true, "netstat": true,
	"ping": true, "dig": true, "nslookup": true,
}

func AssessCommandRisk(command string) (string, string) {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return RiskConfirm, ""
	}
	for _, pattern := range destructivePatterns {
		if pattern.MatchString(trimmed) {
			return RiskDanger, "matches a destructive command pattern"
		}
	}
	if isConservativeReadOnly(trimmed) {
		return RiskReadOnly, ""
	}
	return RiskConfirm, ""
}

var readOnlyPipeFilters = map[string]bool{
	"head": true, "tail": true, "grep": true, "egrep": true, "fgrep": true,
	"wc": true, "sort": true, "uniq": true, "cut": true, "tr": true,
	"awk": true, "column": true, "nl": true,
}

func isConservativeReadOnly(command string) bool {
	if strings.ContainsAny(command, "\n\r") {
		return false
	}

	sanitized := strings.ReplaceAll(command, "2>/dev/null", "")
	sanitized = strings.ReplaceAll(sanitized, "2> /dev/null", "")
	sanitized = strings.ReplaceAll(sanitized, ">/dev/null", "")
	sanitized = strings.ReplaceAll(sanitized, "> /dev/null", "")
	for _, token := range []string{">", "<", ">>", ";", "&&", "||", "`", "$(", "&>"} {
		if strings.Contains(sanitized, token) {
			return false
		}
	}
	if strings.Contains(sanitized, "|") {
		segments := strings.Split(sanitized, "|")
		if len(segments) < 2 {
			return false
		}

		first := strings.Fields(strings.ToLower(strings.TrimSpace(segments[0])))
		if len(first) == 0 || !readOnlyPrefixes[first[0]] {
			return false
		}

		for _, segment := range segments[1:] {
			fields := strings.Fields(strings.TrimSpace(segment))
			if len(fields) == 0 || !readOnlyPipeFilters[fields[0]] {
				return false
			}
		}
		return true
	}
	lowered := strings.ToLower(command)
	fields := strings.Fields(lowered)
	if len(fields) == 0 {
		return false
	}

	if len(fields) >= 2 {
		if readOnlyPrefixes[fields[0]+" "+fields[1]] {
			return true
		}
	}
	return readOnlyPrefixes[fields[0]]
}

func maxRisk(local, model string) string {
	rank := func(risk string) int {
		switch normalizeRisk(risk) {
		case RiskReadOnly:
			return 0
		case RiskDanger:
			return 2
		default:
			return 1
		}
	}
	if rank(model) > rank(local) {
		return normalizeRisk(model)
	}
	return normalizeRisk(local)
}

func RequiresApproval(mode ApprovalMode, command, modelRisk, modelReason string) (bool, string, string) {
	localRisk, localReason := AssessCommandRisk(command)
	risk := maxRisk(localRisk, modelRisk)
	reason := strings.TrimSpace(modelReason)
	if risk == RiskDanger && localReason != "" {
		reason = localReason
	}
	if risk == RiskDanger {

		return true, risk, reason
	}
	switch mode {
	case ApprovalAutomatic:
		return false, risk, reason
	case ApprovalBalanced, ApprovalReview:

		if localRisk == RiskReadOnly && risk != RiskDanger {
			return false, risk, reason
		}
		return risk != RiskReadOnly, risk, reason
	default:
		return true, risk, reason
	}
}
