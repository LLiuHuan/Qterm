package tunnel

import (
	"context"
	"errors"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"changeme/internal/model"
)

const (
	serviceProbeTimeout = 8 * time.Second
	serviceOutputLimit  = 512 * 1024
	serviceLogOutputCap = 512 * 1024
	serviceListCommand  = `LC_ALL=C systemctl list-units --type=service --all --no-legend --no-pager --plain 2>/dev/null`
	serviceFilesCommand = `LC_ALL=C systemctl list-unit-files --type=service --no-legend --no-pager --plain 2>/dev/null`

	serviceCustomCommand = `LC_ALL=C ls -l /etc/systemd/system/*.service 2>/dev/null`

	serviceUserListCommand  = `XDG_RUNTIME_DIR=${XDG_RUNTIME_DIR:-/run/user/$(id -u)} LC_ALL=C systemctl --user list-units --type=service --all --no-legend --no-pager --plain 2>/dev/null`
	serviceUserFilesCommand = `XDG_RUNTIME_DIR=${XDG_RUNTIME_DIR:-/run/user/$(id -u)} LC_ALL=C systemctl --user list-unit-files --type=service --no-legend --no-pager --plain 2>/dev/null`

	serviceUserScopePrefix = "user:"
)

var unitNamePattern = regexp.MustCompile(`^[A-Za-z0-9@._:\\-]{1,128}\.service$`)

var ErrInvalidUnitName = errors.New("tunnel: invalid service unit name")

var ErrInvalidServiceLogLines = errors.New("tunnel: invalid service log lines")

type ServiceProbeFailureKind string

const (
	ServiceProbePermissionDenied   ServiceProbeFailureKind = "permission-denied"
	ServiceProbeManagerUnavailable ServiceProbeFailureKind = "manager-unavailable"
	ServiceProbeJournalUnavailable ServiceProbeFailureKind = "journal-unavailable"
	ServiceProbeUnitNotFound       ServiceProbeFailureKind = "unit-not-found"

	ServiceProbeLogsUnsupported ServiceProbeFailureKind = "logs-unsupported"
)

type ServiceProbeError struct {
	Kind ServiceProbeFailureKind
}

func (e *ServiceProbeError) Error() string {
	if e == nil {
		return "tunnel: service probe failed"
	}
	return "tunnel: service probe failed: " + string(e.Kind)
}

type ServiceAction string

const (
	ServiceStart      ServiceAction = "start"
	ServiceStop       ServiceAction = "stop"
	ServiceRestart    ServiceAction = "restart"
	ServiceDisable    ServiceAction = "disable"
	ServiceEnable     ServiceAction = "enable"
	ServiceDisableNow ServiceAction = "disable-now"
)

func (c *Connection) ReadServices(ctx context.Context) (model.ServiceSnapshot, error) {
	if c == nil {
		return model.ServiceSnapshot{}, ErrRemoteOSUnsupported
	}
	probeCtx, cancel := context.WithTimeout(ctx, serviceProbeTimeout)
	defer cancel()

	unitsOutput, err := c.runProbe(probeCtx, serviceListCommand, serviceOutputLimit)
	if err != nil {

		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return model.ServiceSnapshot{}, err
		}
		if snapshot, openrcErr := c.readOpenRCServices(probeCtx); openrcErr == nil {
			return snapshot, nil
		}

		return model.ServiceSnapshot{}, &ServiceProbeError{Kind: ServiceProbeManagerUnavailable}
	}

	filesOutput, _ := c.runProbe(probeCtx, serviceFilesCommand, serviceOutputLimit)
	customOutput, _ := c.runProbe(probeCtx, serviceCustomCommand, serviceOutputLimit)

	enabled := parseUnitFileStates(filesOutput)
	custom := parseCustomUnits(customOutput)
	services := mergeServiceUnits(parseServiceUnits(unitsOutput, enabled, custom), enabled, custom)

	if userUnitsOutput, userErr := c.runProbe(probeCtx, serviceUserListCommand, serviceOutputLimit); userErr == nil {
		userFilesOutput, _ := c.runProbe(probeCtx, serviceUserFilesCommand, serviceOutputLimit)
		userEnabled := parseUnitFileStates(userFilesOutput)
		userUnits := parseServiceUnits(userUnitsOutput, userEnabled, nil)
		for index := range userUnits {
			userUnits[index].Scope = "user"
		}

		services = append(services, userUnits...)
	}

	sort.SliceStable(services, func(i, j int) bool {
		return serviceLess(services[i], services[j])
	})

	return model.ServiceSnapshot{Manager: "systemd", Services: services, UpdatedAt: time.Now().UnixMilli()}, nil
}

