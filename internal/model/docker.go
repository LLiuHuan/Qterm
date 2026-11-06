package model

type DockerEnvironment struct {
	Available bool `json:"available"`

	DockerVersion string `json:"dockerVersion"`

	APIVersion string `json:"apiVersion"`

	ComposeVersion string `json:"composeVersion"`

	Message string `json:"message"`
}

type DockerContainer struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image"`

	State string `json:"state"`

	Status string `json:"status"`

	Ports string `json:"ports"`

	Project string `json:"project"`

	ProjectDir string `json:"projectDir"`

	Service string `json:"service"`

	CreatedAt string `json:"createdAt"`
}

type DockerPortMapping struct {
	PrivatePort int `json:"privatePort"`

	PublicPort int `json:"publicPort"`

	Type string `json:"type"`
}

type DockerMount struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	ReadOnly    bool   `json:"readOnly"`
}

type DockerLabel struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type DockerImage struct {
	ID string `json:"id"`

	Repository string `json:"repository"`

	Tag string `json:"tag"`

	Digest string `json:"digest"`

	Size string `json:"size"`

	CreatedAt string `json:"createdAt"`

	InUse bool `json:"inUse"`
}

type DockerVolume struct {
	Name string `json:"name"`

	Driver string `json:"driver"`

	Scope string `json:"scope"`

	Mountpoint string `json:"mountpoint"`

	InUse bool `json:"inUse"`
}

type DockerNetwork struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Driver string `json:"driver"`
	Scope  string `json:"scope"`

	InUse bool `json:"inUse"`

	BuiltIn bool `json:"builtIn"`
}

type DockerContainerDetail struct {
	ID string `json:"id"`

	Name string `json:"name"`

	Image string `json:"image"`

	Entrypoint string `json:"entrypoint"`

	Command string `json:"command"`

	Created string `json:"created"`

	State string `json:"state"`

	Status string `json:"status"`

	Ports       []DockerPortMapping `json:"ports"`
	Mounts      []DockerMount       `json:"mounts"`
	Environment []string            `json:"environment"`
	Labels      []DockerLabel       `json:"labels"`
}

type DockerImageDetail struct {
	ID string `json:"id"`

	Reference string `json:"reference"`

	RepoTags    []string `json:"repoTags"`
	RepoDigests []string `json:"repoDigests"`

	Digest string `json:"digest"`

	Created string `json:"created"`

	SizeBytes int64  `json:"sizeBytes"`
	Size      string `json:"size"`

	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Author       string `json:"author"`

	Entrypoint string `json:"entrypoint"`
	Command    string `json:"command"`

	ExposedPorts []string `json:"exposedPorts"`

	Environment []string `json:"environment"`

	Labels []DockerLabel `json:"labels"`

	Layers int `json:"layers"`
}

type DockerVolumeDetail struct {
	Name string `json:"name"`

	Driver string `json:"driver"`
	Scope  string `json:"scope"`

	Mountpoint string `json:"mountpoint"`

	CreatedAt string `json:"createdAt"`

	Labels  []DockerLabel `json:"labels"`
	Options []DockerLabel `json:"options"`
}

type DockerSubnet struct {
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
}

type DockerNetworkMember struct {
	Name string `json:"name"`
	IPv4 string `json:"ipv4"`
	IPv6 string `json:"ipv6"`
	Mac  string `json:"mac"`
}

type DockerNetworkDetail struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Driver  string `json:"driver"`
	Scope   string `json:"scope"`
	Created string `json:"created"`

	Internal   bool `json:"internal"`
	Attachable bool `json:"attachable"`
	Ingress    bool `json:"ingress"`
	EnableIPv6 bool `json:"enableIpv6"`

	Subnets []DockerSubnet `json:"subnets"`

	Containers []DockerNetworkMember `json:"containers"`

	Labels  []DockerLabel `json:"labels"`
	Options []DockerLabel `json:"options"`
}
