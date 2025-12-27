package ssh

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/safabayar/gateway/internal/config"
	"github.com/safabayar/gateway/internal/logger"
	"golang.org/x/crypto/ssh"
)

// BastionServer implements SSH bastion/jump server functionality
type BastionServer struct {
	config        *config.Config
	sshConfig     *ssh.ServerConfig
	authorizedKeys map[string]ssh.PublicKey
	listener      net.Listener
	mu            sync.Mutex
}

// NewBastionServer creates a new SSH bastion server
func NewBastionServer(cfg *config.Config, hostKeyPath string, authorizedKeysPath string) (*BastionServer, error) {
	bs := &BastionServer{
		config:         cfg,
		authorizedKeys: make(map[string]ssh.PublicKey),
	}

	// Load authorized keys for client authentication
	if err := bs.loadAuthorizedKeys(authorizedKeysPath); err != nil {
		return nil, fmt.Errorf("failed to load authorized keys: %w", err)
	}

	// Configure SSH server
	sshConfig := &ssh.ServerConfig{
		PublicKeyCallback: bs.publicKeyCallback,
	}

	// Load host key
	hostKey, err := loadHostKey(hostKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load host key: %w", err)
	}
	sshConfig.AddHostKey(hostKey)

	bs.sshConfig = sshConfig
	return bs, nil
}

// loadAuthorizedKeys loads public keys for client authentication
func (bs *BastionServer) loadAuthorizedKeys(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		// If file doesn't exist, create empty map
		if os.IsNotExist(err) {
			logger.Log.Warn("Authorized keys file not found, will accept all keys (INSECURE)")
			return nil
		}
		return err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
		if err != nil {
			logger.Log.WithError(err).Warnf("Failed to parse authorized key: %s", line)
			continue
		}

		bs.authorizedKeys[string(pubKey.Marshal())] = pubKey
	}

	logger.Log.Infof("Loaded %d authorized keys", len(bs.authorizedKeys))
	return nil
}

// loadHostKey loads or generates SSH host key
func loadHostKey(path string) (ssh.Signer, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		logger.Log.Infof("Host key not found, generating new key at %s", path)
		return generateHostKey(path)
	}

	privateBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return ssh.ParsePrivateKey(privateBytes)
}

// generateHostKey generates a new SSH host key
func generateHostKey(path string) (ssh.Signer, error) {
	// For simplicity, we'll create a key using ssh-keygen
	// In production, use crypto/ed25519 or crypto/rsa to generate keys programmatically
	logger.Log.Error("Host key generation not implemented. Please generate using: ssh-keygen -t ed25519 -f " + path)
	return nil, fmt.Errorf("host key file not found: %s", path)
}

// publicKeyCallback validates client public keys
func (bs *BastionServer) publicKeyCallback(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	logger.Log.WithFields(map[string]interface{}{
		"user":      conn.User(),
		"remote":    conn.RemoteAddr().String(),
		"key_type": key.Type(),
	}).Debug("Public key authentication attempt")

	// If no authorized keys loaded, accept all (INSECURE - for development only)
	if len(bs.authorizedKeys) == 0 {
		logger.Log.Warn("No authorized keys configured, accepting all connections (INSECURE)")
		return &ssh.Permissions{
			Extensions: map[string]string{
				"pubkey-fp": ssh.FingerprintSHA256(key),
			},
		}, nil
	}

	// Check if key is authorized
	keyData := string(key.Marshal())
	if _, exists := bs.authorizedKeys[keyData]; exists {
		logger.Log.Infof("Accepted public key for user %s", conn.User())
		return &ssh.Permissions{
			Extensions: map[string]string{
				"pubkey-fp": ssh.FingerprintSHA256(key),
			},
		}, nil
	}

	logger.Log.Warnf("Rejected public key for user %s", conn.User())
	return nil, fmt.Errorf("unknown public key for %s", conn.User())
}

// Start starts the SSH bastion server
func (bs *BastionServer) Start(address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}

	bs.listener = listener
	logger.Log.Infof("SSH bastion server listening on %s", address)

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Log.WithError(err).Error("Failed to accept connection")
			continue
		}

		go bs.handleConnection(conn)
	}
}

