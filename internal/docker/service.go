package docker

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"changeme/internal/errno"
	"changeme/internal/model"
	"changeme/internal/tunnel"
)

type Runner interface {
	RunDockerCommand(ctx context.Context, args []string) ([]byte, int, error)
}

var (
	ErrUnavailable = tunnel.ErrDockerUnavailable

	ErrPermissionDenied = tunnel.ErrDockerPermissionDenied

	ErrDaemonUnavailable = tunnel.ErrDockerDaemonUnavailable

	ErrOutputTooLarge = tunnel.ErrDockerOutputTooLarge

	ErrInvalidOutput = errInvalidOutput
)

const (
	composeProjectLabel     = "com.docker.compose.project"
	composeServiceLabel     = "com.docker.compose.service"
	composeWorkingDirLabel  = "com.docker.compose.project.working_dir"
	composeConfigFilesLabel = "com.docker.compose.project.config_files"
)

const (
	messageUnavailable       = "docker.unavailable"
	messagePermissionDenied  = "docker.permission_denied"
	messageDaemonUnavailable = "docker.daemon_unavailable"
	messageOutputTooLarge    = "docker.output_too_large"
	messageCommandFailed     = "docker.command_failed"
	messageInvalidContainer  = "docker.invalid_container"
	messageInvalidAction     = "docker.invalid_action"

	messageInvalidImage   = "docker.invalid_image"
	messageInvalidVolume  = "docker.invalid_volume"
	messageInvalidNetwork = "docker.invalid_network"

	messageNotCompose = "docker.not_compose"
)

type CommandError struct {
	ExitCode int
}

func (e *CommandError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("docker command failed with exit status %d", e.ExitCode)
}

type Service struct {
	runner Runner
}

func NewService(runner Runner) *Service {
	return &Service{runner: runner}
}

