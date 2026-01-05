# Gateway Helm Chart

A Helm chart for deploying the multi-protocol network gateway that provides unified access to network devices via gRPC, gNMI, SSH bastion, and NETCONF.

## Overview

The Gateway acts as a centralized access point for network automation, supporting:

| Protocol | Port | Description |
|----------|------|-------------|
| gRPC | 50051 | Command execution API |
| gNMI | 57400 | OpenConfig telemetry and configuration |
| SSH | 22 (external) → 2222 (internal) | Bastion/jump server |
| NETCONF | 830 | XML-based configuration |

## Prerequisites

- Kubernetes 1.21+
- Helm 3.0+
- Network devices accessible from the cluster
- SSH host key for bastion authentication

## Installation

### Quick Start

```bash
# Add the repository (if using Harbor)
helm repo add gateway https://harbor.example.com/chartrepo/library
helm repo update

# Install with default values
helm install gateway gateway/gateway

# Install with custom values file
helm install gateway gateway/gateway -f values-custom.yaml
```

### From Local Chart

```bash
# Clone the repository
git clone https://github.com/safabayar/gateway.git
cd gateway

# Install from local chart
helm install gateway ./helm/gateway -f helm/gateway/values.yaml
```

## Configuration

### Key Configuration Sections

#### Image Configuration

```yaml
image:
  repository: ghcr.io/safabayar/gateway
  tag: "latest"
  pullPolicy: Always
```

#### Gateway Ports

```yaml
gateway:
  logLevel: "info"      # debug, info, warn, error
  grpcPort: 50051
  gnmiPort: 57400
  sshPort: 2222         # Internal port (mapped to 22 externally)
  netconfPort: 830
```

#### Device Configuration

Devices are configured via the `devices.entries` map. The key must match the first subdomain of the FQDN used to access the device.

```yaml
devices:
  domainSuffix: "safabayar.net"
  defaultTimeout: 30
  maxSessions: 100

  entries:
    # Access via: srl1.safabayar.net
    srl1:
      hostname: "srl1.default.svc.cluster.local"
      sshPort: 22
      telnetPort: 23
      netconfPort: 830
      gnmiPort: 57400
      description: "SR Linux Node 1"
      location: "Lab"

    # Access via: router1.safabayar.net
    router1:
      hostname: "10.0.0.1"
      sshPort: 22
      telnetPort: 23
      netconfPort: 830
      gnmiPort: 57400
```

#### SSH Configuration

```yaml
ssh:
  # Generate: ssh-keygen -t ed25519 -f ssh_host_key -N "" && base64 -w 0 ssh_host_key
  hostKey: ""

  # Or use existing secret
  existingHostKeySecret: "my-ssh-secret"
  hostKeySecretKey: "ssh_host_key"

  # Authorized public keys for client authentication
  authorizedKeys:
    - "ssh-ed25519 AAAAC3Nza... user@host"
    - "ssh-rsa AAAAB3Nza... another@host"

  # Or use existing ConfigMap
  existingAuthorizedKeysConfigMap: "my-authorized-keys"
```

#### Service Configuration

```yaml
service:
  type: LoadBalancer  # ClusterIP, NodePort, LoadBalancer
  loadBalancerIP: ""  # Optional static IP

  annotations:
    metallb.universe.tf/loadBalancerIPs: "10.0.0.50"

  grpc:
    enabled: true
    port: 50051
  gnmi:
    enabled: true
    port: 57400
  ssh:
    enabled: true
    port: 22
  netconf:
    enabled: true
    port: 830
```

#### Gateway API (Optional)

Enable Kubernetes Gateway API for advanced routing:

```yaml
gatewayAPI:
  enabled: true
  gatewayClassName: "istio"
  hostname: "*.safabayar.net"

  tls:
    enabled: true
    existingSecret: "gateway-tls"

  grpcRoute:
    enabled: true
  sshRoute:
    enabled: true
```

### All Configuration Options

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of gateway replicas | `2` |
| `image.repository` | Container image repository | `ghcr.io/safabayar/gateway` |
| `image.tag` | Container image tag | `latest` |
| `image.pullPolicy` | Image pull policy | `Always` |
| `gateway.logLevel` | Log level (debug/info/warn/error) | `info` |
| `gateway.grpcPort` | gRPC server port | `50051` |
| `gateway.gnmiPort` | gNMI server port | `57400` |
| `gateway.sshPort` | SSH bastion port | `2222` |
| `gateway.netconfPort` | NETCONF port | `830` |
| `devices.domainSuffix` | Domain suffix for FQDNs | `safabayar.net` |
| `devices.defaultTimeout` | Default operation timeout (seconds) | `30` |
| `devices.maxSessions` | Maximum concurrent sessions | `100` |
| `devices.entries` | Device routing table | `{}` |
| `ssh.hostKey` | SSH host key (base64 encoded) | `""` |
| `ssh.existingHostKeySecret` | Existing secret with host key | `""` |
| `ssh.authorizedKeys` | List of authorized public keys | `[]` |
| `service.type` | Service type | `LoadBalancer` |
| `service.loadBalancerIP` | Static LoadBalancer IP | `""` |
| `resources.requests.memory` | Memory request | `256Mi` |
| `resources.requests.cpu` | CPU request | `250m` |
| `resources.limits.memory` | Memory limit | `512Mi` |
| `resources.limits.cpu` | CPU limit | `500m` |
| `autoscaling.enabled` | Enable HPA | `false` |
| `autoscaling.minReplicas` | Minimum replicas | `2` |
| `autoscaling.maxReplicas` | Maximum replicas | `10` |
| `podDisruptionBudget.enabled` | Enable PDB | `false` |

