package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"changeme/internal/docker"
	"changeme/internal/errno"
	"changeme/internal/host"
	"changeme/internal/model"
	"changeme/internal/session"
)

const (
	dockerVersionTimeout = 15 * time.Second

	dockerQueryTimeout = 20 * time.Second

	dockerPruneTimeout = 30 * time.Second

	dockerComposeTimeout = 110 * time.Second
)

func (s *Service) DockerEnvironment(ctx context.Context, sessionID string) (model.DockerEnvironment, error) {
	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return model.DockerEnvironment{}, err
	}
	runner, err := s.dockerRunner(ctx, sessionID)
	if err != nil {
		return model.DockerEnvironment{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerVersionTimeout)
	defer cancel()
	return publicValue(docker.NewService(runner).Environment(callCtx))
}

func (s *Service) DockerContainers(ctx context.Context, sessionID string) ([]model.DockerContainer, error) {
	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return nil, err
	}
	runner, err := s.dockerRunner(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerQueryTimeout)
	defer cancel()
	return publicValue(docker.NewService(runner).Containers(callCtx))
}

func (s *Service) DockerContainerDetail(
	ctx context.Context,
	sessionID string,
	containerID string,
) (model.DockerContainerDetail, error) {
	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return model.DockerContainerDetail{}, err
	}
	runner, err := s.dockerRunner(ctx, sessionID)
	if err != nil {
		return model.DockerContainerDetail{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerQueryTimeout)
	defer cancel()
	return publicValue(docker.NewService(runner).ContainerDetail(callCtx, containerID))
}

func (s *Service) DockerOpenContainerLogs(
	ctx context.Context,
	sessionID string,
	containerID string,
) error {
	if ctx == nil {
		return errno.New(errno.CodeInvalidArg, "error.invalidArgument", "")
	}
	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return err
	}
	runner, err := s.dockerRunner(ctx, sessionID)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerQueryTimeout)
	defer cancel()
	command, err := docker.NewService(runner).ContainerLogsCommand(callCtx, containerID)
	if err != nil {
		return err
	}
	return s.writeDockerTerminalCommand(ctx, sessionID, command)
}

func (s *Service) DockerOpenContainerStats(
	ctx context.Context,
	sessionID string,
	containerID string,
) error {
	if ctx == nil {
		return errno.New(errno.CodeInvalidArg, "error.invalidArgument", "")
	}
	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return err
	}
	runner, err := s.dockerRunner(ctx, sessionID)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerQueryTimeout)
	defer cancel()
	command, err := docker.NewService(runner).ContainerStatsCommand(callCtx, containerID)
	if err != nil {
		return err
	}
	return s.writeDockerTerminalCommand(ctx, sessionID, command)
}

func (s *Service) DockerOpenComposeLogs(
	ctx context.Context,
	sessionID string,
	containerID string,
) error {
	if ctx == nil {
		return errno.New(errno.CodeInvalidArg, "error.invalidArgument", "")
	}
	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return err
	}
	runner, err := s.dockerRunner(ctx, sessionID)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerQueryTimeout)
	defer cancel()
	command, err := docker.NewService(runner).ComposeLogsCommand(callCtx, containerID)
	if err != nil {
		return err
	}
	return s.writeDockerTerminalCommand(ctx, sessionID, command)
}

func (s *Service) DockerOpenComposeStats(
	ctx context.Context,
	sessionID string,
	containerID string,
) error {
	if ctx == nil {
		return errno.New(errno.CodeInvalidArg, "error.invalidArgument", "")
	}
	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return err
	}
	runner, err := s.dockerRunner(ctx, sessionID)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerQueryTimeout)
	defer cancel()
	command, err := docker.NewService(runner).ComposeStatsCommand(callCtx, containerID)
	if err != nil {
		return err
	}
	return s.writeDockerTerminalCommand(ctx, sessionID, command)
}

func (s *Service) DockerComposeAction(
	ctx context.Context,
	sessionID string,
	containerID string,
	action string,
) error {
	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return err
	}
	runner, err := s.dockerRunner(ctx, sessionID)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerComposeTimeout)
	defer cancel()
	return docker.NewService(runner).ComposeAction(callCtx, containerID, docker.ComposeAction(action))
}

func (s *Service) DockerComposeFilePath(
	ctx context.Context,
	sessionID string,
	containerID string,
) (string, error) {
	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return "", err
	}
	runner, err := s.dockerRunner(ctx, sessionID)
	if err != nil {
		return "", err
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerQueryTimeout)
	defer cancel()
	return publicValue(docker.NewService(runner).ComposeFilePath(callCtx, containerID))
}

