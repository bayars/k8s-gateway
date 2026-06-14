package ssh

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/safabayar/gateway/internal/config"
	"github.com/safabayar/gateway/internal/logger"
)

func TestMain(m *testing.M) {
	logger.InitLogger("/tmp/ssh_bastion_test.log", "debug")
	os.Exit(m.Run())
}

// ─── Test Helpers ──────────────────────────────────────────────────────────────

// mockConnMetadata satisfies ssh.ConnMetadata for publicKeyCallback tests.
type mockConnMetadata struct {
	user string
	addr net.Addr
}

func (m *mockConnMetadata) User() string          { return m.user }
func (m *mockConnMetadata) SessionID() []byte     { return nil }
func (m *mockConnMetadata) ClientVersion() []byte { return nil }
func (m *mockConnMetadata) ServerVersion() []byte { return nil }
func (m *mockConnMetadata) RemoteAddr() net.Addr  { return m.addr }
func (m *mockConnMetadata) LocalAddr() net.Addr   { return m.addr }

// generateTestHostKeyFile writes an RSA host key to a temp file and returns the path.
func generateTestHostKeyFile(t *testing.T) string {
	t.Helper()
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generateTestHostKeyFile: %v", err)
	}
	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	}
	f, err := os.CreateTemp("", "test-host-key-*")
	if err != nil {
		t.Fatalf("generateTestHostKeyFile: %v", err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	if err := pem.Encode(f, pemBlock); err != nil {
		t.Fatalf("generateTestHostKeyFile: pem.Encode: %v", err)
	}
	f.Close()
	return f.Name()
}

// generateTestClientKey creates an ed25519 key pair.
// Returns the ssh.Signer and a path to a temp authorized_keys file containing the public key.
func generateTestClientKey(t *testing.T) (ssh.Signer, string) {
	t.Helper()
	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generateTestClientKey: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(privKey)
	if err != nil {
		t.Fatalf("generateTestClientKey: NewSignerFromKey: %v", err)
	}
	f, err := os.CreateTemp("", "authorized-keys-*")
	if err != nil {
		t.Fatalf("generateTestClientKey: CreateTemp: %v", err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	if _, err := f.Write(ssh.MarshalAuthorizedKey(signer.PublicKey())); err != nil {
		t.Fatalf("generateTestClientKey: Write: %v", err)
	}
	f.Close()
	return signer, f.Name()
}

// newTestConfig returns a ConfigHolder with the given devices and domain suffix "safabayar.net".
func newTestConfig(devices map[string]config.DeviceConfig) *config.ConfigHolder {
	return config.NewConfigHolder(&config.Config{
		Devices:  devices,
		Settings: config.Settings{DomainSuffix: "safabayar.net"},
	}, "")
}

// startTestBastion starts a BastionServer on a random port, returns its address.
func startTestBastion(t *testing.T, cfg *config.ConfigHolder, authKeysPath string) string {
	t.Helper()
	hostKeyPath := generateTestHostKeyFile(t)
	bastion, err := NewBastionServer(cfg, hostKeyPath, authKeysPath)
	if err != nil {
		t.Fatalf("startTestBastion: NewBastionServer: %v", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("startTestBastion: net.Listen: %v", err)
	}
	t.Cleanup(func() {
		bastion.Stop()
		ln.Close()
	})
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go bastion.handleConnection(conn)
		}
	}()
	return ln.Addr().String()
}

// newTestSSHClient dials the bastion and returns an authenticated client.
func newTestSSHClient(t *testing.T, addr string, signer ssh.Signer) *ssh.Client {
	t.Helper()
	cfg := &ssh.ClientConfig{
		User:            "testuser",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		t.Fatalf("newTestSSHClient: Dial: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

// startEchoServer starts a TCP echo server and returns its address.
func startEchoServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("startEchoServer: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}(conn)
		}
	}()
	return ln.Addr().String()
}

// ─── Construction Tests ────────────────────────────────────────────────────────

func TestNewBastionServer_ValidSetup(t *testing.T) {
	hostKeyPath := generateTestHostKeyFile(t)
	signer, authPath := generateTestClientKey(t)
	cfg := newTestConfig(map[string]config.DeviceConfig{
		"srl1": {Hostname: "10.0.0.1", SSHPort: 22},
	})

	bs, err := NewBastionServer(cfg, hostKeyPath, authPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if bs == nil {
		t.Fatal("Expected non-nil BastionServer")
	}
	if bs.sshConfig == nil {
		t.Error("Expected sshConfig to be configured")
	}
	if bs.config == nil {
		t.Error("Expected config to be set")
	}

	// Verify the authorized key was loaded
	bs.mu.RLock()
	_, exists := bs.authorizedKeys[string(signer.PublicKey().Marshal())]
	bs.mu.RUnlock()
	if !exists {
		t.Error("Expected authorized key to be loaded")
	}

	bs.Stop()
}

func TestNewBastionServer_MissingHostKey(t *testing.T) {
	cfg := newTestConfig(nil)
	_, err := NewBastionServer(cfg, "/nonexistent/host.key", "")
	if err == nil {
		t.Fatal("Expected error for missing host key, got nil")
	}
}

func TestNewBastionServer_MissingAuthorizedKeysFile(t *testing.T) {
	// Missing authorized_keys file should warn but succeed (accepts all keys)
	hostKeyPath := generateTestHostKeyFile(t)
	cfg := newTestConfig(nil)

	bs, err := NewBastionServer(cfg, hostKeyPath, "/nonexistent/authorized_keys")
	if err != nil {
		t.Fatalf("Expected no error for missing authorized_keys, got: %v", err)
	}
	defer bs.Stop()

	bs.mu.RLock()
	count := len(bs.authorizedKeys)
	bs.mu.RUnlock()
	if count != 0 {
		t.Errorf("Expected 0 keys loaded from missing file, got %d", count)
	}
}

// ─── Authorized Keys Tests ─────────────────────────────────────────────────────

func TestLoadAuthorizedKeys_ValidKeys(t *testing.T) {
	// Generate two client keys and write both to authorized_keys
	_, privKey1, _ := ed25519.GenerateKey(rand.Reader)
	signer1, _ := ssh.NewSignerFromKey(privKey1)
	_, privKey2, _ := ed25519.GenerateKey(rand.Reader)
	signer2, _ := ssh.NewSignerFromKey(privKey2)

	f, _ := os.CreateTemp("", "auth-keys-*")
	t.Cleanup(func() { os.Remove(f.Name()) })
	_, _ = f.Write(ssh.MarshalAuthorizedKey(signer1.PublicKey()))
	_, _ = f.Write(ssh.MarshalAuthorizedKey(signer2.PublicKey()))
	f.Close()

	hostKeyPath := generateTestHostKeyFile(t)
	bs, err := NewBastionServer(newTestConfig(nil), hostKeyPath, f.Name())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer bs.Stop()

	bs.mu.RLock()
	count := len(bs.authorizedKeys)
	_, key1OK := bs.authorizedKeys[string(signer1.PublicKey().Marshal())]
	_, key2OK := bs.authorizedKeys[string(signer2.PublicKey().Marshal())]
	bs.mu.RUnlock()

	if count != 2 {
		t.Errorf("Expected 2 authorized keys, got %d", count)
	}
	if !key1OK {
		t.Error("Expected first key to be authorized")
	}
	if !key2OK {
		t.Error("Expected second key to be authorized")
	}
}

func TestLoadAuthorizedKeys_MissingFile(t *testing.T) {
	hostKeyPath := generateTestHostKeyFile(t)
	bs, err := NewBastionServer(newTestConfig(nil), hostKeyPath, "/no/such/file")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer bs.Stop()

	bs.mu.RLock()
	count := len(bs.authorizedKeys)
	bs.mu.RUnlock()
	if count != 0 {
		t.Errorf("Expected 0 keys for missing file, got %d", count)
	}
}

func TestLoadAuthorizedKeys_SkipsInvalidLinesAndComments(t *testing.T) {
	_, privKey, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := ssh.NewSignerFromKey(privKey)

	f, _ := os.CreateTemp("", "auth-keys-*")
	t.Cleanup(func() { os.Remove(f.Name()) })

	content := "# This is a comment\n" +
		"\n" +
		"not-a-valid-key\n" +
		string(ssh.MarshalAuthorizedKey(signer.PublicKey())) +
		"# Another comment\n"
	_, _ = f.Write([]byte(content))
	f.Close()

	hostKeyPath := generateTestHostKeyFile(t)
	bs, err := NewBastionServer(newTestConfig(nil), hostKeyPath, f.Name())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer bs.Stop()

	bs.mu.RLock()
	count := len(bs.authorizedKeys)
	bs.mu.RUnlock()
	if count != 1 {
		t.Errorf("Expected 1 valid key (comments and invalid lines skipped), got %d", count)
	}
}

// ─── Public Key Callback Tests ────────────────────────────────────────────────

func TestPublicKeyCallback_NoKeysAcceptsAll(t *testing.T) {
	// No authorized_keys file → keyCount == 0 → accept all
	hostKeyPath := generateTestHostKeyFile(t)
	bs, _ := NewBastionServer(newTestConfig(nil), hostKeyPath, "")
	defer bs.Stop()

	_, privKey, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := ssh.NewSignerFromKey(privKey)

	meta := &mockConnMetadata{
		user: "testuser",
		addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345},
	}
	perms, err := bs.publicKeyCallback(meta, signer.PublicKey())
	if err != nil {
		t.Errorf("Expected all keys accepted when no keys configured, got: %v", err)
	}
	if perms == nil {
		t.Error("Expected non-nil permissions")
	}
}

func TestPublicKeyCallback_AcceptsKnownKey(t *testing.T) {
	signer, authPath := generateTestClientKey(t)
	hostKeyPath := generateTestHostKeyFile(t)
	bs, _ := NewBastionServer(newTestConfig(nil), hostKeyPath, authPath)
	defer bs.Stop()

	meta := &mockConnMetadata{
		user: "testuser",
		addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345},
	}
	perms, err := bs.publicKeyCallback(meta, signer.PublicKey())
	if err != nil {
		t.Errorf("Expected known key to be accepted, got: %v", err)
	}
	if perms == nil {
		t.Fatal("Expected non-nil permissions")
	}
	if perms.Extensions["pubkey-fp"] == "" {
		t.Error("Expected pubkey fingerprint in permissions extensions")
	}
}

func TestPublicKeyCallback_RejectsUnknownKey(t *testing.T) {
	// Load one key, then try to authenticate with a different key
	_, authPath := generateTestClientKey(t)
	hostKeyPath := generateTestHostKeyFile(t)
	bs, _ := NewBastionServer(newTestConfig(nil), hostKeyPath, authPath)
	defer bs.Stop()

	// Generate a completely different key (not in authorized_keys)
	_, unknownPrivKey, _ := ed25519.GenerateKey(rand.Reader)
	unknownSigner, _ := ssh.NewSignerFromKey(unknownPrivKey)

	meta := &mockConnMetadata{
		user: "testuser",
		addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345},
	}
	_, err := bs.publicKeyCallback(meta, unknownSigner.PublicKey())
	if err == nil {
		t.Error("Expected unknown key to be rejected, got nil error")
	}
}

// ─── Direct TCP/IP Routing Tests ──────────────────────────────────────────────

func TestHandleDirectTCPIP_InventoryRouting(t *testing.T) {
	// Start an echo server to act as the mock device
	echoAddr := startEchoServer(t)
	echoHost, echoPortStr, _ := net.SplitHostPort(echoAddr)
	echoPort, _ := strconv.Atoi(echoPortStr)

	// Config: srl1.safabayar.net → echo server
	cfg := newTestConfig(map[string]config.DeviceConfig{
		"srl1": {Hostname: echoHost, SSHPort: echoPort},
	})

	signer, authPath := generateTestClientKey(t)
	bastionAddr := startTestBastion(t, cfg, authPath)
	client := newTestSSHClient(t, bastionAddr, signer)

	// ProxyJump sends direct-tcpip with TargetAddr="srl1.safabayar.net", TargetPort=22.
	// The bastion should route via inventory to echoHost:echoPort, not DNS-resolve safabayar.net.
	conn, err := client.Dial("tcp", "srl1.safabayar.net:22")
	if err != nil {
		t.Fatalf("direct-tcpip via inventory failed: %v", err)
	}
	defer conn.Close()

	// Verify tunnel reaches the echo server
	testData := []byte("hello-inventory-routing")
	if _, err := conn.Write(testData); err != nil {
		t.Fatalf("Write: %v", err)
	}

	buf := make([]byte, len(testData))
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("ReadFull: %v", err)
	}
	if !bytes.Equal(buf, testData) {
		t.Errorf("Expected echo %q, got %q", testData, buf)
	}
}

