package app

import (
	"changeme/internal/ai"
	"changeme/internal/clitool"
	"changeme/internal/errno"
)

func (s *Service) cliCommandGate(command string) error {
	settings, err := s.cliToolService().Settings()
	if err != nil {
		return errno.New(errno.CodePermissionDenied, "settings.cli.execGuardUnavailable", "")
	}
	if settings.ExecGuard == clitool.ExecGuardOff {
		return nil
	}

	if err := s.ensureAICommandRules(); err != nil {
		return errno.New(errno.CodePermissionDenied, "settings.cli.execGuardUnavailable", "")
	}
	s.commandRules.mu.RLock()
	compiled := s.commandRules.compiled
	s.commandRules.mu.RUnlock()
	if match := ai.MatchCommandRules(command, compiled); match.Matched {

		return errno.New(errno.CodePermissionDenied, "settings.cli.execDenied", ai.DescribeCommandRuleDenial(match))
	}
	if settings.ExecGuard != clitool.ExecGuardStrict {
		return nil
	}

	risk, reason := ai.AssessCommandRisk(command)
	if risk != ai.RiskReadOnly {
		return errno.New(errno.CodePermissionDenied, "settings.cli.execNeedsApproval", reason)
	}
	return nil
}
