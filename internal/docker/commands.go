package docker

import (
	"errors"
	"regexp"
	"strings"
)

const (
	dockerExecutable = "docker"

	containerIDPattern = `^[a-f0-9]{12,64}$`

	containerShellInnerCommand = `command -v bash >/dev/null 2>&1 && exec bash || exec sh`

	actionStart   = "start"
	actionStop    = "stop"
	actionRestart = "restart"
	actionRemove  = "remove"

	removeForceFlag = "-f"

	containerListFormat = `{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.State}}\t{{.Status}}\t{{.Ports}}\t{{.CreatedAt}}\t{{.Label "com.docker.compose.project"}}\t{{.Label "com.docker.compose.service"}}\t{{.Label "com.docker.compose.project.working_dir"}}`

	containerListColumns = 10

	versionFormat = `{{json .}}`

	composeVersionShortFormat = "--short"

	imageDigestRefPattern = `^sha256:[a-f0-9]{64}$`
	imageShortRefPattern  = `^[a-f0-9]{12,64}$`

	imageTagRefPattern = `^[a-z0-9][a-z0-9._:/-]*:[a-zA-Z0-9_.-]{1,128}$`

	volumeNamePattern = `^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,254}$`

	imageListFormat  = `{{json .}}`
	volumeListFormat = `{{json .}}`

	networkListFormat = `{{json .}}`

	networkInUseFormat = `{{json .NetworkSettings.Networks}}`

	containerInUseFormat = `{{.Image}}\t{{json .Mounts}}`

	inUseInspectBatchSize = 100

	pruneAllFlag   = "-a"
	pruneForceFlag = "-f"
)

var containerIDRegexp = regexp.MustCompile(containerIDPattern)

var (
	imageDigestRefRegexp = regexp.MustCompile(imageDigestRefPattern)
	imageShortRefRegexp  = regexp.MustCompile(imageShortRefPattern)
	imageTagRefRegexp    = regexp.MustCompile(imageTagRefPattern)

	volumeNameRegexp = regexp.MustCompile(volumeNamePattern)

	composeProjectNameRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
)

var (
	ErrInvalidContainerID = errors.New("invalid docker container id")

	ErrInvalidAction = errors.New("invalid docker container action")

	ErrInvalidImageRef = errors.New("invalid docker image reference")

	ErrInvalidVolumeName = errors.New("invalid docker volume name")

	ErrInvalidNetworkID = errors.New("invalid docker network id")

	ErrNotComposeProject = errors.New("container is not part of a compose project with a known config file")

	errTooManyContainerIDs = errors.New("too many container ids for one docker inspect")
)

func ValidateContainerID(id string) error {
	if !containerIDRegexp.MatchString(id) {
		return ErrInvalidContainerID
	}
	return nil
}

func ContainerShellCommand(id string) (string, error) {
	if err := ValidateContainerID(id); err != nil {
		return "", err
	}
	var builder strings.Builder
	builder.WriteString(dockerExecutable)
	builder.WriteString(" exec -it ")
	builder.WriteString(quotePOSIXShellArgument(id))
	builder.WriteString(" sh -lc ")
	builder.WriteString(quotePOSIXShellArgument(containerShellInnerCommand))
	builder.WriteString("\r")
	return builder.String(), nil
}

func dockerVersionCommand() []string {
	return []string{"version", "--format", versionFormat}
}

func dockerComposeVersionCommand() []string {
	return []string{"compose", "version", composeVersionShortFormat}
}

func dockerContainersCommand() []string {
	return []string{"ps", "-a", "--format", containerListFormat}
}

func dockerInspectCommand(id string) ([]string, error) {
	if err := ValidateContainerID(id); err != nil {
		return nil, err
	}
	return []string{"inspect", id}, nil
}

func dockerInspectLabelsCommand(id string) ([]string, error) {
	if err := ValidateContainerID(id); err != nil {
		return nil, err
	}
	return []string{"inspect", "--format", `{{json .Config.Labels}}`, id}, nil
}

func dockerImageInspectCommand(ref string) ([]string, error) {
	if err := ValidateImageRef(ref); err != nil {
		return nil, err
	}
	return []string{"image", "inspect", ref}, nil
}

func dockerVolumeInspectCommand(name string) ([]string, error) {
	if err := ValidateVolumeName(name); err != nil {
		return nil, err
	}
	return []string{"volume", "inspect", name}, nil
}