func TestHandleDirectTCPIP_InventoryOverridesPayloadPort(t *testing.T) {
	// The inventory's SSHPort should be used, not the port from the direct-tcpip payload.
	// Scenario: client sends port 22, device is actually on a random port.
	echoAddr := startEchoServer(t)
	echoHost, echoPortStr, _ := net.SplitHostPort(echoAddr)
	echoPort, _ := strconv.Atoi(echoPortStr)

	cfg := newTestConfig(map[string]config.DeviceConfig{
		"srl1": {Hostname: echoHost, SSHPort: echoPort}, // NOT port 22
	})

	signer, authPath := generateTestClientKey(t)
	bastionAddr := startTestBastion(t, cfg, authPath)
	client := newTestSSHClient(t, bastionAddr, signer)

	// Client sends port 22 in the direct-tcpip request (as ProxyJump would)
	conn, err := client.Dial("tcp", "srl1.safabayar.net:22")
	if err != nil {
		t.Fatalf("direct-tcpip failed: %v", err)
	}
	defer conn.Close()

	testData := []byte("port-override-check")
	_, _ = conn.Write(testData)
	buf := make([]byte, len(testData))
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("ReadFull (echo server not reached — port not overridden): %v", err)
	}
	if !bytes.Equal(buf, testData) {
		t.Errorf("Expected echo %q, got %q", testData, buf)
	}
}

