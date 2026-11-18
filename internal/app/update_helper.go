package app

import (
	"errors"
	"os"
	"time"

	"changeme/internal/update"
)

func RunUpdateHelperIfRequested(args []string) (bool, error) {
	planPath, ok := update.ParseUpdatePlanArg(args)
	if !ok {
		return false, nil
	}

	entry := openHelperLog(update.UpdatePlan{})
	plan, err := update.ReadUpdatePlan(planPath)
	if entry == nil && err == nil {
		entry = openHelperLog(plan)
	}
	helperLogger = entry
	defer entry.close()

	entry.log("INFO", "helper.plan.parse", "plan", planPath)

	dropPlan := func(reason string) {
		update.RemoveUpdatePlan(planPath)
		entry.log("INFO", "helper.plan.removed", "plan", planPath, "reason", reason)
	}
	if err != nil {
		entry.log("ERROR", "helper.plan.read_failed", "plan", planPath, "error", err)
		dropPlan("read_failed")
		return true, err
	}

	nonce, ok := update.ParsePlanNonceArg(args)
	if !ok {
		entry.log("ERROR", "helper.plan.nonce_missing", "plan", planPath, "archive", plan.Archive, "target", plan.Target)
		dropPlan("nonce_missing")
		return true, errors.New("update helper is missing the plan nonce")
	}
	executable, err := os.Executable()
	if err != nil {
		entry.log("ERROR", "helper.plan.executable_failed", "error", err)
		dropPlan("executable_failed")
		return true, err
	}
	if err := update.VerifyUpdatePlan(plan, nonce, executable); err != nil {

		entry.log("ERROR", "helper.plan.verify_failed", "plan", planPath, "archive", plan.Archive,
			"target", plan.Target, "nonce", helperLogRedacted, "error", err)
		dropPlan("verify_failed")
		return true, err
	}
	started := time.Now()
	entry.log("INFO", "helper.apply.start", "plan", planPath, "archive", plan.Archive,
		"target", plan.Target, "pid", plan.Pid, "nonce", helperLogRedacted)
	failure := applyUpdatePlan(plan)
	elapsed := time.Since(started)
	if failure != nil {

		entry.log("ERROR", "helper.apply.done", "elapsed_ms", elapsed.Milliseconds(), "error", failure)
	} else {
		entry.log("INFO", "helper.apply.done", "elapsed_ms", elapsed.Milliseconds())
	}

	dropPlan("apply_done")
	return true, failure
}
