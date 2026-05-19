package template

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed templates/*.yaml
var embeddedFS embed.FS

type ResourcesDef struct {
	CPURequest    string `yaml:"cpu_request"    json:"cpu_request"`
	CPULimit      string `yaml:"cpu_limit"      json:"cpu_limit"`
	MemoryRequest string `yaml:"memory_request" json:"memory_request"`
	MemoryLimit   string `yaml:"memory_limit"   json:"memory_limit"`
}

type PortDef struct {
	Port     int    `yaml:"port"     json:"port"`
	Protocol string `yaml:"protocol" json:"protocol"`
}

type EnvVarDef struct {
	Key          string `yaml:"key"           json:"key"`
	Required     bool   `yaml:"required"      json:"required"`
	Description  string `yaml:"description"   json:"description"`
	AutoGenerate bool   `yaml:"auto_generate" json:"auto_generate"`
	GenerateType string `yaml:"generate_type" json:"generate_type"`
	Default      string `yaml:"default"       json:"default"`
}

type VolumeDef struct {
	Required      bool   `yaml:"required"        json:"required"`
	MountPath     string `yaml:"mount_path"      json:"mount_path"`
	DefaultSizeMB int    `yaml:"default_size_mb" json:"default_size_mb"`
}

type IngressDef struct {
	Enabled  bool `yaml:"enabled"   json:"enabled"`
	HttpPort int  `yaml:"http_port" json:"http_port"`
}

type ContainerDef struct {
	Image     string       `yaml:"image"     json:"image"`
	Tag       string       `yaml:"tag"       json:"tag"`
	Replicas  int          `yaml:"replicas"  json:"replicas"`
	Resources ResourcesDef `yaml:"resources" json:"resources"`
	Ports     []PortDef    `yaml:"ports"     json:"ports"`
	Command   string       `yaml:"command"   json:"command"`
	Args      string       `yaml:"args"      json:"args"`
}

type Template struct {
	Name        string       `yaml:"name"         json:"name"`
	DisplayName string       `yaml:"display_name" json:"display_name"`
	Category    string       `yaml:"category"     json:"category"`
	Description string       `yaml:"description"  json:"description"`
	Icon        string       `yaml:"icon"         json:"icon"`
	Color       string       `yaml:"color"        json:"color"`
	Version     string       `yaml:"version"      json:"version"`
	Container   ContainerDef `yaml:"container"    json:"container"`
	EnvVars     []EnvVarDef  `yaml:"env_vars"     json:"env_vars"`
	Volume      *VolumeDef   `yaml:"volume"       json:"volume"`
	Ingress     *IngressDef  `yaml:"ingress"      json:"ingress"`
}

var (
	registry   map[string]*Template
	registryMu sync.RWMutex
)

func LoadAll() error {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = make(map[string]*Template)

	overrideDir := os.Getenv("TEMPLATES_DIR")
	if overrideDir != "" {
		return loadFromDir(overrideDir)
	}
	return loadFromFS(embeddedFS, "templates")
}

func loadFromDir(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".yaml" {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return parseAndStore(data, path)
	})
}

func loadFromFS(fsys embed.FS, dir string) error {
	entries, err := fsys.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("template: read embedded dir %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		data, err := fsys.ReadFile(dir + "/" + e.Name())
		if err != nil {
			return err
		}
		if err := parseAndStore(data, e.Name()); err != nil {
			return err
		}
	}
	return nil
}

func parseAndStore(data []byte, name string) error {
	var t Template
	if err := yaml.Unmarshal(data, &t); err != nil {
		return fmt.Errorf("template: parse %s: %w", name, err)
	}
	registry[t.Name] = &t
	return nil
}

func List() []*Template {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]*Template, 0, len(registry))
	for _, t := range registry {
		out = append(out, t)
	}
	return out
}

func Get(name string) (*Template, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	t, ok := registry[name]
	return t, ok
}