func TestHandleDirectTCPIP_FallbackForwarding(t *testing.T) {
	// Target not in inventory → bastion falls back to raw OS DNS forwarding.
	echoAddr := startEchoServer(t)

	// Empty device inventory — nothing matches
	cfg := newTestConfig(map[string]config.DeviceConfig{})

	signer, authPath := generateTestClientKey(t)
	bastionAddr := startTestBastion(t, cfg, authPath)
	client := newTestSSHClient(t, bastionAddr, signer)

	// Dial directly to the echo server IP:port — not in inventory, forwarded as-is
	conn, err := client.Dial("tcp", echoAddr)
	if err != nil {
		t.Fatalf("direct-tcpip fallback failed: %v", err)
	}
	defer conn.Close()

	testData := []byte("hello-fallback")
	_, _ = conn.Write(testData)
	buf := make([]byte, len(testData))
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("ReadFull: %v", err)
	}
	if !bytes.Equal(buf, testData) {
		t.Errorf("Expected echo %q, got %q", testData, buf)
	}
}

// ─── Interactive Shell Tests ──────────────────────────────────────────────────

func TestInteractiveShell_WelcomeBanner(t *testing.T) {
	cfg := newTestConfig(map[string]config.DeviceConfig{
		"srl1": {Hostname: "10.0.0.1", SSHPort: 22},
	})
	signer, authPath := generateTestClientKey(t)
	bastionAddr := startTestBastion(t, cfg, authPath)
	client := newTestSSHClient(t, bastionAddr, signer)

	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer session.Close()

	var buf bytes.Buffer
	session.Stdout = &buf
	stdin, _ := session.StdinPipe()
	_ = session.Shell()

	time.Sleep(200 * time.Millisecond)
	_, _ = stdin.Write([]byte("exit\r\n"))
	_ = session.Wait()

	output := buf.String()
	for _, want := range []string{
		"Welcome to the Gateway Bastion Server",
		"Available devices",
		"srl1.safabayar.net",
		"Commands",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("Expected banner to contain %q\nGot:\n%s", want, output)
		}
	}
}

