package docker

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"

	"changeme/internal/model"
)

var errInvalidOutput = errors.New("invalid docker command output")

type dockerVersionInfo struct {
	clientVersion    string
	clientAPIVersion string
	serverVersion    string
	serverAPIVersion string
}

type versionPayload struct {
	Client struct {
		Version    string `json:"Version"`
		APIVersion string `json:"ApiVersion"`
	} `json:"Client"`
	Server struct {
		Version    string `json:"Version"`
		APIVersion string `json:"ApiVersion"`
	} `json:"Server"`
}

func parseDockerVersion(data []byte) (dockerVersionInfo, error) {
	var payload versionPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return dockerVersionInfo{}, errInvalidOutput
	}
	info := dockerVersionInfo{
		clientVersion:    strings.TrimSpace(payload.Client.Version),
		clientAPIVersion: strings.TrimSpace(payload.Client.APIVersion),
		serverVersion:    strings.TrimSpace(payload.Server.Version),
		serverAPIVersion: strings.TrimSpace(payload.Server.APIVersion),
	}
	if info.clientVersion == "" && info.serverVersion == "" {
		return dockerVersionInfo{}, errInvalidOutput
	}
	return info, nil
}

func parseContainerList(data []byte) ([]model.DockerContainer, int) {
	containers := make([]model.DockerContainer, 0, 16)
	skipped := 0
	for _, line := range splitDockerLines(data) {
		if line == "" {

			continue
		}
		fields := strings.SplitN(line, "\t", containerListColumns)
		if len(fields) != containerListColumns {
			skipped++
			continue
		}
		containers = append(containers, model.DockerContainer{
			ID:         fields[0],
			Name:       fields[1],
			Image:      fields[2],
			State:      fields[3],
			Status:     fields[4],
			Ports:      fields[5],
			CreatedAt:  fields[6],
			Project:    fields[7],
			Service:    fields[8],
			ProjectDir: fields[9],
		})
	}
	return containers, skipped
}

