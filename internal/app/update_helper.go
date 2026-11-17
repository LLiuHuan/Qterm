package app

import (
	"errors"
	"os"

	"changeme/internal/update"
)

func RunUpdateHelperIfRequested(args []string) (bool, error) {
	planPath, ok := update.ParseUpdatePlanArg(args)
	if !ok {
		return false, nil
	}

	plan, err := update.ReadUpdatePlan(planPath)
	if err != nil {
		update.RemoveUpdatePlan(planPath)
		return true, err
	}
	nonce, ok := update.ParsePlanNonceArg(args)
	if !ok {
		update.RemoveUpdatePlan(planPath)
		return true, errors.New("update helper is missing the plan nonce")
	}
	executable, err := os.Executable()
	if err != nil {
		update.RemoveUpdatePlan(planPath)
		return true, err
	}
	if err := update.VerifyUpdatePlan(plan, nonce, executable); err != nil {
		update.RemoveUpdatePlan(planPath)
		return true, err
	}
	failure := applyUpdatePlan(plan)

	update.RemoveUpdatePlan(planPath)
	return true, failure
}
