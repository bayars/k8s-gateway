package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create temporary config file
	configContent := `
devices:
  testrouter:
    hostname: "192.168.1.1"
    ssh_port: 22
    telnet_port: 23
    netconf_port: 830
    gnmi_port: 57400
    description: "Test Router"
    location: "Test Lab"

settings:
  domain_suffix: "test.com"
  default_timeout: 30
  max_sessions: 50
  log_level: "debug"
`

	tmpFile, err := os.CreateTemp("", "devices-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// Test loading config
	cfg, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify config
	if len(cfg.Devices) != 1 {
		t.Errorf("Expected 1 device, got %d", len(cfg.Devices))
	}

	device, exists := cfg.Devices["testrouter"]
	if !exists {
		t.Fatal("Device 'testrouter' not found")
	}

	if device.Hostname != "192.168.1.1" {
		t.Errorf("Expected hostname '192.168.1.1', got '%s'", device.Hostname)
	}

	if device.SSHPort != 22 {
		t.Errorf("Expected SSH port 22, got %d", device.SSHPort)
	}

	if cfg.Settings.DomainSuffix != "test.com" {
		t.Errorf("Expected domain suffix 'test.com', got '%s'", cfg.Settings.DomainSuffix)
	}

	if cfg.Settings.LogLevel != "debug" {
		t.Errorf("Expected log level 'debug', got '%s'", cfg.Settings.LogLevel)
	}

	if device.GNMIPort != 57400 {
		t.Errorf("Expected gNMI port 57400, got %d", device.GNMIPort)
	}
}

func TestGetDeviceByFQDN(t *testing.T) {
	cfg := &Config{
		Devices: map[string]DeviceConfig{
			"router1": {
				Hostname:    "10.0.1.10",
				SSHPort:     22,
				TelnetPort:  23,
				NetconfPort: 830,
				Description: "Router 1",
			},
			"switch1": {
				Hostname:    "10.0.2.10",
				SSHPort:     22,
				TelnetPort:  23,
				NetconfPort: 830,
				Description: "Switch 1",
			},
		},
		Settings: Settings{
			DomainSuffix: "example.com",
		},
	}

	tests := []struct {
		name       string
		fqdn       string
		wantDevice string
		wantErr    bool
	}{
		{
			name:       "Valid FQDN with subdomain",
			fqdn:       "router1.myCustomer.example.com",
			wantDevice: "router1",
			wantErr:    false,
		},
		{
			name:       "Valid FQDN simple",
			fqdn:       "switch1.example.com",
			wantDevice: "switch1",
			wantErr:    false,
		},
		{
			name:       "Device not found",
			fqdn:       "router999.example.com",
			wantDevice: "",
			wantErr:    true,
		},
		{
			name:       "Invalid FQDN",
			fqdn:       "",
			wantDevice: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			device, deviceName, err := cfg.GetDeviceByFQDN(tt.fqdn)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if deviceName != tt.wantDevice {
				t.Errorf("Expected device name '%s', got '%s'", tt.wantDevice, deviceName)
			}

			if device == nil {
				t.Error("Expected device config, got nil")
			}
		})
	}
}
