package sftp

import (
	"context"
	"errors"

	"changeme/internal/model"
)

func (m *SFTPTransferManager) ResolveConflict(ctx context.Context, id, action string) error {
	switch action {
	case "overwrite":
		_, err := m.ConfirmOverwrite(ctx, id)
		return err
	case "copy":
		_, err := m.restart(ctx, id, model.SFTPTransferAwaitingOverwrite, false, true, ErrSFTPTransferNotConfirmable)
		return err
	case "cancel", "stop":
	default:
		return model.ErrInvalidSFTPTransfer
	}
	task, err := m.lookup(ctx, id)
	if err != nil {
		return err
	}
	task.mu.RLock()
	batchID, sessionID, state := task.request.BatchID, task.request.SessionID, task.snapshot.State
	task.mu.RUnlock()
	if state != model.SFTPTransferAwaitingOverwrite {
		return ErrSFTPTransferNotConfirmable
	}
	if action == "cancel" {
		return m.Cancel(ctx, id)
	}

	m.mu.Lock()
	m.stoppedBatches[batchID] = true
	ids := make([]string, 0)
	for candidateID, candidate := range m.tasks {
		if candidate.request.BatchID == batchID && candidate.request.SessionID == sessionID {
			ids = append(ids, candidateID)
		}
	}
	m.mu.Unlock()
	var result error
	for _, candidateID := range ids {
		result = errors.Join(result, m.Cancel(ctx, candidateID))
	}
	return result
}

func (s *Service) ResolveSFTPTransferConflict(ctx context.Context, id, action string) error {
	_, transfers, _, err := s.transferDependencies()
	if err != nil {
		return err
	}
	if err := transfers.ResolveConflict(ctx, id, action); err != nil {
		return TransferServiceError(err)
	}
	return nil
}