func serviceSortRank(service model.ServiceInfo) int {
	switch service.ActiveState {
	case "active", "activating", "reloading":
		return 0
	default:
		return 1
	}
}

func serviceLess(left, right model.ServiceInfo) bool {
	if left.Custom != right.Custom {
		return left.Custom
	}
	leftRank, rightRank := serviceSortRank(left), serviceSortRank(right)
	if leftRank != rightRank {
		return leftRank < rightRank
	}
	return left.Name < right.Name
}

func (c *Connection) RunServiceAction(ctx context.Context, unit string, action ServiceAction) error {
	if c == nil {
		return ErrRemoteOSUnsupported
	}
	probeCtx, cancel := context.WithTimeout(ctx, serviceProbeTimeout)
	defer cancel()

	if manager := c.serviceManager(probeCtx); manager == "openrc" {
		if !openrcNamePattern.MatchString(unit) {
			return ErrInvalidUnitName
		}
		return c.runOpenRCAction(probeCtx, unit, action)
	}
	scope, name := splitServiceScope(unit)
	if !unitNamePattern.MatchString(name) {
		return ErrInvalidUnitName
	}
	systemctl := systemctlScoped(scope)
	var command string
	switch action {
	case ServiceStart:
		command = systemctl + " start " + quoteServiceUnit(name)
	case ServiceStop:
		command = systemctl + " stop " + quoteServiceUnit(name)
	case ServiceRestart:
		command = systemctl + " restart " + quoteServiceUnit(name)
	case ServiceEnable:
		command = systemctl + " enable " + quoteServiceUnit(name)
	case ServiceDisable:
		command = systemctl + " disable " + quoteServiceUnit(name)
	default:
		return errors.New("tunnel: unsupported service action")
	}
	_, err := c.runProbe(probeCtx, command+" 2>&1", serviceOutputLimit)
	return err
}

func (c *Connection) ReadServiceDetail(ctx context.Context, unit string) (model.ServiceDetail, error) {
	if c == nil {
		return model.ServiceDetail{}, ErrRemoteOSUnsupported
	}
	detailCtx, detailCancel := context.WithTimeout(ctx, serviceProbeTimeout)
	defer detailCancel()
	if manager := c.serviceManager(detailCtx); manager == "openrc" {
		if !openrcNamePattern.MatchString(unit) {
			return model.ServiceDetail{}, ErrInvalidUnitName
		}
		return c.readOpenRCDetail(detailCtx, unit)
	}
	scope, name := splitServiceScope(unit)
	if !unitNamePattern.MatchString(name) {
		return model.ServiceDetail{}, ErrInvalidUnitName
	}
	systemctl := systemctlScoped(scope)
	properties := "Id,Description,LoadState,ActiveState,SubState,UnitFileState,MainPID,ExecMainStartTimestamp,ExecMainExitTimestamp,ExecMainStatus,NRestarts,MemoryCurrent,TasksCurrent,FragmentPath"
	probeCtx, cancel := context.WithTimeout(ctx, serviceProbeTimeout)
	defer cancel()
	quotedUnit := quoteServiceUnit(name)
	output, stderr, err := c.runProbeWithStderr(probeCtx, "LC_ALL=C "+systemctl+" show "+quotedUnit+" --no-pager -p "+properties, serviceOutputLimit)
	fields := parseServiceShowFields(output)

	if err != nil {

		classified := classifyServiceProbeFailure("detail", stderr, err)
		if isHardServiceProbeFailure(classified) {
			return model.ServiceDetail{}, classified
		}
		if len(fields) > 0 {
			return model.ServiceDetail{Name: unit, Fields: fields}, nil
		}

		if servicePropertyUnsupported(stderr) {
			compatProperties := "Id,Description,LoadState,ActiveState,SubState,UnitFileState,MainPID,ExecMainStartTimestamp,MemoryCurrent,TasksCurrent,FragmentPath"
			output, stderr, err = c.runProbeWithStderr(probeCtx, "LC_ALL=C "+systemctl+" show "+quotedUnit+" --no-pager -p "+compatProperties, serviceOutputLimit)
			fields = parseServiceShowFields(output)
			if len(fields) > 0 && !isHardServiceProbeFailure(classifyServiceProbeFailure("detail", stderr, err)) {
				return model.ServiceDetail{Name: unit, Fields: fields}, nil
			}
		}
		return model.ServiceDetail{}, classified
	}
	if len(fields) == 0 {
		return model.ServiceDetail{}, errors.New("tunnel: service detail returned no properties")
	}
	return model.ServiceDetail{Name: unit, Fields: fields}, nil
}