// handleConnection handles an incoming SSH connection
func (bs *BastionServer) handleConnection(netConn net.Conn) {
	logger.Log.Infof("New connection from %s", netConn.RemoteAddr())

	// Perform SSH handshake
	sshConn, chans, reqs, err := ssh.NewServerConn(netConn, bs.sshConfig)
	if err != nil {
		logger.Log.WithError(err).Error("Failed to handshake")
		netConn.Close()
		return
	}
	defer sshConn.Close()

	logger.Log.Infof("SSH connection established for user %s from %s", sshConn.User(), sshConn.RemoteAddr())

	// Discard global requests
	go ssh.DiscardRequests(reqs)

	// Handle channels
	for newChannel := range chans {
		go bs.handleChannel(sshConn, newChannel)
	}
}

// handleChannel handles an SSH channel (session, direct-tcpip, etc.)
func (bs *BastionServer) handleChannel(sshConn *ssh.ServerConn, newChannel ssh.NewChannel) {
	logger.Log.Debugf("New channel type: %s", newChannel.ChannelType())

	switch newChannel.ChannelType() {
	case "session":
		bs.handleSession(sshConn, newChannel)
	case "direct-tcpip":
		bs.handleDirectTCPIP(sshConn, newChannel)
	default:
		newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %s", newChannel.ChannelType()))
	}
}

// ptyRequestMsg represents a PTY request payload
type ptyRequestMsg struct {
	Term     string
	Columns  uint32
	Rows     uint32
	Width    uint32
	Height   uint32
	Modelist []byte
}

// windowChangeMsg represents a window-change request payload
type windowChangeMsg struct {
	Columns uint32
	Rows    uint32
	Width   uint32
	Height  uint32
}

// handleSession handles an SSH session channel
func (bs *BastionServer) handleSession(sshConn *ssh.ServerConn, newChannel ssh.NewChannel) {
	channel, requests, err := newChannel.Accept()
	if err != nil {
		logger.Log.WithError(err).Error("Failed to accept channel")
		return
	}
	defer channel.Close()

	username := sshConn.User()

	// Terminal info from client
	var termInfo ptyRequestMsg
	termInfo.Term = "xterm-256color"
	termInfo.Columns = 80
	termInfo.Rows = 24

	// Handle session requests (shell, exec, pty-req, etc.)
	for req := range requests {
		switch req.Type {
		case "pty-req":
			// Parse PTY request to get terminal size
			if err := ssh.Unmarshal(req.Payload, &termInfo); err != nil {
				logger.Log.WithError(err).Warn("Failed to parse pty-req")
			}
			logger.Log.Infof("PTY request: term=%s, cols=%d, rows=%d", termInfo.Term, termInfo.Columns, termInfo.Rows)
			// Use defaults if client didn't send terminal size
			if termInfo.Columns == 0 {
				termInfo.Columns = 120
			}
			if termInfo.Rows == 0 {
				termInfo.Rows = 30
			}
			logger.Log.Infof("PTY size (after defaults): cols=%d, rows=%d", termInfo.Columns, termInfo.Rows)
			req.Reply(true, nil)

		case "shell":
			req.Reply(true, nil)
			// Run interactive shell with terminal info
			bs.runInteractiveShellWithPty(channel, username, &termInfo, requests)
			return

		case "exec":
			// Parse command - expected format: "ssh router1.myCustomer.safabayar.net"
			command := string(req.Payload[4:]) // Skip length prefix

			logger.Log.Infof("Exec request from %s: %s", username, command)

			// Handle the command with terminal info
			bs.handleCommandWithPty(channel, username, command, &termInfo, requests)
			req.Reply(true, nil)
			return

		default:
			req.Reply(false, nil)
		}
	}
}

// runInteractiveShellWithPty provides an interactive shell with PTY support
func (bs *BastionServer) runInteractiveShellWithPty(channel ssh.Channel, username string, termInfo *ptyRequestMsg, requests <-chan *ssh.Request) {
	// Pass termInfo and requests to runInteractiveShell so PTY info is available
	// when user types 'ssh <device>'
	bs.runInteractiveShellWithTermInfo(channel, username, termInfo, requests)
}

// runInteractiveShell provides an interactive shell for device selection (legacy without PTY)
func (bs *BastionServer) runInteractiveShell(channel ssh.Channel, username string) {
	bs.runInteractiveShellWithTermInfo(channel, username, nil, nil)
}

