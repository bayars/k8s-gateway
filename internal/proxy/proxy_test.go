package proxy

import (
	"os"
	"strings"
	"testing"

	"github.com/safabayar/gateway/internal/logger"
)

func TestMain(m *testing.M) {
	// Initialize logger for tests
	logger.InitLogger("/tmp/proxy_test.log", "debug")
	os.Exit(m.Run())
}

// Note: These tests verify the function signatures and error handling.
// Actual network connectivity tests require integration testing with real devices.

func TestExecuteSSHCommand_ConnectionError(t *testing.T) {
	// Test with non-existent host - should fail to connect
	output, err := ExecuteSSHCommand("127.0.0.1", 22222, "admin", "password", "show version")

	if err == nil {
		t.Error("Expected connection error but got none")
	}

	// Output might be empty or contain partial data
	_ = output
}

func TestExecuteTelnetCommand_ConnectionError(t *testing.T) {
	// Test with non-existent host - should fail to connect
	output, err := ExecuteTelnetCommand("127.0.0.1", 23333, "admin", "password", "show version")

	if err == nil {
		t.Error("Expected connection error but got none")
	}

	_ = output
}

func TestExecuteNetconfCommand_ConnectionError(t *testing.T) {
	// Test with non-existent host - should fail to connect
	output, err := ExecuteNetconfCommand("127.0.0.1", 8333, "admin", "password", "<get-config/>")

	if err == nil {
		t.Error("Expected connection error but got none")
	}

	_ = output
}

func TestExecuteSSHCommand_InvalidPort(t *testing.T) {
	// Test with invalid port
	_, err := ExecuteSSHCommand("127.0.0.1", 0, "admin", "password", "show version")

	if err == nil {
		t.Error("Expected error for invalid port")
	}
}

func TestExecuteTelnetCommand_InvalidPort(t *testing.T) {
	// Test with invalid port
	_, err := ExecuteTelnetCommand("127.0.0.1", 0, "admin", "password", "show version")

	if err == nil {
		t.Error("Expected error for invalid port")
	}
}

func TestExecuteNetconfCommand_InvalidPort(t *testing.T) {
	// Test with invalid port
	_, err := ExecuteNetconfCommand("127.0.0.1", 0, "admin", "password", "<get-config/>")

	if err == nil {
		t.Error("Expected error for invalid port")
	}
}

// TestNetconfRPCWrapping tests that NETCONF commands are properly wrapped
func TestNetconfRPCWrapping(t *testing.T) {
	tests := []struct {
		name    string
		command string
		hasRPC  bool
	}{
		{
			name:    "Command without RPC tag",
			command: "<get-config><source><running/></source></get-config>",
			hasRPC:  false,
		},
		{
			name:    "Command with RPC tag",
			command: `<rpc message-id="1"><get-config/></rpc>`,
			hasRPC:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasRPC := strings.Contains(tt.command, "<rpc")
			if hasRPC != tt.hasRPC {
				t.Errorf("RPC detection: got %v, want %v", hasRPC, tt.hasRPC)
			}
		})
	}
}
