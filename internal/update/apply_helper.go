package update

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type UpdatePlan struct {
	Archive string `json:"archive"`

	ArchiveSHA256 string `json:"archiveSha256"`

	Target string `json:"target"`

	Pid int `json:"pid"`

	Nonce string `json:"nonce"`
}

const UpdateHelperFlagPrefix = "--qterm-apply-update="

const planNoncePrefix = "--qterm-apply-update-nonce="

func ParseUpdatePlanArg(args []string) (string, bool) {
	for _, arg := range args {
		if !strings.HasPrefix(arg, UpdateHelperFlagPrefix) {
			continue
		}
		path := strings.TrimPrefix(arg, UpdateHelperFlagPrefix)
		if strings.TrimSpace(path) == "" {
			return "", false
		}
		return path, true
	}
	return "", false
}

func ParsePlanNonceArg(args []string) (string, bool) {
	for _, arg := range args {
		if !strings.HasPrefix(arg, planNoncePrefix) {
			continue
		}
		nonce := strings.TrimPrefix(arg, planNoncePrefix)
		if strings.TrimSpace(nonce) == "" {
			return "", false
		}
		return nonce, true
	}
	return "", false
}

func WriteUpdatePlan(path string, plan UpdatePlan) error {
	body, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o600)
}

func ReadUpdatePlan(path string) (UpdatePlan, error) {
	var plan UpdatePlan
	body, err := os.ReadFile(path)
	if err != nil {
		return plan, err
	}
	if err := json.Unmarshal(body, &plan); err != nil {
		return plan, err
	}
	if strings.TrimSpace(plan.Archive) == "" || strings.TrimSpace(plan.Target) == "" {
		return plan, errors.New("update plan is incomplete")
	}
	if !isSHA256Hex(plan.ArchiveSHA256) || strings.TrimSpace(plan.Nonce) == "" {
		return plan, errors.New("update plan is not signed")
	}
	return plan, nil
}

func VerifyUpdatePlan(plan UpdatePlan, expectedNonce, executable string) error {
	if strings.TrimSpace(expectedNonce) == "" {
		return errors.New("update plan nonce is missing")
	}
	if plan.Nonce != expectedNonce {
		return errors.New("update plan nonce mismatch")
	}
	if !samePath(plan.Target, executable) {
		return errors.New("update plan target mismatch")
	}
	if !isSHA256Hex(plan.ArchiveSHA256) {
		return errors.New("update plan archive hash is invalid")
	}
	actual, err := fileSHA256(plan.Archive)
	if err != nil {
		return err
	}
	if !strings.EqualFold(actual, plan.ArchiveSHA256) {
		return errors.New("update archive hash mismatch")
	}
	return nil
}

func samePath(left, right string) bool {
	if strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
		return false
	}
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func fileSHA256(path string) (string, error) {
	handle, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer handle.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, handle); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func isSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	return isHex(value)
}

func RemoveUpdatePlan(path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	_ = os.Remove(path)
}
