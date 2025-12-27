package proxy

import (
	"bytes"
	"fmt"
	"time"

	"github.com/safabayar/gateway/internal/logger"
	"golang.org/x/crypto/ssh"
)

// ExecuteSSHCommand executes a command on a remote device via SSH
func ExecuteSSHCommand(hostname string, port int, username, password, command string) (string, error) {
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // In production, use proper host key verification
		Timeout:         30 * time.Second,
	}

	address := fmt.Sprintf("%s:%d", hostname, port)
	logger.Log.WithFields(map[string]interface{}{
		"address":  address,
		"username": username,
	}).Debug("Connecting to SSH server")

	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return "", fmt.Errorf("failed to dial SSH: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	logger.Log.WithField("command", command).Debug("Executing SSH command")

	if err := session.Run(command); err != nil {
		return stdout.String() + stderr.String(), fmt.Errorf("command execution failed: %w", err)
	}

	return stdout.String(), nil
}
