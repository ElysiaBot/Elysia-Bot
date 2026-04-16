package pluginsdk

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
}

func NewRegistry() *Registry {
	return &Registry{plugins: make(map[string]Plugin)}
}

func (r *Registry) Register(plugin Plugin) error {
	if err := plugin.Validate(); err != nil {
		return fmt.Errorf("invalid plugin: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[plugin.Manifest.ID]; exists {
		return fmt.Errorf("plugin %q already registered", plugin.Manifest.ID)
	}

	r.plugins[plugin.Manifest.ID] = plugin
	return nil
}

func (r *Registry) Get(id string) (Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugin, exists := r.plugins[id]
	if !exists {
		return Plugin{}, errors.New("plugin not found")
	}

	return plugin, nil
}

func (r *Registry) List() []PluginManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	manifests := make([]PluginManifest, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		manifests = append(manifests, plugin.Manifest)
	}

	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].ID < manifests[j].ID
	})

	return manifests
}