func MapError(err error) error {
	if err == nil {
		return nil
	}
	if errno.Extract(err) != nil {
		return err
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	switch {
	case errors.Is(err, ErrInvalidContainerID):
		return errno.New(errno.CodeInvalidArg, messageInvalidContainer, "")
	case errors.Is(err, ErrInvalidAction):
		return errno.New(errno.CodeInvalidArg, messageInvalidAction, "")
	case errors.Is(err, ErrInvalidImageRef):
		return errno.New(errno.CodeInvalidArg, messageInvalidImage, "")
	case errors.Is(err, ErrInvalidVolumeName):
		return errno.New(errno.CodeInvalidArg, messageInvalidVolume, "")
	case errors.Is(err, ErrInvalidNetworkID):
		return errno.New(errno.CodeInvalidArg, messageInvalidNetwork, "")
	case errors.Is(err, ErrNotComposeProject):
		return errno.New(errno.CodeInvalidArg, messageNotCompose, "")
	case errors.Is(err, ErrUnavailable):
		return errno.New(errno.CodeUnknown, messageUnavailable, "")
	case errors.Is(err, ErrPermissionDenied):
		return errno.New(errno.CodePermissionDenied, messagePermissionDenied, "")
	case errors.Is(err, ErrDaemonUnavailable):
		return errno.New(errno.CodeUnknown, messageDaemonUnavailable, "")
	case errors.Is(err, ErrOutputTooLarge):
		return errno.New(errno.CodeUnknown, messageOutputTooLarge, "")
	default:

		return errno.New(errno.CodeUnknown, messageCommandFailed, "")
	}
}

func (s *Service) Environment(ctx context.Context) (model.DockerEnvironment, error) {
	environment := model.DockerEnvironment{}
	output, err := s.execute(ctx, dockerVersionCommand())
	if err != nil {
		if message := availabilityMessage(err); message != "" {
			environment.Message = message
			return environment, nil
		}
		return model.DockerEnvironment{}, MapError(err)
	}
	info, err := parseDockerVersion(output)
	if err != nil {

		environment.Message = messageCommandFailed
		return environment, nil
	}
	environment.Available = true
	environment.DockerVersion = info.serverVersion
	if environment.DockerVersion == "" {
		environment.DockerVersion = info.clientVersion
	}
	environment.APIVersion = info.serverAPIVersion
	if environment.APIVersion == "" {
		environment.APIVersion = info.clientAPIVersion
	}

	if composeOutput, composeErr := s.execute(ctx, dockerComposeVersionCommand()); composeErr == nil {
		environment.ComposeVersion = strings.TrimSpace(string(composeOutput))
	}
	return environment, nil
}

func (s *Service) Containers(ctx context.Context) ([]model.DockerContainer, error) {
	output, err := s.execute(ctx, dockerContainersCommand())
	if err != nil {
		return nil, MapError(err)
	}
	containers, _ := parseContainerList(output)
	return containers, nil
}

func (s *Service) ContainerDetail(ctx context.Context, id string) (model.DockerContainerDetail, error) {
	args, err := dockerInspectCommand(id)
	if err != nil {
		return model.DockerContainerDetail{}, MapError(err)
	}
	output, err := s.execute(ctx, args)
	if err != nil {
		return model.DockerContainerDetail{}, MapError(err)
	}
	detail, err := parseContainerDetail(output)
	if err != nil {
		return model.DockerContainerDetail{}, MapError(err)
	}
	return detail, nil
}

func (s *Service) ContainerTarget(ctx context.Context, id string) (ContainerTarget, error) {
	args, err := dockerInspectLabelsCommand(id)
	if err != nil {
		return ContainerTarget{}, MapError(err)
	}
	output, err := s.execute(ctx, args)
	if err != nil {
		return ContainerTarget{}, MapError(err)
	}
	labels, err := parseContainerLabels(output)
	if err != nil {
		return ContainerTarget{}, MapError(err)
	}
	return ContainerTarget{
		ID:         id,
		Project:    labels[composeProjectLabel],
		Service:    labels[composeServiceLabel],
		ProjectDir: labels[composeWorkingDirLabel],

		ConfigFile: firstConfigFile(labels[composeConfigFilesLabel]),
	}, nil
}

func (s *Service) ContainerLogsCommand(ctx context.Context, id string) (string, error) {
	target, err := s.ContainerTarget(ctx, id)
	if err != nil {
		return "", err
	}
	command, err := ContainerLogsShellCommand(target)
	if err != nil {
		return "", MapError(err)
	}
	return command, nil
}

func (s *Service) ContainerStatsCommand(ctx context.Context, id string) (string, error) {
	target, err := s.ContainerTarget(ctx, id)
	if err != nil {
		return "", err
	}
	command, err := ContainerStatsShellCommand(target)
	if err != nil {
		return "", MapError(err)
	}
	return command, nil
}

func firstConfigFile(value string) string {
	for _, part := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (s *Service) ComposeLogsCommand(ctx context.Context, id string) (string, error) {
	target, err := s.ContainerTarget(ctx, id)
	if err != nil {
		return "", err
	}
	command, err := ComposeLogsShellCommand(target)
	if err != nil {
		return "", MapError(err)
	}
	return command, nil
}

func (s *Service) ComposeStatsCommand(ctx context.Context, id string) (string, error) {
	target, err := s.ContainerTarget(ctx, id)
	if err != nil {
		return "", err
	}
	command, err := ComposeStatsShellCommand(target)
	if err != nil {
		return "", MapError(err)
	}
	return command, nil
}

func (s *Service) ComposeAction(ctx context.Context, id string, action ComposeAction) error {
	target, err := s.ContainerTarget(ctx, id)
	if err != nil {
		return err
	}
	args, err := ComposeActionCommand(target, action)
	if err != nil {
		return MapError(err)
	}
	if _, err := s.execute(ctx, args); err != nil {
		return MapError(err)
	}
	return nil
}

func (s *Service) ComposeFilePath(ctx context.Context, id string) (string, error) {
	target, err := s.ContainerTarget(ctx, id)
	if err != nil {
		return "", err
	}
	if !target.isComposeProject() {
		return "", MapError(ErrNotComposeProject)
	}
	return target.ConfigFile, nil
}

func (s *Service) ContainerAction(ctx context.Context, id string, action string, force bool) error {
	args, err := dockerActionCommand(id, action, force)
	if err != nil {
		return MapError(err)
	}
	if _, err := s.execute(ctx, args); err != nil {
		return MapError(err)
	}
	return nil
}

func (s *Service) Images(ctx context.Context) ([]model.DockerImage, error) {
	output, err := s.execute(ctx, dockerImagesCommand())
	if err != nil {
		return nil, MapError(err)
	}
	images, _ := parseImageList(output)

	danglingOutput, danglingErr := s.execute(ctx, dockerDanglingImagesCommand())
	if danglingErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, MapError(ctxErr)
		}
	} else {
		dangling, _ := parseImageList(danglingOutput)
		images = mergeImages(images, dangling)
	}
	index, err := s.containerInUseIndex(ctx)
	if err != nil {
		return nil, MapError(err)
	}
	markImageInUse(images, index)
	return images, nil
}

func mergeImages(images []model.DockerImage, extra []model.DockerImage) []model.DockerImage {
	if len(extra) == 0 {
		return images
	}
	seen := make(map[string]struct{}, len(images)+len(extra))
	for _, image := range images {
		seen[image.ID+"\x00"+image.Repository+"\x00"+image.Tag] = struct{}{}
	}
	for _, image := range extra {
		key := image.ID + "\x00" + image.Repository + "\x00" + image.Tag
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		images = append(images, image)
	}
	return images
}

func (s *Service) Volumes(ctx context.Context) ([]model.DockerVolume, error) {
	output, err := s.execute(ctx, dockerVolumesCommand())
	if err != nil {
		return nil, MapError(err)
	}
	volumes, _ := parseVolumeList(output)
	index, err := s.containerInUseIndex(ctx)
	if err != nil {
		return nil, MapError(err)
	}
	markVolumeInUse(volumes, index)
	return volumes, nil
}

