package config

import (
	"sync"

	"sigs.k8s.io/yaml"
)

// LauncherConfig holds ordered rules for alternative launcher image selection.
type LauncherConfig struct {
	Rules []Rule `json:"rules"`
}

// Rule maps a selector to an alternative launcher image.
type Rule struct {
	Name     string   `json:"name"`
	Image    string   `json:"image"`
	Selector Selector `json:"selector"`
}

// Selector defines the criteria for matching a VMI.
// DeviceNames and VMLabels are OR'd: if either matches, the rule applies.
type Selector struct {
	DeviceNames []string  `json:"deviceNames,omitempty"`
	VMLabels    *VMLabels `json:"vmLabels,omitempty"`
}

// VMLabels matches VMIs by label selectors.
type VMLabels struct {
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

// ConfigStore provides thread-safe access to the current LauncherConfig.
type ConfigStore struct {
	mu     sync.RWMutex
	config *LauncherConfig
}

// NewConfigStore creates a new empty ConfigStore.
func NewConfigStore() *ConfigStore {
	return &ConfigStore{}
}

// Get returns the current config, or nil if none has been loaded.
func (s *ConfigStore) Get() *LauncherConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// Update parses raw YAML and atomically replaces the stored config.
func (s *ConfigStore) Update(data []byte) error {
	var cfg LauncherConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = &cfg
	return nil
}