func TestInteractiveShell_ListCommand(t *testing.T) {
	cfg := newTestConfig(map[string]config.DeviceConfig{
		"srl1": {Hostname: "10.0.0.1", SSHPort: 22},
		"srl2": {Hostname: "10.0.0.2", SSHPort: 22},
	})
	signer, authPath := generateTestClientKey(t)
	bastionAddr := startTestBastion(t, cfg, authPath)
	client := newTestSSHClient(t, bastionAddr, signer)

	session, _ := client.NewSession()
	defer session.Close()

	var buf bytes.Buffer
	session.Stdout = &buf
	stdin, _ := session.StdinPipe()
	_ = session.Shell()

	time.Sleep(200 * time.Millisecond)
	_, _ = stdin.Write([]byte("list\r\n"))
	time.Sleep(100 * time.Millisecond)
	_, _ = stdin.Write([]byte("exit\r\n"))
	_ = session.Wait()

	output := buf.String()
	if !strings.Contains(output, "srl1.safabayar.net") {
		t.Errorf("Expected 'srl1.safabayar.net' in list output\nGot:\n%s", output)
	}
	if !strings.Contains(output, "srl2.safabayar.net") {
		t.Errorf("Expected 'srl2.safabayar.net' in list output\nGot:\n%s", output)
	}
}

