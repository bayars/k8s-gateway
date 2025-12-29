package proxy

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/safabayar/gateway/internal/logger"
)

// ExecuteTelnetCommand executes a command on a remote device via Telnet
func ExecuteTelnetCommand(hostname string, port int, username, password, command string) (string, error) {
	address := fmt.Sprintf("%s:%d", hostname, port)
	logger.Log.WithFields(map[string]interface{}{
		"address":  address,
		"username": username,
	}).Debug("Connecting to Telnet server")

	conn, err := net.DialTimeout("tcp", address, 30*time.Second)
	if err != nil {
		return "", fmt.Errorf("failed to connect to telnet: %w", err)
	}
	defer conn.Close()

	// Set read/write deadlines
	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return "", fmt.Errorf("failed to set deadline: %w", err)
	}

	// Read initial prompt
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return "", fmt.Errorf("failed to read initial prompt: %w", err)
	}

	output := string(buf[:n])

	// Send username
	if _, err := conn.Write([]byte(username + "\r\n")); err != nil {
		return "", fmt.Errorf("failed to send username: %w", err)
	}

	// Read password prompt
	n, err = conn.Read(buf)
	if err != nil {
		return "", fmt.Errorf("failed to read password prompt: %w", err)
	}
	output += string(buf[:n])

	// Send password
	if _, err := conn.Write([]byte(password + "\r\n")); err != nil {
		return "", fmt.Errorf("failed to send password: %w", err)
	}

	// Read login response
	n, err = conn.Read(buf)
	if err != nil {
		return "", fmt.Errorf("failed to read login response: %w", err)
	}
	output += string(buf[:n])

	// Send command
	logger.Log.WithField("command", command).Debug("Executing Telnet command")
	if _, err := conn.Write([]byte(command + "\r\n")); err != nil {
		return "", fmt.Errorf("failed to send command: %w", err)
	}

	// Read command output
	time.Sleep(100 * time.Millisecond) // Brief delay for command execution

	n, err = conn.Read(buf)
	if err != nil && !strings.Contains(err.Error(), "i/o timeout") {
		return output, fmt.Errorf("failed to read command output: %w", err)
	}

	if n > 0 {
		output += string(buf[:n])
	}

	// Send exit command
	_, _ = conn.Write([]byte("exit\r\n"))

	return output, nil
}
