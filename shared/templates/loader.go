package templates

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"sync"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

//go:embed *.yaml
var yamlFiles embed.FS

// EnvVarDef defines a single user-configurable environment variable.
type EnvVarDef struct {
	Key         string `yaml:"key"         json:"key"`
	Description string `yaml:"description" json:"description"`
	Default     string `yaml:"default"     json:"default"`
	Required    bool   `yaml:"required"    json:"required"`
	// Generate specifies auto-generation strategy when the value is empty.
	// Supported: "password" (32-char hex), "uuid"
	Generate string `yaml:"generate"    json:"generate,omitempty"`
}

// ResourceLimit defines CPU/Memory for requests and limits independently.
type ResourceLimit struct {
	CPU    string `yaml:"cpu"    json:"cpu"`
	Memory string `yaml:"memory" json:"memory"`
}

// ResourcesSpec defines the K8s resource requirements for the container.
type ResourcesSpec struct {
	Requests ResourceLimit `yaml:"requests" json:"requests"`
	Limits   ResourceLimit `yaml:"limits"   json:"limits"`
}

// VolumeSpec defines the persistent volume attached to a template container.
type VolumeSpec struct {
	Enabled   bool   `yaml:"enabled"    json:"enabled"`
	SizeMB    int    `yaml:"size_mb"    json:"size_mb"`
	MountPath string `yaml:"mount_path" json:"mount_path"`
}

// ServicePortDef defines a single port exposed by the K8s Service.
type ServicePortDef struct {
	Name     string `yaml:"name"     json:"name"`
	Port     int    `yaml:"port"     json:"port"`
	Protocol string `yaml:"protocol" json:"protocol"`
}

// ServiceSpec defines the K8s Service configuration for this template.
type ServiceSpec struct {
	Type  string           `yaml:"type"  json:"type"`
	Ports []ServicePortDef `yaml:"ports" json:"ports"`
}

// Template is the parsed representation of a single template YAML file.
type Template struct {
	ID          string        `yaml:"id"           json:"id"`
	Label       string        `yaml:"label"        json:"label"`
	Version     string        `yaml:"version"      json:"version"`
	Image       string        `yaml:"image"        json:"-"`
	Description string        `yaml:"description"  json:"description"`
	Port        int           `yaml:"port"         json:"port"`
	DefaultName string        `yaml:"default_name" json:"default_name"`
	EnvVars     []EnvVarDef   `yaml:"env_vars"     json:"env_vars"`
	Resources   ResourcesSpec `yaml:"resources"    json:"resources"`
	Volume      VolumeSpec    `yaml:"volume"       json:"volume"`
	Service     ServiceSpec   `yaml:"service"      json:"service"`
}

var (
	once     sync.Once
	registry map[string]Template
	ordered  []Template
)

func load() {
	once.Do(func() {
		registry = make(map[string]Template)
		entries, err := fs.Glob(yamlFiles, "*.yaml")
		if err != nil {
			panic(fmt.Sprintf("templates: failed to glob yaml files: %v", err))
		}
		for _, name := range entries {
			data, err := yamlFiles.ReadFile(name)
			if err != nil {
				panic(fmt.Sprintf("templates: failed to read %s: %v", name, err))
			}
			var t Template
			if err := yaml.Unmarshal(data, &t); err != nil {
				panic(fmt.Sprintf("templates: failed to parse %s: %v", name, err))
			}
			registry[t.ID] = t
			ordered = append(ordered, t)
		}
	})
}

// List returns all templates in filesystem order.
func List() []Template {
	load()
	return ordered
}

// GetByID returns the template with the given ID, or false if not found.
func GetByID(id string) (Template, bool) {
	load()
	t, ok := registry[id]
	return t, ok
}

// ResolvedEnvVarsJSON returns the default+generated env vars as a JSON object string.
// Values marked with generate: are auto-filled when empty.
func ResolvedEnvVarsJSON(id string, userProvided map[string]string) string {
	t, ok := GetByID(id)
	if !ok {
		return "{}"
	}
	m := make(map[string]string, len(t.EnvVars))
	for _, v := range t.EnvVars {
		if val, ok := userProvided[v.Key]; ok && val != "" {
			m[v.Key] = val
			continue
		}
		if v.Default != "" {
			m[v.Key] = v.Default
		}
		if v.Generate != "" {
			m[v.Key] = generateValue(v.Generate)
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// DefaultEnvVarsJSON returns defaults (without resolving generate:) for display.
func DefaultEnvVarsJSON(id string) string {
	t, ok := GetByID(id)
	if !ok {
		return "{}"
	}
	m := make(map[string]string, len(t.EnvVars))
	for _, v := range t.EnvVars {
		if v.Default != "" {
			m[v.Key] = v.Default
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// DefaultPortsJSON returns the service ports for the template as a JSON array string.
func DefaultPortsJSON(id string) string {
	t, ok := GetByID(id)
	if !ok {
		return "[]"
	}
	type portEntry struct {
		Name     string `json:"name"`
		Port     int    `json:"port"`
		Target   int    `json:"target"`
		Protocol string `json:"protocol"`
	}
	entries := make([]portEntry, 0, len(t.Service.Ports))
	for _, p := range t.Service.Ports {
		entries = append(entries, portEntry{
			Name:     p.Name,
			Port:     p.Port,
			Target:   p.Port,
			Protocol: p.Protocol,
		})
	}
	// fall back to top-level port if service.ports is empty
	if len(entries) == 0 && t.Port > 0 {
		entries = append(entries, portEntry{Port: t.Port, Target: t.Port, Protocol: "TCP"})
	}
	b, err := json.Marshal(entries)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// ResourcesJSON returns the resources spec as a JSON string compatible with parseResources.
func ResourcesJSON(id string) string {
	t, ok := GetByID(id)
	if !ok {
		return "{}"
	}
	type res struct {
		CPURequest    string `json:"cpu_request,omitempty"`
		MemoryRequest string `json:"memory_request,omitempty"`
		CPULimit      string `json:"cpu_limit,omitempty"`
		MemoryLimit   string `json:"memory_limit,omitempty"`
	}
	r := res{
		CPURequest:    t.Resources.Requests.CPU,
		MemoryRequest: t.Resources.Requests.Memory,
		CPULimit:      t.Resources.Limits.CPU,
		MemoryLimit:   t.Resources.Limits.Memory,
	}
	b, err := json.Marshal(r)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func generateValue(strategy string) string {
	switch strings.ToLower(strategy) {
	case "password":
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return "changeme"
		}
		return hex.EncodeToString(b)
	case "uuid":
		return uuid.New().String()
	default:
		return ""
	}
}