func TestInteractiveShell_ExitCommand(t *testing.T) {
	cfg := newTestConfig(map[string]config.DeviceConfig{})
	signer, authPath := generateTestClientKey(t)
	bastionAddr := startTestBastion(t, cfg, authPath)
	client := newTestSSHClient(t, bastionAddr, signer)

	session, _ := client.NewSession()
	defer session.Close()

	var buf bytes.Buffer
	session.Stdout = &buf
	stdin, _ := session.StdinPipe()
	_ = session.Shell()

	time.Sleep(200 * time.Millisecond)
	_, _ = stdin.Write([]byte("exit\r\n"))
	_ = session.Wait()

	if !strings.Contains(buf.String(), "Goodbye") {
		t.Errorf("Expected 'Goodbye' message on exit\nGot:\n%s", buf.String())
	}
}

func TestInteractiveShell_UnknownCommand(t *testing.T) {
	cfg := newTestConfig(map[string]config.DeviceConfig{})
	signer, authPath := generateTestClientKey(t)
	bastionAddr := startTestBastion(t, cfg, authPath)
	client := newTestSSHClient(t, bastionAddr, signer)

	session, _ := client.NewSession()
	defer session.Close()

	var buf bytes.Buffer
	session.Stdout = &buf
	stdin, _ := session.StdinPipe()
	_ = session.Shell()

	time.Sleep(200 * time.Millisecond)
	_, _ = stdin.Write([]byte("foobar\r\n"))
	time.Sleep(100 * time.Millisecond)
	_, _ = stdin.Write([]byte("exit\r\n"))
	_ = session.Wait()

	output := buf.String()
	if !strings.Contains(output, "Unknown command") {
		t.Errorf("Expected 'Unknown command' for unrecognized input\nGot:\n%s", output)
	}
}

// ─── Exec Command Tests ───────────────────────────────────────────────────────

func TestExecCommand_DeviceNotFound(t *testing.T) {
	cfg := newTestConfig(map[string]config.DeviceConfig{
		"srl1": {Hostname: "10.0.0.1", SSHPort: 22},
	})
	signer, authPath := generateTestClientKey(t)
	bastionAddr := startTestBastion(t, cfg, authPath)
	client := newTestSSHClient(t, bastionAddr, signer)

	session, _ := client.NewSession()
	defer session.Close()

	var buf bytes.Buffer
	session.Stdout = &buf
	// Run returns error (no exit status from bastion) — ignore it
	_ = session.Run("ssh unknown.safabayar.net")

	output := buf.String()
	if !strings.Contains(output, "device not found") && !strings.Contains(output, "Error") {
		t.Errorf("Expected device-not-found error in output\nGot:\n%s", output)
	}
}

func TestExecCommand_InvalidCommandFormat(t *testing.T) {
	cfg := newTestConfig(map[string]config.DeviceConfig{})
	signer, authPath := generateTestClientKey(t)
	bastionAddr := startTestBastion(t, cfg, authPath)
	client := newTestSSHClient(t, bastionAddr, signer)

	session, _ := client.NewSession()
	defer session.Close()

	var buf bytes.Buffer
	session.Stdout = &buf
	_ = session.Run("notansscommand")

	output := buf.String()
	if !strings.Contains(output, "Invalid command") && !strings.Contains(output, "Error") {
		t.Errorf("Expected invalid command error in output\nGot:\n%s", output)
	}
}

// ─── Stop Tests ───────────────────────────────────────────────────────────────

func TestBastionServer_StopWithoutListener(t *testing.T) {
	hostKeyPath := generateTestHostKeyFile(t)
	bs, err := NewBastionServer(newTestConfig(nil), hostKeyPath, "")
	if err != nil {
		t.Fatalf("NewBastionServer: %v", err)
	}
	// Stop before Start — should be a no-op
	if err := bs.Stop(); err != nil {
		t.Errorf("Stop() without listener returned error: %v", err)
	}
}

func TestBastionServer_StopClosesListener(t *testing.T) {
	hostKeyPath := generateTestHostKeyFile(t)
	bs, _ := NewBastionServer(newTestConfig(nil), hostKeyPath, "")

	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	bs.listener = ln

	if err := bs.Stop(); err != nil {
		t.Errorf("Stop() returned error: %v", err)
	}

	// Verify listener is actually closed — a new Dial should fail
	_, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
	if err == nil {
		t.Error("Expected connection to fail after Stop(), but it succeeded")
	}
}
