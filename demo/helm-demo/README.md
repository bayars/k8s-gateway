# Gateway Helm Chart Demo

This demo shows how to deploy the Multi-Protocol Gateway using the Helm chart.

## Prerequisites

- Kubernetes cluster
- Helm 3.x installed
- `kubectl` configured
- Optional: `gnmic`, `grpcurl` for testing

## Quick Start

### 1. Deploy SR Linux Lab

```bash
# Deploy the gateway with SR Linux lab configuration
./deploy-helm-demo.sh
```

This will:
1. Deploy the SR Linux topology (if not present)
2. Install the gateway Helm chart with lab values
3. Display connection information

### 2. Test the Deployment

```bash
./test-helm-demo.sh
```

### 3. Cleanup

```bash
./cleanup-helm-demo.sh
```

## Manual Installation

### Install with Default Values

```bash
helm install gateway ../../helm/gateway -n default
```

### Install with Custom Values

```bash
# SR Linux Lab
helm install gateway ../../helm/gateway -f values-srlinux-lab.yaml

# Minimal configuration
helm install gateway ../../helm/gateway -f values-minimal.yaml

# Production configuration
helm install gateway ../../helm/gateway -f values-production.yaml -n gateway --create-namespace
```

### Override Values on Command Line

```bash
# Add a device
helm install gateway ../../helm/gateway \
  --set devices.entries.myrouter.hostname=10.0.0.1 \
  --set devices.entries.myrouter.sshPort=22 \
  --set devices.entries.myrouter.gnmiPort=57400

# Change replica count
helm install gateway ../../helm/gateway \
  --set replicaCount=3

# Add SSH authorized key
helm install gateway ../../helm/gateway \
  --set 'ssh.authorizedKeys[0]=ssh-ed25519 AAAA... user@host'
```

## Value Files

| File | Description |
|------|-------------|
| `values-srlinux-lab.yaml` | SR Linux lab with 2 nodes, LoadBalancer services |
| `values-minimal.yaml` | Minimal config for quick testing |
| `values-production.yaml` | Production-ready with HA, autoscaling, security |

## Key Configuration Options

### Devices

```yaml
devices:
  domainSuffix: "safabayar.net"
  entries:
    device1:
      hostname: "10.0.0.1"
      sshPort: 22
      gnmiPort: 57400
```

### SSH Configuration

```yaml
ssh:
  # Option 1: Inline host key (base64 encoded)
  hostKey: "LS0tLS1CRUdJ..."

  # Option 2: Use existing secret
  existingHostKeySecret: "my-ssh-secret"

  # Authorized keys
  authorizedKeys:
    - "ssh-ed25519 AAAA... user@host"
```

### Services

```yaml
services:
  ssh:
    type: LoadBalancer  # or ClusterIP, NodePort
    port: 22
    annotations:
      metallb.universe.tf/loadBalancerIPs: "10.0.0.50"
```

### Autoscaling

```yaml
autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70
```

## Upgrade

```bash
# Update devices
helm upgrade gateway ../../helm/gateway \
  --set devices.entries.newdevice.hostname=10.0.0.2

# Update image
helm upgrade gateway ../../helm/gateway \
  --set image.tag=v1.1.0

# Apply new values file
helm upgrade gateway ../../helm/gateway -f values-production.yaml
```

## Testing the Gateway

After deployment, test the gateway using the single LoadBalancer IP:

```bash
# Get the Gateway IP
export GATEWAY_IP=$(kubectl get svc gateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

# Test SSH bastion
ssh -i ../ssh-keys/private/client_key -p 22 user@$GATEWAY_IP "list"

# Test gRPC
grpcurl -plaintext $GATEWAY_IP:50051 list

# Test gNMI (requires gnmic)
gnmic -a $GATEWAY_IP:57400 --insecure \
  --metadata "x-gnmi-target=srl1.safabayar.net:admin:NokiaSrl1!" \
  capabilities
```

## Troubleshooting

### Check Deployment Status

```bash
helm status gateway
kubectl get pods -l app.kubernetes.io/instance=gateway
```

### View Logs

```bash
kubectl logs -l app.kubernetes.io/instance=gateway -f
```

### Check ConfigMaps

```bash
kubectl get configmap -l app.kubernetes.io/instance=gateway
kubectl describe configmap gateway-config
```

### Debug Template Rendering

```bash
helm template gateway ../../helm/gateway -f values-srlinux-lab.yaml
```