// runInteractiveShellWithTermInfo provides an interactive shell with optional PTY info
func (bs *BastionServer) runInteractiveShellWithTermInfo(channel ssh.Channel, username string, termInfo *ptyRequestMsg, requests <-chan *ssh.Request) {
	// Send welcome banner
	channel.Write([]byte("\r\n"))
	channel.Write([]byte("╔══════════════════════════════════════════════════════════════╗\r\n"))
	channel.Write([]byte("║           Welcome to the Gateway Bastion Server              ║\r\n"))
	channel.Write([]byte("╚══════════════════════════════════════════════════════════════╝\r\n"))
	channel.Write([]byte("\r\n"))
	channel.Write([]byte("Available devices:\r\n"))

	for deviceName := range bs.config.Devices {
		channel.Write([]byte(fmt.Sprintf("  • %s.%s\r\n", deviceName, bs.config.Settings.DomainSuffix)))
	}

	channel.Write([]byte("\r\n"))
	channel.Write([]byte("Commands:\r\n"))
	channel.Write([]byte("  ssh <device-fqdn>  - Connect to a device\r\n"))
	channel.Write([]byte("  list               - Show available devices\r\n"))
	channel.Write([]byte("  exit               - Close connection\r\n"))
	channel.Write([]byte("\r\n"))

	// Interactive command loop
	for {
		channel.Write([]byte("bastion> "))

		// Read command line with simple line editing
		line, err := bs.readLine(channel)
		if err != nil {
			logger.Log.WithError(err).Debug("Error reading from channel")
			return
		}

		command := strings.TrimSpace(line)
		if command == "" {
			continue
		}

		logger.Log.Infof("Interactive command from %s: %s", username, command)

		switch {
		case command == "exit" || command == "quit":
			channel.Write([]byte("Goodbye!\r\n"))
			return

		case command == "list" || command == "ls":
			channel.Write([]byte("\r\nAvailable devices:\r\n"))
			for deviceName := range bs.config.Devices {
				channel.Write([]byte(fmt.Sprintf("  • %s.%s\r\n", deviceName, bs.config.Settings.DomainSuffix)))
			}
			channel.Write([]byte("\r\n"))

		case strings.HasPrefix(command, "ssh "):
			// Use PTY-aware handler if we have termInfo
			if termInfo != nil {
				bs.handleCommandWithPty(channel, username, command, termInfo, requests)
			} else {
				bs.handleCommand(channel, username, command)
			}
			// After device session ends, show prompt again
			channel.Write([]byte("\r\n"))

		default:
			channel.Write([]byte(fmt.Sprintf("Unknown command: %s\r\n", command)))
			channel.Write([]byte("Use 'ssh <device-fqdn>' to connect or 'exit' to quit\r\n"))
		}
	}
}

// readLine reads a line from the channel with basic line editing
func (bs *BastionServer) readLine(channel ssh.Channel) (string, error) {
	var line []byte
	buf := make([]byte, 1)

	for {
		n, err := channel.Read(buf)
		if err != nil {
			return "", err
		}
		if n == 0 {
			continue
		}

		char := buf[0]

		switch char {
		case '\r', '\n':
			channel.Write([]byte("\r\n"))
			return string(line), nil

		case 127, 8: // Backspace or Delete
			if len(line) > 0 {
				line = line[:len(line)-1]
				channel.Write([]byte("\b \b")) // Erase character on screen
			}

		case 3: // Ctrl+C
			channel.Write([]byte("^C\r\n"))
			return "", fmt.Errorf("interrupted")

		case 4: // Ctrl+D
			if len(line) == 0 {
				return "exit", nil
			}

		default:
			if char >= 32 && char < 127 {
				line = append(line, char)
				channel.Write([]byte{char}) // Echo character
			}
		}
	}
}

// handleCommandWithPty processes ssh commands with PTY info
func (bs *BastionServer) handleCommandWithPty(channel ssh.Channel, defaultUsername, command string, termInfo *ptyRequestMsg, requests <-chan *ssh.Request) {
	parts := strings.Fields(command)
	if len(parts) < 2 || parts[0] != "ssh" {
		channel.Write([]byte("Error: Invalid command format. Use: ssh <device-fqdn>\r\n"))
		return
	}

	targetFQDN := parts[1]

	// Get device config
	device, deviceName, err := bs.config.GetDeviceByFQDN(targetFQDN)
	if err != nil {
		channel.Write([]byte(fmt.Sprintf("Error: %s\r\n", err)))
		return
	}

	channel.Write([]byte(fmt.Sprintf("Connecting to %s (%s)...\r\n", deviceName, device.Hostname)))

	// Prompt for username
	channel.Write([]byte(fmt.Sprintf("Username [%s]: ", defaultUsername)))
	usernameInput, err := bs.readLine(channel)
	if err != nil {
		channel.Write([]byte(fmt.Sprintf("Error reading username: %s\r\n", err)))
		return
	}

	// Use default if empty
	username := strings.TrimSpace(usernameInput)
	if username == "" {
		username = defaultUsername
	}

	// Prompt for password
	channel.Write([]byte("Password: "))
	password, err := bs.readPassword(channel)
	if err != nil {
		channel.Write([]byte(fmt.Sprintf("\r\nError reading password: %s\r\n", err)))
		return
	}
	channel.Write([]byte("\r\n"))

	// Connect to target device with PTY info
	logger.Log.Infof("Proxying to device with PTY: cols=%d, rows=%d, term=%s", termInfo.Columns, termInfo.Rows, termInfo.Term)
	bs.proxyToDeviceWithPty(channel, device, username, password, termInfo, requests)
}