func parseServiceShowFields(output string) map[string]string {
	fields := make(map[string]string, 16)
	for _, line := range strings.Split(output, "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if !found || key == "" {
			continue
		}
		fields[key] = value
	}
	return fields
}

func isHardServiceProbeFailure(err error) bool {
	var probeErr *ServiceProbeError
	if !errors.As(err, &probeErr) {

		return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
	}
	switch probeErr.Kind {
	case ServiceProbePermissionDenied, ServiceProbeManagerUnavailable,
		ServiceProbeUnitNotFound, ServiceProbeJournalUnavailable, ServiceProbeLogsUnsupported:
		return true
	default:
		return false
	}
}

func (c *Connection) ReadServiceLogs(ctx context.Context, unit string, lines int) (model.ServiceLog, error) {
	if c == nil {
		return model.ServiceLog{}, ErrRemoteOSUnsupported
	}
	logCtx, logCancel := context.WithTimeout(ctx, serviceProbeTimeout)
	defer logCancel()
	if manager := c.serviceManager(logCtx); manager == "openrc" {
		if !openrcNamePattern.MatchString(unit) {
			return model.ServiceLog{}, ErrInvalidUnitName
		}

		return model.ServiceLog{}, &ServiceProbeError{Kind: ServiceProbeLogsUnsupported}
	}
	logScope, logUnit := splitServiceScope(unit)
	if !unitNamePattern.MatchString(logUnit) {
		return model.ServiceLog{}, ErrInvalidUnitName
	}
	lineCount, ok := serviceLogLineCount(lines)
	if !ok {
		return model.ServiceLog{}, ErrInvalidServiceLogLines
	}
	journalctl := "journalctl"
	if logScope == "user" {

		journalctl = `XDG_RUNTIME_DIR=${XDG_RUNTIME_DIR:-/run/user/$(id -u)} journalctl --user`
	}
	command := "LC_ALL=C " + journalctl + " -u " + quoteServiceUnit(logUnit) + " -n " + strconv.Itoa(lineCount) + " --no-pager -o cat"
	output, stderr, err := c.runProbeWithStderr(logCtx, command, serviceLogOutputCap)
	if err != nil {
		return model.ServiceLog{}, classifyServiceProbeFailure("logs", stderr, err)
	}
	truncated := len(output) >= serviceLogOutputCap
	return model.ServiceLog{Name: unit, Text: output, Truncated: truncated}, nil
}

func quoteServiceUnit(unit string) string {
	return "'" + strings.ReplaceAll(unit, "'", "'\\''") + "'"
}

func servicePropertyUnsupported(stderr string) bool {
	text := strings.ToLower(strings.TrimSpace(stderr))
	return strings.Contains(text, "unknown assignment") ||
		strings.Contains(text, "unknown property") ||
		strings.Contains(text, "invalid property") ||
		strings.Contains(text, "unknown field")
}

