# Quick Start Guide

This guide will help you get the multi-protocol gateway up and running quickly.

## Prerequisites

- Go 1.21+
- Docker (optional, for containerization)
- Kubernetes cluster (optional, for deployment)
- `protoc` compiler with Go plugins

## Step 1: Install Dependencies

```bash
# Install protoc plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Download Go dependencies
make deps
```

## Step 2: Generate Protobuf Code

```bash
make proto
```

This generates the gRPC client and server code from `proto/gateway.proto`.

## Step 3: Generate SSH Keys

```bash
make generate-keys
```

This creates:
- `config/ssh_host_key` - Server host key
- `config/client_key` - Client SSH key
- `config/authorized_keys` - List of authorized client public keys

## Step 4: Configure Devices

Edit `config/devices.yaml` to add your network devices:

```yaml
devices:
  myrouter:
    hostname: "192.168.1.1"
    ssh_port: 22
    telnet_port: 23
    netconf_port: 830
    description: "My Router"
    location: "Home Lab"

settings:
  domain_suffix: "safabayar.net"
  default_timeout: 30
  max_sessions: 100
  log_level: "info"
```

## Step 5: Build and Run

```bash
# Build the gateway
make build

# Run the gateway
./bin/gateway
```

Or use:

```bash
make run
```

The gateway will start:
- gRPC server on port 50051
- SSH bastion on port 2222

## Step 6: Test the Gateway

### Test gRPC

Build and run the example client:

```bash
cd examples
go build -o grpc_client grpc_client.go

./grpc_client \
  -server localhost:50051 \
  -fqdn myrouter.customer.safabayar.net \
  -username admin \
  -password yourpassword \
  -command "show version" \
  -protocol ssh
```

### Test SSH Bastion

```bash
# Connect to gateway
ssh -i config/client_key -p 2222 admin@localhost

# From gateway shell, connect to device
ssh myrouter.customer.safabayar.net
# Enter device password when prompted
```

## Step 7: Deploy to Kubernetes (Optional)

### Build Docker Image

```bash
make docker-build
```

### Create Kubernetes Secrets

```bash
# Create SSH keys secret
kubectl create secret generic gateway-ssh-keys \
  --from-file=ssh_host_key=config/ssh_host_key \
  --from-file=authorized_keys=config/authorized_keys

# Create TLS certificate secret (example with self-signed cert)
openssl req -x509 -newkey rsa:4096 -keyout tls.key -out tls.crt -days 365 -nodes
kubectl create secret tls gateway-tls-cert --cert=tls.crt --key=tls.key
```

### Deploy

```bash
make k8s-deploy
```

### Verify Deployment

```bash
# Check pods
kubectl get pods -l app=gateway

# Check services
kubectl get svc

# View logs
kubectl logs -f deployment/gateway
```

## Troubleshooting

### gRPC Connection Issues

```bash
# Check if gRPC server is listening
netstat -tuln | grep 50051

# Test with grpcurl
grpcurl -plaintext localhost:50051 list
```

### SSH Connection Issues

```bash
# Check SSH server is running
netstat -tuln | grep 2222

# Test SSH connection with verbose output
ssh -v -i config/client_key -p 2222 admin@localhost
```

### Device Connection Issues

```bash
# Test direct SSH to device
ssh admin@192.168.1.1

# Check gateway logs
tail -f logs/gateway.log
```

### Permission Errors

```bash
# Ensure SSH key permissions are correct
chmod 600 config/ssh_host_key
chmod 600 config/client_key
chmod 644 config/authorized_keys
```

## Next Steps

- Read [CLAUDE.md](CLAUDE.md) for detailed architecture and development guide
- Review [README.md](README.md) for comprehensive documentation
- Check [examples/README.md](examples/README.md) for more usage examples
- Configure production TLS certificates
- Set up monitoring and alerting
- Implement proper host key verification for production use