func dockerNetworkInspectCommand(id string) ([]string, error) {
	if err := ValidateNetworkID(id); err != nil {
		return nil, err
	}
	return []string{"network", "inspect", id}, nil
}

type ContainerTarget struct {
	ID         string
	Project    string
	Service    string
	ProjectDir string
	ConfigFile string
}

func (t ContainerTarget) isCompose() bool {
	return t.Project != "" && t.Service != "" && t.ConfigFile != ""
}

func ContainerLogsShellCommand(target ContainerTarget) (string, error) {
	if err := ValidateContainerID(target.ID); err != nil {
		return "", err
	}
	if target.isCompose() {
		command := ""
		if target.ProjectDir != "" {
			command = "cd " + quotePOSIXShellArgument(target.ProjectDir) + " && "
		}
		command += "docker compose -p " + quotePOSIXShellArgument(target.Project) +
			" -f " + quotePOSIXShellArgument(target.ConfigFile) +
			" logs -f --tail=200 " + quotePOSIXShellArgument(target.Service)
		return command + "\r", nil
	}
	return "docker logs -f --tail=200 " + quotePOSIXShellArgument(target.ID) + "\r", nil
}

func ContainerStatsShellCommand(target ContainerTarget) (string, error) {
	if err := ValidateContainerID(target.ID); err != nil {
		return "", err
	}
	if target.isCompose() {
		prefix := ""
		if target.ProjectDir != "" {
			prefix = "cd " + quotePOSIXShellArgument(target.ProjectDir) + " && "
		}
		composeCmd := "docker compose -p " + quotePOSIXShellArgument(target.Project) +
			" -f " + quotePOSIXShellArgument(target.ConfigFile) +
			" ps -q " + quotePOSIXShellArgument(target.Service)
		command := prefix + `ids="$(` + composeCmd + `)"; if [ -n "$ids" ]; then docker stats $ids; else echo 'No containers'; fi`
		return command + "\r", nil
	}
	return "docker stats " + quotePOSIXShellArgument(target.ID) + "\r", nil
}

func (t ContainerTarget) isComposeProject() bool {
	return t.Project != "" && t.ConfigFile != ""
}

func (t ContainerTarget) cdPrefix() string {
	if t.ProjectDir == "" {
		return ""
	}
	return "cd " + quotePOSIXShellArgument(t.ProjectDir) + " && "
}

func (t ContainerTarget) composeArgv() string {
	return "docker compose -p " + quotePOSIXShellArgument(t.Project) +
		" -f " + quotePOSIXShellArgument(t.ConfigFile) + " "
}

func ComposeLogsShellCommand(target ContainerTarget) (string, error) {
	if err := ValidateContainerID(target.ID); err != nil {
		return "", err
	}
	if !target.isComposeProject() {
		return "", ErrNotComposeProject
	}
	return target.cdPrefix() + target.composeArgv() + "logs -f --tail=200" + "\r", nil
}

func ComposeStatsShellCommand(target ContainerTarget) (string, error) {
	if err := ValidateContainerID(target.ID); err != nil {
		return "", err
	}
	if !target.isComposeProject() {
		return "", ErrNotComposeProject
	}
	composeCmd := target.composeArgv() + "ps -q"
	command := target.cdPrefix() + `ids="$(` + composeCmd + `)"; if [ -n "$ids" ]; then docker stats $ids; else echo 'No containers'; fi`
	return command + "\r", nil
}

func dockerActionCommand(id string, action string, force bool) ([]string, error) {
	if err := ValidateContainerID(id); err != nil {
		return nil, err
	}
	switch action {
	case actionStart, actionStop, actionRestart:
		return []string{action, id}, nil
	case actionRemove:
		if force {
			return []string{"rm", removeForceFlag, id}, nil
		}
		return []string{"rm", id}, nil
	default:
		return nil, ErrInvalidAction
	}
}

func ValidateImageRef(ref string) error {
	if len(ref) == 0 || len(ref) > 255 {
		return ErrInvalidImageRef
	}
	if imageDigestRefRegexp.MatchString(ref) || imageShortRefRegexp.MatchString(ref) {
		return nil
	}

	if strings.HasPrefix(ref, "sha256:") {
		return ErrInvalidImageRef
	}

	if imageTagRefRegexp.MatchString(ref) && !strings.Contains(ref, "..") {
		return nil
	}
	return ErrInvalidImageRef
}