## Example Deployments

### Demo: SR Linux Lab

Deploy for `clabernetes-topology-containerlab.yaml` topology:

```bash
helm install gateway ./helm/gateway -f helm/gateway/values-srlinux-demo.yaml
```

This configures:
- 2 SR Linux nodes (srl1, srl2) in default namespace
- ClusterIP service
- Minimal resources for demo

### Demo: DC2 Datacenter

Deploy for `dc2-topology.yaml` topology:

```bash
helm install gateway ./helm/gateway -f helm/gateway/values-dc2.yaml -n customerb
```

This configures:
- 2 SR Linux spines
- 4 FRR leaves
- 4 traffic generators
- All devices in customerb namespace

### Production Deployment

```bash
helm install gateway ./helm/gateway \
  --set replicaCount=3 \
  --set service.type=LoadBalancer \
  --set autoscaling.enabled=true \
  --set podDisruptionBudget.enabled=true \
  --set ssh.existingHostKeySecret=prod-ssh-key \
  -f values-production.yaml
```

## Usage

### Get Gateway Endpoint

```bash
# For LoadBalancer
export GATEWAY_IP=$(kubectl get svc gateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

# For ClusterIP (port-forward)
kubectl port-forward svc/gateway 50051:50051 57400:57400 22:22
```

### SSH Bastion Access

```bash
# Connect to a device through the gateway
ssh -i ~/.ssh/my_key -p 22 -o ProxyCommand="ssh -i ~/.ssh/my_key -p 22 -W %h:%p user@$GATEWAY_IP" admin@router1.safabayar.net

# Or direct gateway access
ssh -i ~/.ssh/my_key -p 22 user@$GATEWAY_IP
```

### gRPC Command Execution

```bash
grpcurl -plaintext -d '{
  "fqdn": "srl1.safabayar.net",
  "username": "admin",
  "password": "NokiaSrl1!",
  "command": "show version",
  "protocol": "ssh"
}' $GATEWAY_IP:50051 gateway.Gateway/ExecuteCommand
```

### gNMI Operations

```bash
# Capabilities
gnmic -a $GATEWAY_IP:57400 --insecure \
  --metadata "x-gnmi-target=srl1.safabayar.net:admin:NokiaSrl1!" \
  capabilities

# Get configuration
gnmic -a $GATEWAY_IP:57400 --insecure \
  --metadata "x-gnmi-target=srl1.safabayar.net:admin:NokiaSrl1!" \
  get --path "/system/name"

# Subscribe to telemetry
gnmic -a $GATEWAY_IP:57400 --insecure \
  --metadata "x-gnmi-target=srl1.safabayar.net:admin:NokiaSrl1!" \
  subscribe --path "/interface/statistics"
```

## Upgrading

```bash
# Update device configuration
helm upgrade gateway ./helm/gateway \
  --set devices.entries.newdevice.hostname=10.0.0.100

# Update authorized keys
helm upgrade gateway ./helm/gateway \
  --set ssh.authorizedKeys[0]="ssh-ed25519 AAAA..."

# Full upgrade with new values
helm upgrade gateway ./helm/gateway -f values-new.yaml
```

## Troubleshooting

### View Logs

```bash
kubectl logs -f deployment/gateway
```

### Check Pod Status

```bash
kubectl get pods -l app.kubernetes.io/name=gateway
kubectl describe pod -l app.kubernetes.io/name=gateway
```

### Verify Configuration

```bash
# Check ConfigMap
kubectl get configmap gateway-config -o yaml

# Check Secret
kubectl get secret gateway-ssh-host-key -o yaml
```

### Common Issues

| Issue | Solution |
|-------|----------|
| SSH connection refused | Verify authorized_keys ConfigMap contains your public key |
| Device not found | Check device name matches FQDN subdomain in devices.entries |
| gNMI timeout | Verify device hostname is reachable from gateway pod |
| TLS errors | Ensure TLS certificates are properly configured in Gateway API |

## Uninstalling

```bash
helm uninstall gateway
```

## License

MIT License - See [LICENSE](../../LICENSE) for details.