func (s *Service) DockerContainerAction(
	ctx context.Context,
	sessionID string,
	containerID string,
	action string,
	force bool,
) error {
	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return err
	}
	runner, err := s.dockerRunner(ctx, sessionID)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerQueryTimeout)
	defer cancel()
	return publicError(docker.NewService(runner).ContainerAction(callCtx, containerID, action, force))
}

func (s *Service) DockerImageDetail(
	ctx context.Context,
	sessionID string,
	imageRef string,
) (model.DockerImageDetail, error) {
	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return model.DockerImageDetail{}, err
	}
	runner, err := s.dockerRunner(ctx, sessionID)
	if err != nil {
		return model.DockerImageDetail{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerQueryTimeout)
	defer cancel()
	return publicValue(docker.NewService(runner).ImageDetail(callCtx, imageRef))
}

func (s *Service) DockerVolumeDetail(
	ctx context.Context,
	sessionID string,
	volumeName string,
) (model.DockerVolumeDetail, error) {
	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return model.DockerVolumeDetail{}, err
	}
	runner, err := s.dockerRunner(ctx, sessionID)
	if err != nil {
		return model.DockerVolumeDetail{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerQueryTimeout)
	defer cancel()
	return publicValue(docker.NewService(runner).VolumeDetail(callCtx, volumeName))
}

func (s *Service) DockerNetworkDetail(
	ctx context.Context,
	sessionID string,
	networkID string,
) (model.DockerNetworkDetail, error) {
	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return model.DockerNetworkDetail{}, err
	}
	runner, err := s.dockerRunner(ctx, sessionID)
	if err != nil {
		return model.DockerNetworkDetail{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerQueryTimeout)
	defer cancel()
	return publicValue(docker.NewService(runner).NetworkDetail(callCtx, networkID))
}

func (s *Service) DockerImages(ctx context.Context, sessionID string) ([]model.DockerImage, error) {
	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return nil, err
	}
	runner, err := s.dockerRunner(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerQueryTimeout)
	defer cancel()
	return publicValue(docker.NewService(runner).Images(callCtx))
}

func (s *Service) DockerVolumes(ctx context.Context, sessionID string) ([]model.DockerVolume, error) {
	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return nil, err
	}
	runner, err := s.dockerRunner(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerQueryTimeout)
	defer cancel()
	return publicValue(docker.NewService(runner).Volumes(callCtx))
}

func (s *Service) DockerNetworks(ctx context.Context, sessionID string) ([]model.DockerNetwork, error) {
	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return nil, err
	}
	runner, err := s.dockerRunner(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerQueryTimeout)
	defer cancel()
	return publicValue(docker.NewService(runner).Networks(callCtx))
}

func (s *Service) DockerRemoveImage(ctx context.Context, sessionID, imageID string) error {
	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return err
	}
	runner, err := s.dockerRunner(ctx, sessionID)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerQueryTimeout)
	defer cancel()
	return publicError(docker.NewService(runner).RemoveImage(callCtx, imageID))
}

func (s *Service) DockerRemoveVolume(ctx context.Context, sessionID, volumeName string) error {
	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return err
	}
	runner, err := s.dockerRunner(ctx, sessionID)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerQueryTimeout)
	defer cancel()
	return publicError(docker.NewService(runner).RemoveVolume(callCtx, volumeName))
}

func (s *Service) DockerRemoveNetwork(ctx context.Context, sessionID, networkID string) error {
	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return err
	}
	runner, err := s.dockerRunner(ctx, sessionID)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerQueryTimeout)
	defer cancel()
	return publicError(docker.NewService(runner).RemoveNetwork(callCtx, networkID))
}

func (s *Service) DockerPruneNetworks(ctx context.Context, sessionID string) error {
	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return err
	}
	runner, err := s.dockerRunner(ctx, sessionID)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerPruneTimeout)
	defer cancel()
	return publicError(docker.NewService(runner).PruneNetworks(callCtx))
}

func (s *Service) DockerPruneVolumes(ctx context.Context, sessionID string) error {
	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return err
	}
	runner, err := s.dockerRunner(ctx, sessionID)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerPruneTimeout)
	defer cancel()
	return publicError(docker.NewService(runner).PruneVolumes(callCtx))
}

