package proxy

import (
	"bytes"
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/safabayar/gateway/internal/logger"
)

// ExecuteNetconfCommand executes a NETCONF RPC on a remote device
func ExecuteNetconfCommand(hostname string, port int, username, password, command string) (string, error) {
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	address := fmt.Sprintf("%s:%d", hostname, port)
	logger.Log.WithFields(map[string]interface{}{
		"address":  address,
		"username": username,
	}).Debug("Connecting to NETCONF server")

	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return "", fmt.Errorf("failed to dial NETCONF: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create NETCONF session: %w", err)
	}
	defer session.Close()

	// Request NETCONF subsystem
	if err := session.RequestSubsystem("netconf"); err != nil {
		return "", fmt.Errorf("failed to request NETCONF subsystem: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	var stdout bytes.Buffer
	session.Stdout = &stdout

	// Start session
	if err := session.Start(""); err != nil {
		return "", fmt.Errorf("failed to start session: %w", err)
	}

	// Read hello message
	time.Sleep(100 * time.Millisecond)

	// Send NETCONF hello
	hello := `<?xml version="1.0" encoding="UTF-8"?>
<hello xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
  <capabilities>
    <capability>urn:ietf:params:netconf:base:1.0</capability>
  </capabilities>
</hello>]]>]]>`

	if _, err := stdin.Write([]byte(hello)); err != nil {
		return "", fmt.Errorf("failed to send hello: %w", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Send RPC command
	logger.Log.WithField("command", command).Debug("Executing NETCONF RPC")

	// Wrap command in RPC tags if not already present
	rpc := command
	if !bytes.Contains([]byte(command), []byte("<rpc")) {
		rpc = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rpc message-id="1" xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
%s
</rpc>]]>]]>`, command)
	}

	if _, err := stdin.Write([]byte(rpc)); err != nil {
		return "", fmt.Errorf("failed to send RPC: %w", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Close RPC
	closeRPC := `<?xml version="1.0" encoding="UTF-8"?>
<rpc message-id="2" xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
  <close-session/>
</rpc>]]>]]>`

	_, _ = stdin.Write([]byte(closeRPC))
	_ = stdin.Close()

	// Wait for session to complete
	_ = session.Wait()

	return stdout.String(), nil
}