func classifyServiceProbeFailure(operation, stderr string, runErr error) error {
	if runErr == nil {
		return nil
	}
	if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
		return runErr
	}
	text := strings.ToLower(strings.TrimSpace(stderr))
	if serviceTextMatches(text,
		"permission denied", "access denied", "not authorized",
		"authentication is required", "operation not permitted", "insufficient permissions") {
		return &ServiceProbeError{Kind: ServiceProbePermissionDenied}
	}
	if serviceTextMatches(text,
		"could not be found", "not-found", "unit not loaded", "unit not found") {
		return &ServiceProbeError{Kind: ServiceProbeUnitNotFound}
	}
	if operation == "logs" && serviceTextMatches(text,
		"journalctl: command not found", "journalctl: not found", "command not found: journalctl",
		"failed to open the journal", "cannot access journal", "journal has not been booted",
		"journal files are not available") {
		return &ServiceProbeError{Kind: ServiceProbeJournalUnavailable}
	}
	if serviceTextMatches(text,
		"system has not been booted with systemd", "failed to connect to bus",
		"could not connect to system bus", "systemd is not running",
		"systemctl: command not found", "systemctl: not found", "command not found: systemctl") {
		return &ServiceProbeError{Kind: ServiceProbeManagerUnavailable}
	}

	if strings.Contains(strings.ToLower(runErr.Error()), "status 127") {
		if operation == "logs" {
			return &ServiceProbeError{Kind: ServiceProbeJournalUnavailable}
		}
		return &ServiceProbeError{Kind: ServiceProbeManagerUnavailable}
	}
	return runErr
}

func serviceTextMatches(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func serviceLogLineCount(lines int) (int, bool) {
	switch lines {
	case 50, 100, 200, 500, 1000:
		return lines, true
	default:
		return 0, false
	}
}

func parseServiceUnits(output string, enabled map[string]string, custom map[string]bool) []model.ServiceInfo {
	result := make([]model.ServiceInfo, 0, 128)
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		fields := strings.Fields(trimmed)

		if len(fields) > 0 && fields[0] == "●" {
			fields = fields[1:]
		}
		if len(fields) < 4 || !strings.HasSuffix(fields[0], ".service") {
			continue
		}
		description := ""
		if len(fields) > 4 {
			description = strings.Join(fields[4:], " ")
		}
		name := fields[0]
		result = append(result, model.ServiceInfo{
			Name:         name,
			Description:  description,
			LoadState:    fields[1],
			ActiveState:  fields[2],
			SubState:     fields[3],
			EnabledState: enabled[name],
			Custom:       custom[name],
		})
	}
	return result
}

func mergeServiceUnits(units []model.ServiceInfo, enabled map[string]string, custom map[string]bool) []model.ServiceInfo {
	seen := make(map[string]bool, len(units))
	for _, unit := range units {
		seen[unit.Name] = true
	}
	for name, state := range enabled {

		if seen[name] || state == "alias" {
			continue
		}
		units = append(units, model.ServiceInfo{
			Name:         name,
			EnabledState: state,
			Custom:       custom[name],
		})
	}
	return units
}

func parseUnitFileStates(output string) map[string]string {
	states := make(map[string]string, 128)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 || !strings.HasSuffix(fields[0], ".service") {
			continue
		}
		states[fields[0]] = fields[1]
	}
	return states
}

func parseCustomUnits(output string) map[string]bool {
	result := make(map[string]bool, 32)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		left, target, isLink := strings.Cut(line, " -> ")
		name := serviceUnitNameFromListLine(left)
		if name == "" {
			continue
		}

		if isLink && isDistributionServicePath(strings.TrimSpace(target)) {
			continue
		}
		result[name] = true
	}
	return result
}

func serviceUnitNameFromListLine(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	name := path.Base(fields[len(fields)-1])
	if !strings.HasSuffix(name, ".service") {
		return ""
	}
	return name
}

func isDistributionServicePath(target string) bool {
	return strings.HasPrefix(target, "/usr/lib/systemd/system/") ||
		strings.HasPrefix(target, "/lib/systemd/system/")
}