// handleCommand processes ssh commands (legacy without PTY)
func (bs *BastionServer) handleCommand(channel ssh.Channel, defaultUsername, command string) {
	parts := strings.Fields(command)
	if len(parts) < 2 || parts[0] != "ssh" {
		channel.Write([]byte("Error: Invalid command format. Use: ssh <device-fqdn>\r\n"))
		return
	}

	targetFQDN := parts[1]

	// Get device config
	device, deviceName, err := bs.config.GetDeviceByFQDN(targetFQDN)
	if err != nil {
		channel.Write([]byte(fmt.Sprintf("Error: %s\r\n", err)))
		return
	}

	channel.Write([]byte(fmt.Sprintf("Connecting to %s (%s)...\r\n", deviceName, device.Hostname)))

	// Prompt for username
	channel.Write([]byte(fmt.Sprintf("Username [%s]: ", defaultUsername)))
	usernameInput, err := bs.readLine(channel)
	if err != nil {
		channel.Write([]byte(fmt.Sprintf("Error reading username: %s\r\n", err)))
		return
	}

	// Use default if empty
	username := strings.TrimSpace(usernameInput)
	if username == "" {
		username = defaultUsername
	}

	// Prompt for password
	channel.Write([]byte("Password: "))
	password, err := bs.readPassword(channel)
	if err != nil {
		channel.Write([]byte(fmt.Sprintf("\r\nError reading password: %s\r\n", err)))
		return
	}
	channel.Write([]byte("\r\n"))

	// Connect to target device
	bs.proxyToDevice(channel, device, username, password)
}

// readPassword reads password without echoing
func (bs *BastionServer) readPassword(channel ssh.Channel) (string, error) {
	var password []byte
	buf := make([]byte, 1)
	var lastChar byte

	for {
		n, err := channel.Read(buf)
		if err != nil {
			return "", err
		}
		if n == 0 {
			continue
		}

		char := buf[0]

		switch char {
		case '\r', '\n':
			return string(password), nil

		case 127, 8: // Backspace
			if len(password) > 0 {
				password = password[:len(password)-1]
			}

		case 3: // Ctrl+C
			return "", fmt.Errorf("interrupted")

		case '\\': // Backslash - might be escape prefix
			// Store but don't add yet, check if next char is escaped
			lastChar = char

		default:
			if char >= 32 && char < 127 {
				// If previous char was backslash and this is a special char,
				// skip the backslash (unescape)
				if lastChar == '\\' && (char == '!' || char == '$' || char == '`' || char == '"' || char == '\\') {
					// Don't add the backslash, just add this char
					password = append(password, char)
				} else {
					// Add pending backslash if any
					if lastChar == '\\' {
						password = append(password, lastChar)
					}
					password = append(password, char)
				}
				lastChar = 0
			}
		}
	}
}

// handleDirectTCPIP handles direct TCP/IP forwarding
func (bs *BastionServer) handleDirectTCPIP(sshConn *ssh.ServerConn, newChannel ssh.NewChannel) {
	// Parse direct-tcpip payload to get target address
	var payload struct {
		TargetAddr string
		TargetPort uint32
		OriginAddr string
		OriginPort uint32
	}

	if err := ssh.Unmarshal(newChannel.ExtraData(), &payload); err != nil {
		newChannel.Reject(ssh.ConnectionFailed, "failed to parse forward data")
		return
	}

	logger.Log.Infof("Direct TCP/IP forward request to %s:%d", payload.TargetAddr, payload.TargetPort)

	channel, requests, err := newChannel.Accept()
	if err != nil {
		logger.Log.WithError(err).Error("Failed to accept channel")
		return
	}
	defer channel.Close()

	go ssh.DiscardRequests(requests)

	// Connect to target
	targetConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", payload.TargetAddr, payload.TargetPort))
	if err != nil {
		logger.Log.WithError(err).Error("Failed to connect to target")
		return
	}
	defer targetConn.Close()

	// Bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		io.Copy(channel, targetConn)
		wg.Done()
	}()

	go func() {
		io.Copy(targetConn, channel)
		wg.Done()
	}()

	wg.Wait()
}