func ValidateVolumeName(name string) error {
	if !volumeNameRegexp.MatchString(name) {
		return ErrInvalidVolumeName
	}
	return nil
}

func ValidateNetworkID(id string) error {
	if !containerIDRegexp.MatchString(id) {
		return ErrInvalidNetworkID
	}
	return nil
}

type ComposeAction string

const (
	ComposeActionUp ComposeAction = "up"

	ComposeActionStop ComposeAction = "stop"

	ComposeActionDown ComposeAction = "down"

	ComposeActionRestart ComposeAction = "restart"
)

func (a ComposeAction) IsValid() bool {
	switch a {
	case ComposeActionUp, ComposeActionStop, ComposeActionRestart, ComposeActionDown:
		return true
	default:
		return false
	}
}

func ComposeActionCommand(target ContainerTarget, action ComposeAction) ([]string, error) {
	if err := ValidateContainerID(target.ID); err != nil {
		return nil, err
	}
	if !action.IsValid() {
		return nil, ErrInvalidAction
	}
	if !target.isComposeProject() {
		return nil, ErrNotComposeProject
	}

	if !composeProjectNameRegexp.MatchString(target.Project) {
		return nil, ErrNotComposeProject
	}
	if !strings.HasPrefix(target.ConfigFile, "/") {
		return nil, ErrNotComposeProject
	}
	args := []string{"compose", "-p", target.Project, "-f", target.ConfigFile}
	switch action {
	case ComposeActionUp:
		return append(args, "up", "-d"), nil
	case ComposeActionRestart:
		return append(args, "up", "-d", "--force-recreate"), nil
	case ComposeActionStop:
		return append(args, "stop"), nil
	case ComposeActionDown:
		return append(args, "down"), nil
	}
	return nil, ErrInvalidAction
}

func dockerImagesCommand() []string {
	return []string{"images", "--no-trunc", "--format", imageListFormat}
}

func dockerDanglingImagesCommand() []string {
	return []string{"images", "-f", "dangling=true", "--no-trunc", "--format", imageListFormat}
}

func dockerVolumesCommand() []string {
	return []string{"volume", "ls", "--format", volumeListFormat}
}

func dockerNetworksCommand() []string {
	return []string{"network", "ls", "--no-trunc", "--format", networkListFormat}
}

func dockerContainerIDsCommand() []string {
	return []string{"ps", "-a", "-q"}
}

func dockerInUseInspectCommand(ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, ErrInvalidContainerID
	}
	if len(ids) > inUseInspectBatchSize {
		return nil, errTooManyContainerIDs
	}
	args := make([]string, 0, len(ids)+3)
	args = append(args, "inspect", "--format", containerInUseFormat)
	for _, id := range ids {
		if err := ValidateContainerID(id); err != nil {
			return nil, err
		}
		args = append(args, id)
	}
	return args, nil
}

func dockerNetworkInUseInspectCommand(ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, ErrInvalidContainerID
	}
	if len(ids) > inUseInspectBatchSize {
		return nil, errTooManyContainerIDs
	}
	args := make([]string, 0, len(ids)+3)
	args = append(args, "inspect", "--format", networkInUseFormat)
	for _, id := range ids {
		if err := ValidateContainerID(id); err != nil {
			return nil, err
		}
		args = append(args, id)
	}
	return args, nil
}

func dockerRemoveImageCommand(target string) ([]string, error) {
	if err := ValidateImageRef(target); err != nil {
		return nil, err
	}
	return []string{"rmi", target}, nil
}

func dockerRemoveVolumeCommand(name string) ([]string, error) {
	if err := ValidateVolumeName(name); err != nil {
		return nil, err
	}
	return []string{"volume", "rm", name}, nil
}

func dockerRemoveNetworkCommand(networkID string) ([]string, error) {
	if err := ValidateNetworkID(networkID); err != nil {
		return nil, err
	}
	return []string{"network", "rm", networkID}, nil
}

func dockerPruneNetworksCommand() []string {
	return []string{"network", "prune", pruneForceFlag}
}

func dockerPruneVolumesCommand() []string {
	return []string{"volume", "prune", pruneForceFlag}
}

func dockerPruneImagesCommand(all bool) []string {
	if all {
		return []string{"image", "prune", pruneAllFlag, pruneForceFlag}
	}
	return []string{"image", "prune", pruneForceFlag}
}

func quotePOSIXShellArgument(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