const (
	openrcListCommand = `LC_ALL=C rc-service --list 2>/dev/null`

	openrcStatusCommand = `LC_ALL=C rc-status -a 2>/dev/null`

	openrcEnabledCommand = `LC_ALL=C rc-update show 2>/dev/null`

	openrcDescriptionCommand = `LC_ALL=C grep -H -m1 '^description=' /etc/init.d/* 2>/dev/null`

	serviceManagerDetectCommand = `if [ -d /run/systemd/system ] && command -v systemctl >/dev/null 2>&1; then printf 'manager=systemd\n'; elif command -v rc-service >/dev/null 2>&1; then printf 'manager=openrc\n'; else printf 'manager=none\n'; fi`
)

var openrcNamePattern = regexp.MustCompile(`^[A-Za-z0-9@._:][A-Za-z0-9@._:\-]{0,63}$`)

func splitServiceScope(unit string) (scope, name string) {
	if rest, found := strings.CutPrefix(unit, serviceUserScopePrefix); found {
		return "user", rest
	}
	return "", unit
}

func systemctlScoped(scope string) string {
	if scope == "user" {
		return `XDG_RUNTIME_DIR=${XDG_RUNTIME_DIR:-/run/user/$(id -u)} systemctl --user`
	}
	return "systemctl"
}

func (c *Connection) serviceManager(ctx context.Context) string {
	output, err := c.runProbe(ctx, serviceManagerDetectCommand, 4096)
	if err != nil {
		return ""
	}
	value, found := strings.CutPrefix(strings.TrimSpace(output), "manager=")
	if !found {
		return ""
	}
	return strings.TrimSpace(value)
}

func (c *Connection) readOpenRCServices(ctx context.Context) (model.ServiceSnapshot, error) {
	namesOutput, err := c.runProbe(ctx, openrcListCommand, serviceOutputLimit)
	if err != nil {
		return model.ServiceSnapshot{}, err
	}
	names := parseOpenRCServiceNames(namesOutput)
	if len(names) == 0 {

		return model.ServiceSnapshot{}, errors.New("tunnel: openrc returned no services")
	}

	statusOutput, _ := c.runProbe(ctx, openrcStatusCommand, serviceOutputLimit)
	enabledOutput, _ := c.runProbe(ctx, openrcEnabledCommand, serviceOutputLimit)
	descriptionOutput, _ := c.runProbe(ctx, openrcDescriptionCommand, serviceOutputLimit)

	states := parseOpenRCStatus(statusOutput)
	enabled := parseOpenRCEnabled(enabledOutput)
	descriptions := parseOpenRCDescriptions(descriptionOutput)

	services := make([]model.ServiceInfo, 0, len(names))
	for _, name := range names {
		state := states[name]
		enabledState := "disabled"
		if enabled[name] {
			enabledState = "enabled"
		}
		services = append(services, model.ServiceInfo{
			Name:         name,
			Description:  descriptions[name],
			LoadState:    "loaded",
			ActiveState:  openrcActiveState(state),
			SubState:     state,
			EnabledState: enabledState,
		})
	}
	sort.SliceStable(services, func(i, j int) bool {
		return serviceLess(services[i], services[j])
	})
	return model.ServiceSnapshot{Manager: "openrc", Services: services, UpdatedAt: time.Now().UnixMilli()}, nil
}

func openrcActiveState(state string) string {
	switch state {
	case "started", "starting":
		return "active"
	case "crashed":
		return "failed"
	default:
		return "inactive"
	}
}

func parseOpenRCServiceNames(output string) []string {
	names := make([]string, 0, 64)
	for _, line := range strings.Split(output, "\n") {
		name := strings.TrimSpace(line)
		if name == "" || !openrcNamePattern.MatchString(name) {
			continue
		}
		names = append(names, name)
	}
	return names
}

func parseOpenRCStatus(output string) map[string]string {
	states := make(map[string]string, 64)
	for _, line := range strings.Split(output, "\n") {
		open := strings.LastIndex(line, "[")
		close := strings.LastIndex(line, "]")
		if open < 0 || close <= open {
			continue
		}
		name := strings.TrimSpace(line[:open])

		if name == "" || strings.ContainsAny(name, " \t:") {
			continue
		}
		if !openrcNamePattern.MatchString(name) {
			continue
		}
		states[name] = strings.TrimSpace(line[open+1 : close])
	}
	return states
}

