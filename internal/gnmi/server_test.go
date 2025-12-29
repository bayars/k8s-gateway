package gnmi

import (
	"context"
	"testing"

	"github.com/safabayar/gateway/internal/config"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/metadata"
)

func TestNewServer(t *testing.T) {
	cfg := &config.Config{
		Devices: map[string]config.DeviceConfig{
			"srl1": {
				Hostname: "10.0.0.1",
				GNMIPort: 57400,
			},
		},
	}

	server := NewServer(cfg)
	if server == nil {
		t.Error("NewServer returned nil")
	}

	if server.config != cfg {
		t.Error("Server config not set correctly")
	}
}

func TestParseTarget(t *testing.T) {
	cfg := &config.Config{
		Devices: map[string]config.DeviceConfig{},
	}
	server := NewServer(cfg)

	tests := []struct {
		name         string
		target       string
		wantFQDN     string
		wantUsername string
		wantPassword string
		wantErr      bool
	}{
		{
			name:         "FQDN only",
			target:       "srl1.safabayar.net",
			wantFQDN:     "srl1.safabayar.net",
			wantUsername: "admin",
			wantPassword: "NokiaSrl1!",
			wantErr:      false,
		},
		{
			name:         "FQDN with username",
			target:       "srl1.safabayar.net:myuser",
			wantFQDN:     "srl1.safabayar.net",
			wantUsername: "myuser",
			wantPassword: "NokiaSrl1!",
			wantErr:      false,
		},
		{
			name:         "FQDN with username and password",
			target:       "srl1.safabayar.net:myuser:mypass",
			wantFQDN:     "srl1.safabayar.net",
			wantUsername: "myuser",
			wantPassword: "mypass",
			wantErr:      false,
		},
		{
			name:         "Empty target",
			target:       "",
			wantFQDN:     "",
			wantUsername: "admin",
			wantPassword: "NokiaSrl1!",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fqdn, username, password, err := server.parseTarget(tt.target)

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

			if fqdn != tt.wantFQDN {
				t.Errorf("FQDN: got %s, want %s", fqdn, tt.wantFQDN)
			}
			if username != tt.wantUsername {
				t.Errorf("Username: got %s, want %s", username, tt.wantUsername)
			}
			if password != tt.wantPassword {
				t.Errorf("Password: got %s, want %s", password, tt.wantPassword)
			}
		})
	}
}

func TestGetTargetFromContext(t *testing.T) {
	cfg := &config.Config{
		Devices: map[string]config.DeviceConfig{},
	}
	server := NewServer(cfg)

	tests := []struct {
		name       string
		setupCtx   func() context.Context
		prefix     *gnmipb.Path
		wantFQDN   string
		wantErr    bool
	}{
		{
			name: "Target from metadata",
			setupCtx: func() context.Context {
				md := metadata.New(map[string]string{
					"x-gnmi-target": "srl1.safabayar.net:admin:pass123",
				})
				return metadata.NewIncomingContext(context.Background(), md)
			},
			prefix:   nil,
			wantFQDN: "srl1.safabayar.net",
			wantErr:  false,
		},
		{
			name: "Target from prefix",
			setupCtx: func() context.Context {
				return context.Background()
			},
			prefix: &gnmipb.Path{
				Target: "srl2.safabayar.net",
			},
			wantFQDN: "srl2.safabayar.net",
			wantErr:  false,
		},
		{
			name: "No target specified",
			setupCtx: func() context.Context {
				return context.Background()
			},
			prefix:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()
			fqdn, _, _, err := server.getTargetFromContext(ctx, tt.prefix)

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

			if fqdn != tt.wantFQDN {
				t.Errorf("FQDN: got %s, want %s", fqdn, tt.wantFQDN)
			}
		})
	}
}

func TestBasicAuth(t *testing.T) {
	auth := &basicAuth{
		username: "testuser",
		password: "testpass",
		insecure: false,
	}

	// Test GetRequestMetadata
	meta, err := auth.GetRequestMetadata(context.Background())
	if err != nil {
		t.Errorf("GetRequestMetadata error: %v", err)
	}

	if meta["username"] != "testuser" {
		t.Errorf("Username: got %s, want testuser", meta["username"])
	}
	if meta["password"] != "testpass" {
		t.Errorf("Password: got %s, want testpass", meta["password"])
	}

	// Test RequireTransportSecurity
	if auth.RequireTransportSecurity() != true {
		t.Error("RequireTransportSecurity should be true when insecure is false")
	}

	auth.insecure = true
	if auth.RequireTransportSecurity() != false {
		t.Error("RequireTransportSecurity should be false when insecure is true")
	}
}

func TestCapabilities_NoTarget(t *testing.T) {
	cfg := &config.Config{
		Devices: map[string]config.DeviceConfig{
			"srl1": {
				Hostname: "10.0.0.1",
				GNMIPort: 57400,
			},
		},
	}

	server := NewServer(cfg)
	ctx := context.Background()

	_, err := server.Capabilities(ctx, &gnmipb.CapabilityRequest{})
	if err == nil {
		t.Error("Expected error when no target specified")
	}
}

func TestGet_NoTarget(t *testing.T) {
	cfg := &config.Config{
		Devices: map[string]config.DeviceConfig{
			"srl1": {
				Hostname: "10.0.0.1",
				GNMIPort: 57400,
			},
		},
	}

	server := NewServer(cfg)
	ctx := context.Background()

	_, err := server.Get(ctx, &gnmipb.GetRequest{})
	if err == nil {
		t.Error("Expected error when no target specified")
	}
}

func TestSet_NoTarget(t *testing.T) {
	cfg := &config.Config{
		Devices: map[string]config.DeviceConfig{
			"srl1": {
				Hostname: "10.0.0.1",
				GNMIPort: 57400,
			},
		},
	}

	server := NewServer(cfg)
	ctx := context.Background()

	_, err := server.Set(ctx, &gnmipb.SetRequest{})
	if err == nil {
		t.Error("Expected error when no target specified")
	}
}