func (s *Service) Networks(ctx context.Context) ([]model.DockerNetwork, error) {
	output, err := s.execute(ctx, dockerNetworksCommand())
	if err != nil {
		return nil, MapError(err)
	}
	networks, _ := parseNetworkList(output)
	index, err := s.containerNetworkInUseIndex(ctx)
	if err != nil {
		return nil, MapError(err)
	}
	markNetworkState(networks, index)
	return networks, nil
}

func (s *Service) ImageDetail(ctx context.Context, ref string) (model.DockerImageDetail, error) {
	args, err := dockerImageInspectCommand(ref)
	if err != nil {
		return model.DockerImageDetail{}, MapError(err)
	}
	output, err := s.execute(ctx, args)
	if err != nil {
		return model.DockerImageDetail{}, MapError(err)
	}
	detail, err := parseImageDetail(output)
	if err != nil {
		return model.DockerImageDetail{}, MapError(err)
	}
	return detail, nil
}

func (s *Service) VolumeDetail(ctx context.Context, name string) (model.DockerVolumeDetail, error) {
	args, err := dockerVolumeInspectCommand(name)
	if err != nil {
		return model.DockerVolumeDetail{}, MapError(err)
	}
	output, err := s.execute(ctx, args)
	if err != nil {
		return model.DockerVolumeDetail{}, MapError(err)
	}
	detail, err := parseVolumeDetail(output)
	if err != nil {
		return model.DockerVolumeDetail{}, MapError(err)
	}
	return detail, nil
}

func (s *Service) NetworkDetail(ctx context.Context, id string) (model.DockerNetworkDetail, error) {
	args, err := dockerNetworkInspectCommand(id)
	if err != nil {
		return model.DockerNetworkDetail{}, MapError(err)
	}
	output, err := s.execute(ctx, args)
	if err != nil {
		return model.DockerNetworkDetail{}, MapError(err)
	}
	detail, err := parseNetworkDetail(output)
	if err != nil {
		return model.DockerNetworkDetail{}, MapError(err)
	}
	return detail, nil
}

func (s *Service) RemoveImage(ctx context.Context, imageID string) error {
	args, err := dockerRemoveImageCommand(imageID)
	if err != nil {
		return MapError(err)
	}
	if _, err := s.execute(ctx, args); err != nil {
		return MapError(err)
	}
	return nil
}

func (s *Service) RemoveVolume(ctx context.Context, volumeName string) error {
	args, err := dockerRemoveVolumeCommand(volumeName)
	if err != nil {
		return MapError(err)
	}
	if _, err := s.execute(ctx, args); err != nil {
		return MapError(err)
	}
	return nil
}

func (s *Service) RemoveNetwork(ctx context.Context, networkID string) error {
	args, err := dockerRemoveNetworkCommand(networkID)
	if err != nil {
		return MapError(err)
	}
	if _, err := s.execute(ctx, args); err != nil {
		return MapError(err)
	}
	return nil
}

func (s *Service) PruneNetworks(ctx context.Context) error {
	if _, err := s.execute(ctx, dockerPruneNetworksCommand()); err != nil {
		return MapError(err)
	}
	return nil
}

func (s *Service) PruneVolumes(ctx context.Context) error {
	if _, err := s.execute(ctx, dockerPruneVolumesCommand()); err != nil {
		return MapError(err)
	}
	return nil
}

func markNetworkState(networks []model.DockerNetwork, inUse map[string]struct{}) {
	for position := range networks {
		_, used := inUse[networks[position].Name]
		networks[position].InUse = used
		networks[position].BuiltIn = isBuiltInNetwork(networks[position].Name)
	}
}

func isBuiltInNetwork(name string) bool {
	switch name {
	case "bridge", "host", "none":
		return true
	default:
		return false
	}
}

func (s *Service) PruneImages(ctx context.Context, all bool) error {
	if _, err := s.execute(ctx, dockerPruneImagesCommand(all)); err != nil {
		return MapError(err)
	}
	return nil
}

func (s *Service) execute(ctx context.Context, args []string) ([]byte, error) {
	if ctx == nil {
		return nil, errno.New(errno.CodeInvalidArg, "error.invalidArgument", "")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.runner == nil {
		return nil, errno.New(errno.CodeUnknown, messageUnavailable, "")
	}
	output, exitCode, err := s.runner.RunDockerCommand(ctx, args)
	if err != nil {

		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, err
	}
	if exitCode != 0 {

		if exitCode == 127 {
			return nil, ErrUnavailable
		}
		return nil, &CommandError{ExitCode: exitCode}
	}
	return output, nil
}

func availabilityMessage(err error) string {
	switch {
	case errors.Is(err, ErrUnavailable):
		return messageUnavailable
	case errors.Is(err, ErrPermissionDenied):
		return messagePermissionDenied
	case errors.Is(err, ErrDaemonUnavailable):
		return messageDaemonUnavailable
	default:
		return ""
	}
}
