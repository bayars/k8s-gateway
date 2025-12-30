# Multi-Protocol Gateway

A multi-protocol gateway service that provides unified access to network devices through gRPC, SSH, Telnet, and NETCONF protocols. The gateway acts as a bastion/jump server with FQDN-based routing to backend devices.

## Features

- **Multi-Protocol Support**: gRPC, gNMI, SSH bastion, Telnet proxy, and NETCONF proxy
- **gNMI Proxy**: Native gnmic support for SR Linux telemetry and configuration
- **FQDN-Based Routing**: Route to devices using fully qualified domain names (e.g., `srl1.safabayar.net`)
- **Authentication**:
  - Client → Gateway: SSH public key authentication
  - gRPC/gNMI: Username/password in request body or metadata
  - Gateway → Device: Password-based authentication
- **Kubernetes Native**: Designed for deployment with Kubernetes Gateway API
- **Comprehensive Logging**: File-based and stdout logging with structured logs

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                                   CLIENTS                                        │
├──────────────────┬──────────────────┬──────────────────┬────────────────────────┤
│   SSH Client     │   gRPC Client    │   gNMI Client    │   NETCONF Client       │
│   (openssh)      │   (grpcurl)      │   (gnmic)        │   (ncclient)           │
└────────┬─────────┴────────┬─────────┴────────┬─────────┴───────────┬────────────┘
         │                  │                  │                     │
         │ :22              │ :50051           │ :57400              │ :830
         │ SSH Key Auth     │ plaintext        │ insecure            │
         ▼                  ▼                  ▼                     ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                            GATEWAY SERVICE                                       │
│                         (Single LoadBalancer IP)                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │                        FQDN-Based Router                                 │    │
│  │         srl1.safabayar.net  ──►  Extract device name: "srl1"            │    │
│  │         router1.lab.net     ──►  Extract device name: "router1"         │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                      │                                           │
│         ┌────────────────────────────┼────────────────────────────┐             │
│         ▼                            ▼                            ▼             │
│  ┌─────────────┐           ┌─────────────────┐           ┌─────────────┐        │
│  │ SSH Bastion │           │   gRPC Server   │           │ gNMI Proxy  │        │
│  │   Server    │           │                 │           │   Server    │        │
│  │             │           │  ┌───────────┐  │           │             │        │
│  │ Public Key  │           │  │SSH Proxy  │  │           │ x-gnmi-     │        │
│  │ Auth        │           │  │Telnet Prxy│  │           │ target      │        │
│  │             │           │  │NETCONF Prx│  │           │ header      │        │
│  └──────┬──────┘           │  └───────────┘  │           └──────┬──────┘        │
│         │                  └────────┬────────┘                  │               │
│         │                           │                           │               │
│  ┌──────┴───────────────────────────┴───────────────────────────┴──────┐        │
│  │                      Device Config Lookup                            │        │
│  │                       (devices.yaml)                                 │        │
│  │  ┌─────────────────────────────────────────────────────────────┐    │        │
│  │  │ srl1:                    │ srl2:                            │    │        │
│  │  │   hostname: 10.0.0.1     │   hostname: 10.0.0.2             │    │        │
│  │  │   ssh_port: 22           │   ssh_port: 22                   │    │        │
│  │  │   gnmi_port: 57400       │   gnmi_port: 57400               │    │        │
│  │  └─────────────────────────────────────────────────────────────┘    │        │
│  └─────────────────────────────────────────────────────────────────────┘        │
└─────────────────────────────────────────────────────────────────────────────────┘
                                       │
                                       │ Password Auth
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                            BACKEND NETWORK DEVICES                               │
│                                                                                  │
│  ┌──────────────────┐    ┌──────────────────┐    ┌──────────────────┐           │
│  │    SR Linux 1    │    │    SR Linux 2    │    │   Other Device   │           │
│  │   (10.0.0.1)     │    │   (10.0.0.2)     │    │   (10.0.0.x)     │           │
│  │                  │    │                  │    │                  │           │
│  │  SSH    :22      │    │  SSH    :22      │    │  SSH    :22      │           │
│  │  gNMI   :57400   │    │  gNMI   :57400   │    │  Telnet :23      │           │
│  │  NETCONF:830     │    │  NETCONF:830     │    │  NETCONF:830     │           │
│  └──────────────────┘    └──────────────────┘    └──────────────────┘           │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Protocol Flow

| Protocol | Client → Gateway | Gateway → Device | Use Case |
|----------|------------------|------------------|----------|
| **SSH** | Public key auth on port 22 | Password auth | Interactive shell, commands |
| **gRPC** | Plaintext on port 50051 | SSH/Telnet/NETCONF | API-driven automation |
| **gNMI** | Insecure on port 57400 | gNMI with TLS | Telemetry, config management |
| **NETCONF** | Via gRPC | SSH subsystem | XML-based config |

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

#### Option A: Using Helm (Recommended)

```bash
# Install from OCI registry
helm install gateway oci://ghcr.io/safabayar/charts/gateway

# Or with custom values
helm install gateway oci://ghcr.io/safabayar/charts/gateway \
  --set devices.entries.srl1.hostname=10.0.0.1 \
  --set service.type=LoadBalancer
```

#### Option B: Using Raw Manifests

```bash
# Build Docker image
make docker-build

# Deploy to Kubernetes
make k8s-deploy
```

### 5. Access the Gateway

```bash
# Get the LoadBalancer IP
export GATEWAY_IP=$(kubectl get svc gateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

# Test SSH
ssh -i config/client_key -p 22 user@$GATEWAY_IP

# Test gRPC
grpcurl -plaintext $GATEWAY_IP:50051 list

# Test gNMI
gnmic -a $GATEWAY_IP:57400 --insecure \
  --metadata "x-gnmi-target=srl1.safabayar.net:admin:password" \
  capabilities
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
- `--gnmi-port`: gNMI proxy port (default: `57400`)
- `--ssh-port`: SSH bastion port (default: `2222`)
- `--host-key`: Path to SSH host key (default: `config/ssh_host_key`)
- `--authorized-keys`: Path to authorized keys file (default: `config/authorized_keys`)

## Development

### Project Structure

```
.
├── cmd/gateway/          # Main application entry point
├── internal/
│   ├── config/          # Configuration management
│   ├── gnmi/            # gNMI proxy server
│   ├── grpc/            # gRPC server implementation
│   ├── logger/          # Logging utilities
│   ├── proxy/           # Protocol proxies (SSH, Telnet, NETCONF)
│   └── ssh/             # SSH bastion server
├── proto/               # Protocol buffer definitions
├── config/              # Configuration files (devices.yaml, keys)
├── helm/gateway/        # Helm chart for Kubernetes deployment
├── demo/                # Demo environments
│   ├── helm-demo/       # Helm chart demo with SR Linux
│   └── ssh-keys/        # Demo SSH keys
├── k8s/                 # Raw Kubernetes manifests
└── Makefile             # Build automation
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