func parseOpenRCEnabled(output string) map[string]bool {
	enabled := make(map[string]bool, 64)
	for _, line := range strings.Split(output, "\n") {
		name, runlevels, found := strings.Cut(line, "|")
		if !found {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "" || !openrcNamePattern.MatchString(name) {
			continue
		}
		if strings.TrimSpace(runlevels) != "" {
			enabled[name] = true
		}
	}
	return enabled
}

func parseOpenRCDescriptions(output string) map[string]string {
	descriptions := make(map[string]string, 64)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pathPart, value, found := strings.Cut(line, ":description=")
		if !found {
			continue
		}
		name := path.Base(strings.TrimSpace(pathPart))
		if name == "" || !openrcNamePattern.MatchString(name) {
			continue
		}

		descriptions[name] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	return descriptions
}

func (c *Connection) runOpenRCAction(ctx context.Context, unit string, action ServiceAction) error {
	var command string
	switch action {
	case ServiceStart:
		command = "rc-service " + quoteServiceUnit(unit) + " start"
	case ServiceStop:
		command = "rc-service " + quoteServiceUnit(unit) + " stop"
	case ServiceRestart:
		command = "rc-service " + quoteServiceUnit(unit) + " restart"
	case ServiceEnable:
		command = "rc-update add " + quoteServiceUnit(unit)
	case ServiceDisable, ServiceDisableNow:
		command = "rc-update del " + quoteServiceUnit(unit)
	default:
		return errors.New("tunnel: unsupported service action")
	}

	if action == ServiceDisableNow {
		_, stopErr := c.runProbe(ctx, "rc-service "+quoteServiceUnit(unit)+" stop 2>&1", serviceOutputLimit)
		_, delErr := c.runProbe(ctx, "rc-update del "+quoteServiceUnit(unit)+" 2>&1", serviceOutputLimit)
		if stopErr != nil {
			return stopErr
		}
		return delErr
	}
	_, err := c.runProbe(ctx, command+" 2>&1", serviceOutputLimit)
	return err
}

func (c *Connection) readOpenRCDetail(ctx context.Context, unit string) (model.ServiceDetail, error) {
	statusOutput, stderr, err := c.runProbeWithStderr(ctx, "LC_ALL=C rc-service "+quoteServiceUnit(unit)+" status 2>&1", serviceOutputLimit)
	if err != nil && strings.Contains(strings.ToLower(stderr+statusOutput), "does not exist") {
		return model.ServiceDetail{}, &ServiceProbeError{Kind: ServiceProbeUnitNotFound}
	}
	state := parseOpenRCStatusLine(statusOutput)
	if state == "" && err != nil {
		return model.ServiceDetail{}, err
	}
	fields := map[string]string{
		"Id":           unit,
		"LoadState":    "loaded",
		"ActiveState":  openrcActiveState(state),
		"SubState":     state,
		"FragmentPath": "/etc/init.d/" + unit,
	}
	if enabledOutput, enabledErr := c.runProbe(ctx, openrcEnabledCommand, serviceOutputLimit); enabledErr == nil {
		if parseOpenRCEnabled(enabledOutput)[unit] {
			fields["UnitFileState"] = "enabled"
		} else {
			fields["UnitFileState"] = "disabled"
		}
	}
	if descriptionOutput, descriptionErr := c.runProbe(ctx, openrcDescriptionCommand, serviceOutputLimit); descriptionErr == nil {
		if description := parseOpenRCDescriptions(descriptionOutput)[unit]; description != "" {
			fields["Description"] = description
		}
	}
	return model.ServiceDetail{Name: unit, Fields: fields}, nil
}

func parseOpenRCStatusLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if value, found := strings.CutPrefix(trimmed, "* status:"); found {
			return strings.TrimSpace(value)
		}
		if value, found := strings.CutPrefix(trimmed, "status:"); found {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