// proxyToDevice establishes connection to target device and proxies traffic
func (bs *BastionServer) proxyToDevice(clientChannel ssh.Channel, device *config.DeviceConfig, username, password string) {
	// Configure SSH client for target device
	// Support both password and keyboard-interactive authentication
	targetConfig := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range questions {
					answers[i] = password
				}
				return answers, nil
			}),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Connect to target device
	targetAddr := fmt.Sprintf("%s:%d", device.Hostname, device.SSHPort)
	targetConn, err := ssh.Dial("tcp", targetAddr, targetConfig)
	if err != nil {
		clientChannel.Write([]byte(fmt.Sprintf("\nError: Failed to connect to device: %s\n", err)))
		return
	}
	defer targetConn.Close()

	// Create session on target
	targetSession, err := targetConn.NewSession()
	if err != nil {
		clientChannel.Write([]byte(fmt.Sprintf("\nError: Failed to create session: %s\n", err)))
		return
	}
	defer targetSession.Close()

	// Setup I/O
	targetSession.Stdout = clientChannel
	targetSession.Stderr = clientChannel
	targetSession.Stdin = clientChannel

	// Request PTY
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := targetSession.RequestPty("xterm", 80, 40, modes); err != nil {
		clientChannel.Write([]byte(fmt.Sprintf("\nError: Failed to request PTY: %s\n", err)))
		return
	}

	// Start shell
	if err := targetSession.Shell(); err != nil {
		clientChannel.Write([]byte(fmt.Sprintf("\nError: Failed to start shell: %s\n", err)))
		return
	}

	// Wait for session to end
	targetSession.Wait()
	clientChannel.Write([]byte("\n\nConnection closed.\n"))
}

// proxyToDeviceWithPty establishes connection with proper PTY handling
func (bs *BastionServer) proxyToDeviceWithPty(clientChannel ssh.Channel, device *config.DeviceConfig, username, password string, termInfo *ptyRequestMsg, requests <-chan *ssh.Request) {
	// Configure SSH client for target device
	targetConfig := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range questions {
					answers[i] = password
				}
				return answers, nil
			}),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Connect to target device
	targetAddr := fmt.Sprintf("%s:%d", device.Hostname, device.SSHPort)
	targetConn, err := ssh.Dial("tcp", targetAddr, targetConfig)
	if err != nil {
		clientChannel.Write([]byte(fmt.Sprintf("\nError: Failed to connect to device: %s\n", err)))
		return
	}
	defer targetConn.Close()

	// Create session on target
	targetSession, err := targetConn.NewSession()
	if err != nil {
		clientChannel.Write([]byte(fmt.Sprintf("\nError: Failed to create session: %s\n", err)))
		return
	}
	defer targetSession.Close()

	// Setup I/O
	targetSession.Stdout = clientChannel
	targetSession.Stderr = clientChannel
	targetSession.Stdin = clientChannel

	// Request PTY with client's terminal size
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	// Use client's terminal info
	term := termInfo.Term
	if term == "" {
		term = "xterm-256color"
	}
	cols := int(termInfo.Columns)
	if cols == 0 {
		cols = 80
	}
	rows := int(termInfo.Rows)
	if rows == 0 {
		rows = 24
	}

	if err := targetSession.RequestPty(term, rows, cols, modes); err != nil {
		clientChannel.Write([]byte(fmt.Sprintf("\nError: Failed to request PTY: %s\n", err)))
		return
	}

	// Handle window-change requests from client
	go func() {
		for req := range requests {
			if req.Type == "window-change" {
				var winChange windowChangeMsg
				if err := ssh.Unmarshal(req.Payload, &winChange); err == nil {
					// Send window-change to target session
					targetSession.WindowChange(int(winChange.Rows), int(winChange.Columns))
				}
				if req.WantReply {
					req.Reply(true, nil)
				}
			} else {
				if req.WantReply {
					req.Reply(false, nil)
				}
			}
		}
	}()

	// Start shell
	if err := targetSession.Shell(); err != nil {
		clientChannel.Write([]byte(fmt.Sprintf("\nError: Failed to start shell: %s\n", err)))
		return
	}

	// Wait for session to end
	targetSession.Wait()
	clientChannel.Write([]byte("\n\nConnection closed.\n"))
}

// Stop stops the SSH bastion server
func (bs *BastionServer) Stop() error {
	if bs.listener != nil {
		return bs.listener.Close()
	}
	return nil
}