type inspectContainer struct {
	ID      string `json:"Id"`
	Name    string `json:"Name"`
	Created string `json:"Created"`
	Config  struct {
		Image      string            `json:"Image"`
		Entrypoint []string          `json:"Entrypoint"`
		Cmd        []string          `json:"Cmd"`
		Env        []string          `json:"Env"`
		Labels     map[string]string `json:"Labels"`
	} `json:"Config"`
	State struct {
		Status string `json:"Status"`
	} `json:"State"`
	NetworkSettings struct {
		Ports map[string][]portBinding `json:"Ports"`
	} `json:"NetworkSettings"`

	HostConfig struct {
		PortBindings map[string][]portBinding `json:"PortBindings"`
		Binds        []string                 `json:"Binds"`
	} `json:"HostConfig"`
	Mounts []struct {
		Type        string `json:"Type"`
		Name        string `json:"Name"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
		RW          bool   `json:"RW"`
	} `json:"Mounts"`
}

func parseContainerLabels(data []byte) (map[string]string, error) {
	for _, line := range splitDockerLines(data) {
		if line == "" {
			continue
		}
		labels := map[string]string{}
		if err := json.Unmarshal([]byte(line), &labels); err != nil {
			return nil, errInvalidOutput
		}
		return labels, nil
	}
	return map[string]string{}, nil
}

func parseContainerDetail(data []byte) (model.DockerContainerDetail, error) {
	var items []inspectContainer
	if err := json.Unmarshal(data, &items); err != nil {
		return model.DockerContainerDetail{}, errInvalidOutput
	}
	if len(items) == 0 {
		return model.DockerContainerDetail{}, errInvalidOutput
	}
	item := items[0]
	ports := parsePortMappings(item.NetworkSettings.Ports)
	if len(ports) == 0 {

		ports = parsePortMappings(item.HostConfig.PortBindings)
	}
	mounts := parseMounts(item.Mounts)
	if len(mounts) == 0 {

		mounts = parseBinds(item.HostConfig.Binds)
	}
	return model.DockerContainerDetail{
		ID:    item.ID,
		Name:  strings.TrimPrefix(item.Name, "/"),
		Image: item.Config.Image,

		Entrypoint:  strings.Join(item.Config.Entrypoint, " "),
		Command:     strings.Join(item.Config.Cmd, " "),
		Created:     item.Created,
		State:       item.State.Status,
		Status:      item.State.Status,
		Ports:       ports,
		Mounts:      mounts,
		Environment: cloneStrings(item.Config.Env),
		Labels:      parseLabels(item.Config.Labels),
	}, nil
}

type portBinding struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

func parsePortMappings(ports map[string][]portBinding) []model.DockerPortMapping {
	mappings := make([]model.DockerPortMapping, 0, len(ports))
	for key, bindings := range ports {
		privatePort, protocol := splitPortKey(key)
		if len(bindings) == 0 {
			mappings = append(mappings, model.DockerPortMapping{PrivatePort: privatePort, Type: protocol})
			continue
		}
		for _, binding := range bindings {
			publicPort, _ := strconv.Atoi(strings.TrimSpace(binding.HostPort))
			mappings = append(mappings, model.DockerPortMapping{
				PrivatePort: privatePort,
				PublicPort:  publicPort,
				Type:        protocol,
			})
		}
	}
	sort.SliceStable(mappings, func(i, j int) bool {
		if mappings[i].Type != mappings[j].Type {
			return mappings[i].Type < mappings[j].Type
		}
		if mappings[i].PrivatePort != mappings[j].PrivatePort {
			return mappings[i].PrivatePort < mappings[j].PrivatePort
		}
		return mappings[i].PublicPort < mappings[j].PublicPort
	})
	return mappings
}

func splitPortKey(key string) (int, string) {
	parts := strings.SplitN(key, "/", 2)
	port, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	protocol := ""
	if len(parts) > 1 {
		protocol = strings.TrimSpace(parts[1])
	}
	return port, protocol
}

func parseBinds(binds []string) []model.DockerMount {
	mounts := make([]model.DockerMount, 0, len(binds))
	for _, entry := range binds {
		parts := strings.Split(entry, ":")
		if len(parts) < 2 {
			continue
		}
		mount := model.DockerMount{Type: "bind", Source: parts[0], Destination: parts[1]}
		if len(parts) >= 3 {
			mount.ReadOnly = strings.Contains(parts[2], "ro")
		}
		mounts = append(mounts, mount)
	}
	return mounts
}

func parseMounts(mounts []struct {
	Type        string `json:"Type"`
	Name        string `json:"Name"`
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
	RW          bool   `json:"RW"`
}) []model.DockerMount {
	result := make([]model.DockerMount, 0, len(mounts))
	for _, mount := range mounts {
		result = append(result, model.DockerMount{
			Type:        mount.Type,
			Name:        mount.Name,
			Source:      mount.Source,
			Destination: mount.Destination,

			ReadOnly: !mount.RW,
		})
	}
	return result
}

func parseLabels(labels map[string]string) []model.DockerLabel {
	result := make([]model.DockerLabel, 0, len(labels))
	for key, value := range labels {
		result = append(result, model.DockerLabel{Key: key, Value: value})
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})
	return result
}

func cloneStrings(values []string) []string {
	result := make([]string, len(values))
	copy(result, values)
	return result
}

func splitDockerLines(data []byte) []string {
	text := strings.TrimRight(string(data), "\n")
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		lines[index] = strings.TrimSuffix(line, "\r")
	}
	return lines
}

func parseNDJSONObjects[T any](data []byte) ([]T, int) {
	objects := make([]T, 0, 16)
	skipped := 0
	for _, line := range splitDockerLines(data) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var value T
		if err := json.Unmarshal([]byte(line), &value); err != nil {
			skipped++
			continue
		}
		objects = append(objects, value)
	}
	return objects, skipped
}

type imageListRow struct {
	ID         string `json:"ID"`
	Repository string `json:"Repository"`
	Tag        string `json:"Tag"`
	Digest     string `json:"Digest"`
	Size       string `json:"Size"`
	CreatedAt  string `json:"CreatedAt"`
}

func parseImageList(data []byte) ([]model.DockerImage, int) {
	rows, skipped := parseNDJSONObjects[imageListRow](data)
	images := make([]model.DockerImage, 0, len(rows))
	for _, row := range rows {
		id := strings.TrimSpace(row.ID)
		if id == "" {
			skipped++
			continue
		}
		images = append(images, model.DockerImage{
			ID:         id,
			Repository: row.Repository,
			Tag:        row.Tag,
			Digest:     row.Digest,
			Size:       row.Size,
			CreatedAt:  row.CreatedAt,
		})
	}
	return images, skipped
}

type volumeListRow struct {
	Name       string `json:"Name"`
	Driver     string `json:"Driver"`
	Scope      string `json:"Scope"`
	Mountpoint string `json:"Mountpoint"`
}

func parseVolumeList(data []byte) ([]model.DockerVolume, int) {
	rows, skipped := parseNDJSONObjects[volumeListRow](data)
	volumes := make([]model.DockerVolume, 0, len(rows))
	for _, row := range rows {
		name := strings.TrimSpace(row.Name)
		if name == "" {
			skipped++
			continue
		}
		volumes = append(volumes, model.DockerVolume{
			Name:       name,
			Driver:     row.Driver,
			Scope:      row.Scope,
			Mountpoint: row.Mountpoint,
		})
	}
	return volumes, skipped
}

type networkListRow struct {
	ID     string `json:"ID"`
	Name   string `json:"Name"`
	Driver string `json:"Driver"`
	Scope  string `json:"Scope"`
}

func parseNetworkList(data []byte) ([]model.DockerNetwork, int) {
	rows, skipped := parseNDJSONObjects[networkListRow](data)
	networks := make([]model.DockerNetwork, 0, len(rows))
	for _, row := range rows {
		name := row.Name
		if name == "" {
			skipped++
			continue
		}
		networks = append(networks, model.DockerNetwork{
			ID:     row.ID,
			Name:   name,
			Driver: row.Driver,
			Scope:  row.Scope,
		})
	}
	return networks, skipped
}

func parseNetworkInUse(data []byte) map[string]struct{} {
	names := make(map[string]struct{}, 8)
	for _, line := range splitDockerLines(data) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var networks map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &networks); err != nil {
			continue
		}
		for name := range networks {
			names[name] = struct{}{}
		}
	}
	return names
}

func parseContainerIDs(data []byte) []string {
	ids := make([]string, 0, 16)
	for _, line := range splitDockerLines(data) {
		id := strings.TrimSpace(line)
		if id == "" || !containerIDRegexp.MatchString(id) {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

var inUseSeparators = []string{`\t`, "\t"}

func splitInUseLine(line string) (imageRef string, mountsJSON string, ok bool) {
	index, width := -1, 0
	for _, separator := range inUseSeparators {
		found := strings.Index(line, separator)
		if found < 0 {
			continue
		}

		if index < 0 || found < index || (found == index && len(separator) > width) {
			index, width = found, len(separator)
		}
	}
	if index <= 0 {
		return "", "", false
	}
	imageRef = strings.TrimSpace(line[:index])
	mountsJSON = strings.TrimSpace(line[index+width:])
	if imageRef == "" || mountsJSON == "" {
		return "", "", false
	}
	return imageRef, mountsJSON, true
}

func normalizeImageID(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "sha256:")
}

func parseInUseIndex(data []byte) (map[string]struct{}, map[string]struct{}) {
	imageIDs := make(map[string]struct{}, 16)
	volumes := make(map[string]struct{}, 16)
	for _, line := range splitDockerLines(data) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		imageRef, mountsJSON, ok := splitInUseLine(line)
		if !ok {
			continue
		}
		if id := normalizeImageID(imageRef); id != "" {
			imageIDs[id] = struct{}{}
		}
		var mounts []struct {
			Type string `json:"Type"`
			Name string `json:"Name"`
		}
		if err := json.Unmarshal([]byte(mountsJSON), &mounts); err != nil {
			continue
		}
		for _, mount := range mounts {

			if mount.Type == "volume" && mount.Name != "" {
				volumes[mount.Name] = struct{}{}
			}
		}
	}
	return imageIDs, volumes
}

type inspectImage struct {
	ID           string   `json:"Id"`
	RepoTags     []string `json:"RepoTags"`
	RepoDigests  []string `json:"RepoDigests"`
	Created      string   `json:"Created"`
	Size         int64    `json:"Size"`
	Architecture string   `json:"Architecture"`
	Os           string   `json:"Os"`
	Author       string   `json:"Author"`
	Config       struct {
		Env          []string            `json:"Env"`
		Entrypoint   []string            `json:"Entrypoint"`
		Cmd          []string            `json:"Cmd"`
		Labels       map[string]string   `json:"Labels"`
		ExposedPorts map[string]struct{} `json:"ExposedPorts"`
	} `json:"Config"`
	RootFS struct {
		Layers []string `json:"Layers"`
	} `json:"RootFS"`
}

func parseImageDetail(data []byte) (model.DockerImageDetail, error) {
	var items []inspectImage
	if err := json.Unmarshal(data, &items); err != nil {
		return model.DockerImageDetail{}, errInvalidOutput
	}
	if len(items) == 0 {
		return model.DockerImageDetail{}, errInvalidOutput
	}
	item := items[0]
	detail := model.DockerImageDetail{
		ID:           item.ID,
		RepoTags:     cloneStrings(item.RepoTags),
		RepoDigests:  cloneStrings(item.RepoDigests),
		Created:      item.Created,
		SizeBytes:    item.Size,
		Size:         humanImageSize(item.Size),
		Architecture: item.Architecture,
		OS:           item.Os,
		Author:       item.Author,
		Entrypoint:   strings.Join(item.Config.Entrypoint, " "),
		Command:      strings.Join(item.Config.Cmd, " "),
		Environment:  cloneStrings(item.Config.Env),
		Labels:       parseLabels(item.Config.Labels),
		Layers:       len(item.RootFS.Layers),
	}
	if len(detail.RepoTags) > 0 {
		detail.Reference = detail.RepoTags[0]
	}

	for _, digest := range detail.RepoDigests {
		if index := strings.LastIndex(digest, "@"); index >= 0 && index+1 < len(digest) {
			detail.Digest = digest[index+1:]
			break
		}
	}
	detail.ExposedPorts = sortedMapKeys(item.Config.ExposedPorts)
	return detail, nil
}

func humanImageSize(size int64) string {
	if size <= 0 {
		return "0B"
	}
	units := []string{"B", "kB", "MB", "GB", "TB"}
	value := float64(size)
	unit := 0
	for value >= 1000 && unit < len(units)-1 {
		value /= 1000
		unit++
	}
	if unit == 0 {
		return strconv.FormatInt(size, 10) + "B"
	}

	formatted := strconv.FormatFloat(value, 'g', 3, 64)
	return formatted + units[unit]
}

func sortedMapKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

type inspectVolume struct {
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	Mountpoint string            `json:"Mountpoint"`
	CreatedAt  string            `json:"CreatedAt"`
	Scope      string            `json:"Scope"`
	Labels     map[string]string `json:"Labels"`
	Options    map[string]string `json:"Options"`
}

func parseVolumeDetail(data []byte) (model.DockerVolumeDetail, error) {
	var items []inspectVolume
	if err := json.Unmarshal(data, &items); err != nil {
		return model.DockerVolumeDetail{}, errInvalidOutput
	}
	if len(items) == 0 {
		return model.DockerVolumeDetail{}, errInvalidOutput
	}
	item := items[0]
	return model.DockerVolumeDetail{
		Name:       item.Name,
		Driver:     item.Driver,
		Scope:      item.Scope,
		Mountpoint: item.Mountpoint,
		CreatedAt:  item.CreatedAt,
		Labels:     parseLabels(item.Labels),
		Options:    parseLabels(item.Options),
	}, nil
}

type inspectNetwork struct {
	ID         string            `json:"Id"`
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	Scope      string            `json:"Scope"`
	Created    string            `json:"Created"`
	Internal   bool              `json:"Internal"`
	Attachable bool              `json:"Attachable"`
	Ingress    bool              `json:"Ingress"`
	EnableIPv6 bool              `json:"EnableIPv6"`
	Labels     map[string]string `json:"Labels"`
	Options    map[string]string `json:"Options"`
	IPAM       struct {
		Config []struct {
			Subnet  string `json:"Subnet"`
			Gateway string `json:"Gateway"`
		} `json:"Config"`
	} `json:"IPAM"`
	Containers map[string]struct {
		Name        string `json:"Name"`
		IPv4Address string `json:"IPv4Address"`
		IPv6Address string `json:"IPv6Address"`
		MacAddress  string `json:"MacAddress"`
	} `json:"Containers"`
}

func parseNetworkDetail(data []byte) (model.DockerNetworkDetail, error) {
	var items []inspectNetwork
	if err := json.Unmarshal(data, &items); err != nil {
		return model.DockerNetworkDetail{}, errInvalidOutput
	}
	if len(items) == 0 {
		return model.DockerNetworkDetail{}, errInvalidOutput
	}
	item := items[0]
	subnets := make([]model.DockerSubnet, 0, len(item.IPAM.Config))
	for _, config := range item.IPAM.Config {
		subnets = append(subnets, model.DockerSubnet{Subnet: config.Subnet, Gateway: config.Gateway})
	}
	sort.SliceStable(subnets, func(i, j int) bool { return subnets[i].Subnet < subnets[j].Subnet })
	members := make([]model.DockerNetworkMember, 0, len(item.Containers))
	for _, container := range item.Containers {
		members = append(members, model.DockerNetworkMember{
			Name: strings.TrimPrefix(container.Name, "/"),
			IPv4: container.IPv4Address,
			IPv6: container.IPv6Address,
			Mac:  container.MacAddress,
		})
	}
	sort.SliceStable(members, func(i, j int) bool { return members[i].Name < members[j].Name })
	return model.DockerNetworkDetail{
		ID:         item.ID,
		Name:       item.Name,
		Driver:     item.Driver,
		Scope:      item.Scope,
		Created:    item.Created,
		Internal:   item.Internal,
		Attachable: item.Attachable,
		Ingress:    item.Ingress,
		EnableIPv6: item.EnableIPv6,
		Subnets:    subnets,
		Containers: members,
		Labels:     parseLabels(item.Labels),
		Options:    parseLabels(item.Options),
	}, nil
}