func (s *Service) DockerPruneImages(ctx context.Context, sessionID string, all bool) error {
	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return err
	}
	runner, err := s.dockerRunner(ctx, sessionID)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, dockerPruneTimeout)
	defer cancel()
	return publicError(docker.NewService(runner).PruneImages(callCtx, all))
}

func (s *Service) DockerOpenContainerTerminal(
	ctx context.Context,
	sessionID string,
	containerID string,
) error {
	if ctx == nil {
		return errno.New(errno.CodeInvalidArg, "error.invalidArgument", "")
	}

	if err := s.requireDockerPanel(ctx, sessionID); err != nil {
		return err
	}

	command, err := docker.ContainerShellCommand(containerID)
	if err != nil {
		return docker.MapError(err)
	}
	if s.sessions == nil {
		return errno.New(errno.CodeSessionClosed, "session.disconnected", "")
	}
	manager, err := s.sessions.Manager()
	if err != nil {
		return publicError(err)
	}

	if _, err := manager.DockerCommandRunner(ctx, sessionID); err != nil {
		return dockerSessionError(err)
	}
	if err := manager.WriteTerminalInput(ctx, sessionID, []byte(command)); err != nil {
		return dockerTerminalWriteError(err)
	}
	return nil
}

func (s *Service) writeDockerTerminalCommand(ctx context.Context, sessionID, command string) error {
	if s.sessions == nil {
		return errno.New(errno.CodeSessionClosed, "session.disconnected", "")
	}
	manager, err := s.sessions.Manager()
	if err != nil {
		return publicError(err)
	}
	if _, err := manager.DockerCommandRunner(ctx, sessionID); err != nil {
		return dockerSessionError(err)
	}
	if err := manager.WriteTerminalInput(ctx, sessionID, []byte(command)); err != nil {
		return dockerTerminalWriteError(err)
	}
	return nil
}

func dockerTerminalWriteError(err error) error {
	switch {
	case errors.Is(err, session.ErrSessionTerminalUnsupported):

		return errno.New(errno.CodeUnknown, "docker.terminal_unavailable", "")
	case errors.Is(err, session.ErrSessionTerminalDetached),
		errors.Is(err, session.ErrSessionNotConnected):
		return errno.New(errno.CodeSessionClosed, "session.disconnected", "")
	case errno.Extract(err) != nil:
		return err
	default:
		return publicError(err)
	}
}

type dockerTestHooks struct {
	panel *dockerPanelDeps

	runner func(ctx context.Context, sessionID string) (docker.Runner, error)
}

func (s *Service) dockerRunner(ctx context.Context, sessionID string) (docker.Runner, error) {
	if ctx == nil {
		return nil, errno.New(errno.CodeInvalidArg, "error.invalidArgument", "")
	}

	if s.dockerHooks != nil && s.dockerHooks.runner != nil {
		runner, err := s.dockerHooks.runner(ctx, sessionID)
		if err != nil {
			return nil, dockerSessionError(err)
		}
		if runner == nil {
			return nil, errno.New(errno.CodeUnknown, "docker.unavailable", "")
		}
		return runner, nil
	}
	if s.sessions == nil {
		return nil, errno.New(errno.CodeSessionClosed, "session.disconnected", "")
	}
	manager, err := s.sessions.Manager()
	if err != nil {
		return nil, publicError(err)
	}
	runner, err := manager.DockerCommandRunner(ctx, sessionID)
	if err != nil {
		return nil, dockerSessionError(err)
	}
	if runner == nil {
		return nil, errno.New(errno.CodeUnknown, "docker.unavailable", "")
	}
	return runner, nil
}

func dockerSessionError(err error) error {
	if err == nil {
		return nil
	}
	if errno.Extract(err) != nil {
		return err
	}
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, session.ErrDockerUnsupported):

		return errno.New(errno.CodeUnknown, "docker.unavailable", "")
	case errors.Is(err, session.ErrSessionNotFound):
		return errno.New(errno.CodeNotFound, "session.not_found", "")
	case errors.Is(err, session.ErrSessionNotConnected):
		return errno.New(errno.CodeSessionClosed, "session.disconnected", "")
	default:
		return publicError(err)
	}
}

const (
	dockerPanelPreferencePrefix = "qterm.docker.panel.enabled."

	dockerPanelEnabledValue = "1"

	dockerPanelDisabledValue = "0"

	messageDockerNotEnabled = "docker.not_enabled"

	messageDockerPreferenceFailed = "docker.preference_failed"
)

