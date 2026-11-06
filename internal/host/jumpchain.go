package host

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"changeme/internal/errno"
	"changeme/internal/model"
	storage "changeme/internal/store"
)

const MaxJumpChainDepth = 5

func (s *HostService) validateJumpChain(ctx context.Context, selfID, jumpHostID string) error {
	jumpHostID = strings.TrimSpace(jumpHostID)
	if jumpHostID == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}
	return validateJumpChainWith(ctx, s.store, selfID, jumpHostID)
}

func validateJumpChainWith(ctx context.Context, store *storage.Store, selfID, jumpHostID string) error {
	if store == nil {
		return ErrHostServiceNotReady
	}
	if strings.TrimSpace(jumpHostID) == "" {
		return nil
	}
	if jumpHostID == selfID {

		return errno.New(errno.CodeInvalidArg, "host.jump_cycle", "")
	}

	visited := make(map[string]struct{}, MaxJumpChainDepth+1)
	visited[selfID] = struct{}{}
	current := jumpHostID
	for depth := 1; current != ""; depth++ {
		if depth > MaxJumpChainDepth {
			return errno.New(errno.CodeInvalidArg, "host.jump_chain_too_deep", strconv.Itoa(MaxJumpChainDepth))
		}
		if _, seen := visited[current]; seen {
			return errno.New(errno.CodeInvalidArg, "host.jump_cycle", "")
		}
		visited[current] = struct{}{}
		hop, err := store.GetHost(ctx, current)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {

				return errno.New(errno.CodeNotFound, "host.jump_not_found", "")
			}
			return errno.New(errno.CodeStoreFailed, "host.read_failed", "")
		}

		protocol := hop.Protocol
		if protocol == "" {

			protocol = model.ProtocolSSH
		}
		if protocol != model.ProtocolSSH {
			return errno.New(errno.CodeInvalidArg, "host.jump_protocol_invalid", "")
		}
		current = stringValue(hop.JumpHostID)
	}
	return nil
}

func (s *HostService) requireCredentialExists(ctx context.Context, credentialID string) error {
	credentialID = strings.TrimSpace(credentialID)
	if credentialID == "" {
		return nil
	}
	links, lookup := s.credentialLinkSnapshot()
	if links == nil || lookup == nil {
		return errno.New(errno.CodeSessionClosed, "host.credential_not_ready", "")
	}
	exists, err := lookup(ctx, credentialID)
	if err != nil {
		if errors.Is(err, model.ErrCredentialVaultLocked) {
			return errno.New(errno.CodePermissionDenied, "host.credential_locked", "")
		}
		return errno.New(errno.CodeStoreFailed, "host.credential_check_failed", "")
	}
	if !exists {
		return errno.New(errno.CodeNotFound, "credential.not_found", "")
	}
	return nil
}

func jumpHostInUseError(count int) error {
	detail := ""
	if count > 0 {
		detail = strconv.Itoa(count)
	}
	return errno.New(errno.CodeInvalidArg, "host.jump_in_use", detail)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
