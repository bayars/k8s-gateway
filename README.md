# Multi-Protocol Gateway

A multi-protocol gateway service that provides unified access to network devices through gRPC, SSH, Telnet, and NETCONF protocols. The gateway acts as a bastion/jump server with FQDN-based routing to backend devices.

## Features

- **Multi-Protocol Support**: gRPC, SSH bastion, Telnet proxy, and NETCONF proxy
- **FQDN-Based Routing**: Route to devices using fully qualified domain names (e.g., `router1.myCustomer.safabayar.net`)
- **Authentication**:
  - Client → Gateway: SSH public key authentication
  - gRPC: Username/password in request body
  - Gateway → Device: Password-based authentication
- **Kubernetes Native**: Designed for deployment with Kubernetes Gateway API
- **TLS Termination**: TLS handled at Gateway API layer
- **Comprehensive Logging**: File-based and stdout logging with structured logs

## Architecture

```
Client → Gateway API (TLS termination) → Gateway Service → Backend Devices
         ├─ gRPC (port 443)            ├─ SSH (port 22)
         ├─ SSH (port 22)               ├─ Telnet (port 23)
         └─ NETCONF (port 830)          └─ NETCONF (port 830)
```

## Prerequisites

- Go 1.21 or higher
- Docker (for containerization)
- Kubernetes cluster with Gateway API CRDs installed
- `protoc` compiler for gRPC development

## Quick Start

### 1. Generate SSH Keys

```bash
make generate-keys
```

### 2. Configure Devices

Edit `config/devices.yaml` to add your network devices:

```yaml
devices:
  router1:
    hostname: "10.0.1.10"
    ssh_port: 22
    telnet_port: 23
    netconf_port: 830
    description: "Core Router 1"
```

### 3. Build and Run Locally

```bash
# Build the application
make build

# Run the gateway
make run
```

### 4. Deploy to Kubernetes

```bash
# Build Docker image
make docker-build

# Deploy to Kubernetes
make k8s-deploy
```

## Usage

### gRPC Client Example

```go
import (
    pb "github.com/safabayar/gateway/proto"
    "google.golang.org/grpc"
)

conn, _ := grpc.Dial("gateway.safabayar.net:443", grpc.WithTransportCredentials(...))
client := pb.NewGatewayClient(conn)

req := &pb.CommandRequest{
    Fqdn:     "router1.myCustomer.safabayar.net",
    Username: "admin",
    Password: "password",
    Command:  "show version",
    Protocol: "ssh",
}

resp, _ := client.ExecuteCommand(context.Background(), req)
fmt.Println(resp.Output)
```

### SSH Bastion Access

```bash
# Connect to gateway with public key auth
ssh -i config/client_key user@gateway.safabayar.net

# From gateway shell, connect to device
ssh router1.myCustomer.safabayar.net
```

## Configuration

### Device Configuration (`config/devices.yaml`)

```yaml
devices:
  <device-name>:
    hostname: "<ip-or-hostname>"
    ssh_port: 22
    telnet_port: 23
    netconf_port: 830
    description: "<description>"
    location: "<location>"

settings:
  domain_suffix: "safabayar.net"
  default_timeout: 30
  max_sessions: 100
  log_level: "info"
```

### Command-Line Flags

- `--config`: Path to device configuration file (default: `config/devices.yaml`)
- `--log`: Path to log file (default: `logs/gateway.log`)
- `--grpc-port`: gRPC server port (default: `50051`)
- `--ssh-port`: SSH bastion port (default: `2222`)
- `--host-key`: Path to SSH host key (default: `config/ssh_host_key`)
- `--authorized-keys`: Path to authorized keys file (default: `config/authorized_keys`)

## Development

### Project Structure

```
.
├── cmd/gateway/          # Main application
├── internal/
│   ├── config/          # Configuration management
│   ├── grpc/            # gRPC server implementation
│   ├── logger/          # Logging utilities
│   ├── proxy/           # Protocol proxy implementations
│   └── ssh/             # SSH bastion server
├── proto/               # Protocol buffer definitions
├── config/              # Configuration files
├── k8s/                 # Kubernetes manifests
└── Makefile            # Build automation
```

### Build Commands

```bash
make help          # Show all available commands
make build         # Build binary
make test          # Run tests
make proto         # Generate protobuf code
make docker-build  # Build Docker image
make k8s-deploy    # Deploy to Kubernetes
```

## Security Considerations

1. **TLS**: TLS is terminated at the Gateway API layer. Ensure proper certificate management.
2. **SSH Keys**: Use strong SSH keys (ed25519 recommended). Never commit private keys to version control.
3. **Host Key Verification**: In production, implement proper SSH host key verification instead of `InsecureIgnoreHostKey`.
4. **Secrets Management**: Use Kubernetes secrets or external secret managers for credentials.
5. **Network Policies**: Implement Kubernetes network policies to restrict traffic.

## License

Copyright © 2024 Safabayar