type dockerPanelSession interface {
	GetSession(ctx context.Context, id string) (model.Session, error)
}

type dockerPanelPreference interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string) error
}

type dockerPanelDeps struct {
	sessions    dockerPanelSession
	preferences dockerPanelPreference
}

func dockerPanelPreferenceKey(hostID string) string {
	return dockerPanelPreferencePrefix + hostID
}

func (s *Service) dockerPanel() (dockerPanelDeps, error) {
	if s == nil {
		return dockerPanelDeps{}, errno.New(errno.CodeSessionClosed, "session.disconnected", "")
	}
	if s.dockerHooks != nil && s.dockerHooks.panel != nil {
		return *s.dockerHooks.panel, nil
	}
	if s.sessions == nil {
		return dockerPanelDeps{}, errno.New(errno.CodeSessionClosed, "session.disconnected", "")
	}
	if s.preferences == nil {
		return dockerPanelDeps{}, errno.New(errno.CodeUnknown, messageDockerPreferenceFailed, "")
	}
	return dockerPanelDeps{sessions: s.sessions, preferences: s.preferences}, nil
}

func (d dockerPanelDeps) sessionHostID(ctx context.Context, sessionID string) (string, error) {
	if d.sessions == nil {
		return "", errno.New(errno.CodeSessionClosed, "session.disconnected", "")
	}
	snapshot, err := d.sessions.GetSession(ctx, sessionID)
	if err != nil {
		return "", dockerSessionError(err)
	}
	return strings.TrimSpace(snapshot.HostID), nil
}

func (d dockerPanelDeps) enabledForHost(ctx context.Context, hostID string) (bool, error) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {

		return false, nil
	}
	if d.preferences == nil {
		return false, errno.New(errno.CodeUnknown, messageDockerPreferenceFailed, "")
	}
	value, found, err := d.preferences.Get(ctx, dockerPanelPreferenceKey(hostID))
	if err != nil {
		return false, errno.New(errno.CodeUnknown, messageDockerPreferenceFailed, "")
	}
	if !found {
		return false, nil
	}
	return value == dockerPanelEnabledValue, nil
}

func (d dockerPanelDeps) setEnabledForHost(ctx context.Context, hostID string, enabled bool) error {
	if d.preferences == nil {
		return errno.New(errno.CodeUnknown, messageDockerPreferenceFailed, "")
	}
	value := dockerPanelDisabledValue
	if enabled {
		value = dockerPanelEnabledValue
	}
	if err := d.preferences.Set(ctx, dockerPanelPreferenceKey(hostID), value); err != nil {

		return errno.New(errno.CodeUnknown, messageDockerPreferenceFailed, "")
	}
	return nil
}

func (s *Service) requireDockerPanel(ctx context.Context, sessionID string) error {
	if ctx == nil {
		return errno.New(errno.CodeInvalidArg, "error.invalidArgument", "")
	}
	deps, err := s.dockerPanel()
	if err != nil {
		return err
	}
	hostID, err := deps.sessionHostID(ctx, sessionID)
	if err != nil {
		return err
	}
	enabled, err := deps.enabledForHost(ctx, hostID)
	if err != nil {
		return err
	}
	if !enabled {
		return errno.New(errno.CodePermissionDenied, messageDockerNotEnabled, "")
	}
	return nil
}

func (s *Service) DockerPanelEnabled(ctx context.Context, sessionID string) (bool, error) {
	if ctx == nil {
		return false, errno.New(errno.CodeInvalidArg, "error.invalidArgument", "")
	}
	deps, err := s.dockerPanel()
	if err != nil {
		return false, err
	}
	hostID, err := deps.sessionHostID(ctx, sessionID)
	if err != nil {
		return false, err
	}
	return deps.enabledForHost(ctx, hostID)
}

func (s *Service) SetDockerPanelEnabled(ctx context.Context, hostID string, enabled bool) error {
	if ctx == nil {
		return errno.New(errno.CodeInvalidArg, "error.invalidArgument", "")
	}
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return errno.New(errno.CodeInvalidArg, "error.invalidArgument", "")
	}

	if _, err := host.SessionHost(s.hosts, ctx, hostID); err != nil {
		return publicError(err)
	}
	deps, err := s.dockerPanel()
	if err != nil {
		return err
	}
	return deps.setEnabledForHost(ctx, hostID, enabled)
}
