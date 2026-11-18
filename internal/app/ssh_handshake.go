package app

import (
	"changeme/internal/model"
)

func (s *Service) SSHHandshakeAlgorithmCatalog() model.SSHHandshakeAlgorithms {
	return model.NewSSHHandshakeAlgorithms()
}
