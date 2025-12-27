package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// DeviceConfig represents a single device configuration
type DeviceConfig struct {
	Hostname    string `yaml:"hostname"`
	SSHPort     int    `yaml:"ssh_port"`
	TelnetPort  int    `yaml:"telnet_port"`
	NetconfPort int    `yaml:"netconf_port"`
	Description string `yaml:"description"`
	Location    string `yaml:"location"`
}

// Settings represents global gateway settings
type Settings struct {
	DomainSuffix  string `yaml:"domain_suffix"`
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
