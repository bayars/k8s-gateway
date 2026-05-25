package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// DeviceConfig represents a single device configuration
type DeviceConfig struct {
	Hostname    string `yaml:"hostname"`
	SSHPort     int    `yaml:"ssh_port"`
	TelnetPort  int    `yaml:"telnet_port"`
	NetconfPort int    `yaml:"netconf_port"`
	GNMIPort    int    `yaml:"gnmi_port"`
	Description string `yaml:"description"`
	Location    string `yaml:"location"`
}

// Settings represents global gateway settings
type Settings struct {
	DomainSuffix   string `yaml:"domain_suffix"`
	DefaultTimeout int    `yaml:"default_timeout"`
	MaxSessions    int    `yaml:"max_sessions"`
	LogLevel       string `yaml:"log_level"`
}

// Config represents the complete configuration
type Config struct {
	Devices  map[string]DeviceConfig `yaml:"devices"`
	Settings Settings                `yaml:"settings"`
}

// LoadConfig loads configuration from YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// ConfigHolder is a thread-safe, hot-reloadable wrapper around Config.
// All three servers (gRPC, gNMI, SSH) call Get() per request so they
// always see the latest config without a pod restart.
type ConfigHolder struct {
	mu         sync.RWMutex
	current    *Config
	configPath string
	watcher    *fsnotify.Watcher
}

// NewConfigHolder wraps an initial Config with a holder.
func NewConfigHolder(cfg *Config, path string) *ConfigHolder {
	return &ConfigHolder{current: cfg, configPath: path}
}

// Get returns the current Config. Safe for concurrent use.
func (h *ConfigHolder) Get() *Config {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.current
}

// Watch starts an fsnotify watcher on the config file directory and reloads
// devices.yaml when Kubernetes updates the ConfigMap symlink ("..data").
// If the reloaded config is invalid or empty it is discarded and the previous
// config continues to serve traffic.
func (h *ConfigHolder) Watch(log *logrus.Logger) error {
	if h.configPath == "" {
		return nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create config watcher: %w", err)
	}
	h.watcher = watcher

	dir := filepath.Dir(h.configPath)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) != 0 {
					if event.Name == h.configPath ||
						filepath.Base(event.Name) == filepath.Base(h.configPath) ||
						event.Name == dir ||
						strings.Contains(event.Name, "..data") {
						log.Info("Device config changed, reloading...")
						if err := h.reload(log); err != nil {
							log.WithError(err).Error("Failed to reload device config, keeping previous config")
						}
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.WithError(err).Error("Config watcher error")
			}
		}
	}()

	if err := watcher.Add(dir); err != nil {
		return fmt.Errorf("failed to watch config directory %s: %w", dir, err)
	}

	log.Infof("Watching for device config changes in: %s", dir)
	return nil
}

// reload reads and validates the config file, then atomically replaces current.
func (h *ConfigHolder) reload(log *logrus.Logger) error {
	cfg, err := LoadConfig(h.configPath)
	if err != nil {
		return err
	}
	if len(cfg.Devices) == 0 {
		return fmt.Errorf("reloaded config contains no devices, ignoring")
	}
	h.mu.Lock()
	h.current = cfg
	h.mu.Unlock()
	log.Infof("Device config reloaded: %d devices", len(cfg.Devices))
	return nil
}

// Close stops the file watcher.
func (h *ConfigHolder) Close() {
	if h.watcher != nil {
		_ = h.watcher.Close()
	}
}

// GetDeviceByFQDN extracts device name from FQDN and returns its config
func (c *Config) GetDeviceByFQDN(fqdn string) (*DeviceConfig, string, error) {
	// Parse FQDN: router1.myCustomer.safabayar.net -> router1
	parts := strings.Split(fqdn, ".")
	if len(parts) < 1 {
		return nil, "", fmt.Errorf("invalid FQDN format: %s", fqdn)
	}

	deviceName := parts[0]

	device, exists := c.Devices[deviceName]
	if !exists {
		return nil, "", fmt.Errorf("device not found: %s", deviceName)
	}

	return &device, deviceName, nil
}
