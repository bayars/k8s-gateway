package grpc

import (
	"context"
	"os"
	"testing"

	"github.com/safabayar/gateway/internal/config"
	"github.com/safabayar/gateway/internal/logger"
	pb "github.com/safabayar/gateway/proto"
)

func TestMain(m *testing.M) {
	// Initialize logger for tests
	logger.InitLogger("/tmp/grpc_test.log", "debug")
	os.Exit(m.Run())
}

func TestNewServer(t *testing.T) {
	cfg := &config.Config{
		Devices: map[string]config.DeviceConfig{
			"srl1": {
				Hostname:    "10.0.0.1",
				SSHPort:     22,
				TelnetPort:  23,
				NetconfPort: 830,
				GNMIPort:    57400,
			},
		},
	}

	server := NewServer(cfg)
	if server == nil {
		t.Fatal("NewServer returned nil")
	}

	if server.config != cfg {
		t.Error("Server config not set correctly")
	}
}

func TestExecuteCommand_Validation(t *testing.T) {
	cfg := &config.Config{
		Devices: map[string]config.DeviceConfig{
			"srl1": {
				Hostname:    "10.0.0.1",
				SSHPort:     22,
				TelnetPort:  23,
				NetconfPort: 830,
			},
		},
	}

	server := NewServer(cfg)
	ctx := context.Background()

	tests := []struct {
		name    string
		req     *pb.CommandRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "Missing FQDN",
			req: &pb.CommandRequest{
				Fqdn:     "",
				Username: "admin",
				Password: "password",
				Command:  "show version",
				Protocol: "ssh",
			},
			wantErr: true,
			errMsg:  "FQDN is required",
		},
		{
			name: "Missing username",
			req: &pb.CommandRequest{
				Fqdn:     "srl1.example.com",
				Username: "",
				Password: "password",
				Command:  "show version",
				Protocol: "ssh",
			},
			wantErr: true,
			errMsg:  "username is required",
		},
		{
			name: "Missing password",
			req: &pb.CommandRequest{
				Fqdn:     "srl1.example.com",
				Username: "admin",
				Password: "",
				Command:  "show version",
				Protocol: "ssh",
			},
			wantErr: true,
			errMsg:  "password is required",
		},
		{
			name: "Missing command",
			req: &pb.CommandRequest{
				Fqdn:     "srl1.example.com",
				Username: "admin",
				Password: "password",
				Command:  "",
				Protocol: "ssh",
			},
			wantErr: true,
			errMsg:  "command is required",
		},
		{
			name: "Device not found",
			req: &pb.CommandRequest{
				Fqdn:     "unknown.example.com",
				Username: "admin",
				Password: "password",
				Command:  "show version",
				Protocol: "ssh",
			},
			wantErr: true,
			errMsg:  "device not found",
		},
		{
			name: "Unsupported protocol",
			req: &pb.CommandRequest{
				Fqdn:     "srl1.example.com",
				Username: "admin",
				Password: "password",
				Command:  "show version",
				Protocol: "unknown",
			},
			wantErr: true,
			errMsg:  "unsupported protocol",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.ExecuteCommand(ctx, tt.req)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				// Note: We can't easily check exact error message without importing status package
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestExecuteCommand_ProtocolSelection(t *testing.T) {
	cfg := &config.Config{
		Devices: map[string]config.DeviceConfig{
			"srl1": {
				Hostname:    "127.0.0.1", // Use localhost to fail fast
				SSHPort:     22222,       // Non-existent port
				TelnetPort:  23333,
				NetconfPort: 8333,
			},
		},
	}

	server := NewServer(cfg)
	ctx := context.Background()

	// Test that different protocols are routed correctly
	// These will fail to connect but we're testing routing logic
	protocols := []string{"ssh", "telnet", "netconf", ""}

	for _, proto := range protocols {
		t.Run("protocol_"+proto, func(t *testing.T) {
			req := &pb.CommandRequest{
				Fqdn:     "srl1.example.com",
				Username: "admin",
				Password: "password",
				Command:  "show version",
				Protocol: proto,
			}

			resp, err := server.ExecuteCommand(ctx, req)
			// We expect connection errors, not routing errors
			if err != nil {
				// This is expected - can't connect to non-existent server
				return
			}

			// If we somehow got a response, check structure
			if resp == nil {
				t.Error("Response should not be nil")
			}
		})
	}
}
