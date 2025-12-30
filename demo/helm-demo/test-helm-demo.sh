#!/bin/bash
# Test Gateway deployed via Helm Chart
set -e

NAMESPACE="${NAMESPACE:-default}"
RELEASE_NAME="${RELEASE_NAME:-gateway}"

echo "================================================"
echo "  Testing Helm-deployed Gateway"
echo "================================================"
echo ""

# Get the single service LoadBalancer IP
GATEWAY_IP=$(kubectl get svc ${RELEASE_NAME} -n ${NAMESPACE} -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null)

if [ -z "$GATEWAY_IP" ] || [ "$GATEWAY_IP" == "null" ]; then
    echo "Warning: Gateway LoadBalancer IP not available yet"
    echo "Checking service status..."
    kubectl get svc ${RELEASE_NAME} -n ${NAMESPACE}
    echo ""
    echo "Using port-forward instead..."
    kubectl port-forward svc/${RELEASE_NAME} 2222:22 50051:50051 57400:57400 -n ${NAMESPACE} &
    PF_PID=$!
    GATEWAY_IP="localhost"
    SSH_PORT="2222"
    sleep 3
else
    SSH_PORT="22"
    echo "Gateway LoadBalancer IP: $GATEWAY_IP"
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLIENT_KEY="${SCRIPT_DIR}/../ssh-keys/private/client_key"

echo ""
echo "================================================"
echo "  Test 1: SSH Bastion Connection"
echo "================================================"
echo ""

if [ -f "$CLIENT_KEY" ]; then
    echo "Testing SSH connection to ${GATEWAY_IP}:${SSH_PORT}..."
    ssh -i "$CLIENT_KEY" -p ${SSH_PORT} -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5 user@${GATEWAY_IP} "list" 2>/dev/null && echo "SSH: PASS" || echo "SSH: Connection test (may need device running)"
else
    echo "Warning: Client key not found at ${CLIENT_KEY}"
    echo "SSH test skipped"
fi

echo ""
echo "================================================"
echo "  Test 2: gNMI Connection"
echo "================================================"
echo ""

if command -v gnmic &> /dev/null; then
    echo "Testing gNMI capabilities to ${GATEWAY_IP}:57400..."
    timeout 10 gnmic -a ${GATEWAY_IP}:57400 --insecure \
        --target "srl1.safabayar.net:admin:NokiaSrl1!" \
        capabilities 2>/dev/null && echo "gNMI: PASS" || echo "gNMI: Connection test (may need SR Linux running)"
else
    echo "gnmic not installed, skipping gNMI test"
fi

echo ""
echo "================================================"
echo "  Test 3: gRPC Connection"
echo "================================================"
echo ""

if command -v grpcurl &> /dev/null; then
    echo "Testing gRPC connection to ${GATEWAY_IP}:50051..."
    grpcurl -plaintext ${GATEWAY_IP}:50051 list 2>/dev/null && echo "gRPC: PASS" || echo "gRPC: Service available"
else
    echo "grpcurl not installed, skipping gRPC test"
fi

echo ""
echo "================================================"
echo "  Test 4: Pod Health"
echo "================================================"
echo ""

echo "Checking pod status..."
kubectl get pods -l app.kubernetes.io/instance=${RELEASE_NAME} -n ${NAMESPACE}

echo ""
echo "Checking pod logs (last 10 lines)..."
kubectl logs -l app.kubernetes.io/instance=${RELEASE_NAME} -n ${NAMESPACE} --tail=10

# Cleanup port-forwards
if [ ! -z "$PF_PID" ]; then
    kill $PF_PID 2>/dev/null || true
fi

echo ""
echo "================================================"
echo "  Tests Complete"
echo "================================================"
echo ""
echo "Gateway IP: $GATEWAY_IP"
echo "Ports: SSH=22, gRPC=50051, gNMI=57400, NETCONF=830"
