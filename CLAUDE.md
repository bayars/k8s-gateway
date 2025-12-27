# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

```bash
# Generate protobuf code (required after modifying proto/gateway.proto)
make proto

# Build the gateway binary
make build

# Run locally for development
make run

# Run tests
make test

# Format code
make fmt

# Generate SSH keys (required for first-time setup)
make generate-keys

# Build Docker image
make docker-build

# Deploy to Kubernetes
make k8s-deploy

# Remove from Kubernetes
make k8s-delete
```

## Architecture Overview

This is a **multi-protocol gateway** that acts as a unified access point for network devices. It supports four protocols: gRPC, SSH (bastion mode), Telnet, and NETCONF.

### Key Design Patterns

**FQDN-Based Routing**: The gateway extracts device identifiers from FQDNs (e.g., `router1.myCustomer.safabayar.net` → `router1`) and looks up the device configuration in `config/devices.yaml`.

**Authentication Chain**:
- Client → Gateway: SSH public key authentication (for SSH bastion) or username/password (for gRPC)
- Gateway → Backend Device: Password-based authentication

**TLS Termination**: TLS is handled at the Kubernetes Gateway API layer, not within the application. The gateway receives plaintext traffic after TLS termination.

### Protocol Implementations

1. **gRPC Server** (`internal/grpc/server.go`):
   - Accepts FQDN, username, password, command, and protocol in requests
   - Routes to appropriate proxy based on protocol field
   - Supports both unary and streaming RPCs

2. **SSH Bastion** (`internal/ssh/bastion.go`):
   - Acts as SSH jump/bastion server
   - Authenticates clients using public keys from `config/authorized_keys`
   - Proxies connections to backend devices using password auth
   - Handles both interactive shell sessions and direct-tcpip port forwarding

3. **Protocol Proxies** (`internal/proxy/`):
   - `ssh.go`: Executes commands via SSH client library
   - `telnet.go`: Raw TCP connection with manual authentication handshake
   - `netconf.go`: SSH-based NETCONF over SSH subsystem

### Critical Configuration

**Device Routing Table** (`config/devices.yaml`):
```yaml
devices:
  <device-name>:  # This is extracted from FQDN subdomain
    hostname: "<backend-ip>"
    ssh_port: 22
    telnet_port: 23
    netconf_port: 830
```

The device name in YAML must match the first subdomain of the FQDN. For example, `router1.myCustomer.safabayar.net` requires a device entry named `router1`.

**SSH Keys**:
- `config/ssh_host_key`: Server's private host key (generate with `make generate-keys`)
- `config/authorized_keys`: Client public keys allowed to connect
- Both are mounted as Kubernetes secrets in production

### Kubernetes Deployment

**Gateway API Resources** (`k8s/gateway-api.yaml`):
- HTTPS listener (port 443) for gRPC with TLS termination
- TCP listener (port 22) for SSH bastion with TCP passthrough
- HTTPRoute matches gRPC requests by `content-type: application/grpc` header
- TCPRoute forwards SSH traffic directly

**Services**:
- `gateway-grpc`: ClusterIP service for gRPC (internal only)
- `gateway-ssh`: LoadBalancer service for SSH (external access on port 22)
- `gateway-netconf`: ClusterIP service for NETCONF (internal only)

### Port Mapping

- **gRPC**: External 443 (HTTPS) → Internal 50051
- **SSH**: External 22 → Internal 2222
- **NETCONF**: Port 830 (accessed via gRPC, not directly exposed)
- **Telnet**: Port 23 (accessed via gRPC, not directly exposed)

### Logging

The logger (`internal/logger/logger.go`) writes to both stdout and a file simultaneously using `io.MultiWriter`. This ensures logs appear in container stdout (for Kubernetes) and are persisted to files.

Log format: Structured JSON with timestamp, level, message, and fields.

### Security Considerations

**Host Key Verification**: The current implementation uses `ssh.InsecureIgnoreHostKey()` for simplicity. For production, implement proper host key verification by:
1. Maintaining a known_hosts database
2. Using `ssh.FixedHostKey()` for static keys
3. Implementing custom `HostKeyCallback` that validates against stored keys

**Secret Management**: SSH keys and TLS certificates are stored in Kubernetes secrets. Never commit these to Git.

**Password Transmission**: Passwords are transmitted in plaintext within the Kubernetes cluster after TLS termination. Consider implementing additional encryption for sensitive credentials.

### Protocol-Specific Implementation Notes

**SSH (`internal/proxy/ssh.go`)**:
- Uses `golang.org/x/crypto/ssh` client library
- Creates new session for each command execution
- Captures both stdout and stderr
- 30-second timeout for connections

**Telnet (`internal/proxy/telnet.go`)**:
- Uses raw TCP sockets (no telnet protocol library)
- Manual authentication: reads prompts, sends username/password
- Relies on timing (sleep) to wait for command output
- Less reliable than SSH; consider for legacy devices only

**NETCONF (`internal/proxy/netconf.go`)**:
- Uses SSH with "netconf" subsystem request
- Sends NETCONF hello handshake
- Wraps commands in `<rpc>` tags if not already present
- Uses `]]>]]>` framing for message delimitation

### Common Development Tasks

**Adding a New Device**:
1. Edit `config/devices.yaml` (or `k8s/configmap.yaml` for Kubernetes)
2. Add device entry with hostname and ports
3. Restart gateway or update ConfigMap

**Modifying gRPC Protocol**:
1. Edit `proto/gateway.proto`
2. Run `make proto` to regenerate Go code
3. Update `internal/grpc/server.go` to handle new fields

**Changing Authentication**:
- For SSH client→gateway: Modify `publicKeyCallback` in `internal/ssh/bastion.go`
- For gateway→device: Modify auth methods in proxy implementations

**Debugging Connection Issues**:
1. Check logs: `kubectl logs -f deployment/gateway`
2. Verify device configuration in ConfigMap
3. Test backend connectivity: `kubectl exec -it <pod> -- ssh user@<device-ip>`
4. Ensure SSH keys are properly mounted: `kubectl exec -it <pod> -- ls -la /root/.ssh/`

### Testing Locally

Before deploying to Kubernetes, test locally:

```bash
# Terminal 1: Run gateway
make run

# Terminal 2: Test gRPC (requires grpcurl or custom client)
grpcurl -plaintext -d '{"fqdn":"router1.myCustomer.safabayar.net","username":"admin","password":"pass","command":"show version","protocol":"ssh"}' localhost:50051 gateway.Gateway/ExecuteCommand

# Terminal 3: Test SSH bastion
ssh -i config/client_key -p 2222 user@localhost
```

### Error Handling Patterns

- All proxy functions return `(string, error)` - output and error
- gRPC responses include both output and error fields
- Exit codes: 0 for success, 1 for failure
- Errors are logged with context using `logrus.WithError()` and `logrus.WithFields()`

### Dependencies

- `google.golang.org/grpc`: gRPC framework
- `golang.org/x/crypto/ssh`: SSH client and server
- `github.com/sirupsen/logrus`: Structured logging
- `gopkg.in/yaml.v3`: YAML configuration parsing
- `google.golang.org/protobuf`: Protocol buffer runtime

### File Structure

```
cmd/gateway/main.go           - Entry point, starts gRPC and SSH servers
internal/
  config/config.go            - YAML config loading and device lookup
  grpc/server.go              - gRPC service implementation
  logger/logger.go            - Logging initialization
  proxy/
    ssh.go                    - SSH command execution
    telnet.go                 - Telnet command execution
    netconf.go                - NETCONF RPC execution
  ssh/bastion.go              - SSH bastion/jump server
proto/gateway.proto           - gRPC service definition
config/devices.yaml           - Device routing table
k8s/                         - Kubernetes manifests
```
