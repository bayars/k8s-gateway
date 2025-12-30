#!/bin/bash
# Deploy Gateway using Helm Chart with SR Linux Lab values
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HELM_CHART_DIR="${SCRIPT_DIR}/../../helm/gateway"
NAMESPACE="${NAMESPACE:-default}"
RELEASE_NAME="${RELEASE_NAME:-gateway}"

echo "================================================"
echo "  Deploying Gateway with Helm Chart"
echo "================================================"
echo ""
echo "Release: ${RELEASE_NAME}"
echo "Namespace: ${NAMESPACE}"
echo "Chart: ${HELM_CHART_DIR}"
echo ""

# Check if helm is installed
if ! command -v helm &> /dev/null; then
    echo "Error: helm is not installed"
    exit 1
fi

# Check if the chart exists
if [ ! -f "${HELM_CHART_DIR}/Chart.yaml" ]; then
    echo "Error: Helm chart not found at ${HELM_CHART_DIR}"
    exit 1
fi

# Create namespace if it doesn't exist
kubectl get namespace ${NAMESPACE} &> /dev/null || kubectl create namespace ${NAMESPACE}

# Deploy SR Linux topology first (if not already deployed)
if ! kubectl get pods -l app=srl1 -n ${NAMESPACE} &> /dev/null; then
    echo "Deploying SR Linux topology..."
    kubectl apply -f "${SCRIPT_DIR}/../clabernetes-topology-containerlab.yaml" -n ${NAMESPACE}
    echo "Waiting for SR Linux pods to be ready..."
    sleep 10
fi

# Install or upgrade the Helm release
echo ""
echo "Installing/Upgrading Helm release..."
helm upgrade --install ${RELEASE_NAME} ${HELM_CHART_DIR} \
    -f "${SCRIPT_DIR}/values-srlinux-lab.yaml" \
    -n ${NAMESPACE} \
    --wait \
    --timeout 5m

echo ""
echo "================================================"
echo "  Deployment Complete!"
echo "================================================"
echo ""

# Show release info
helm status ${RELEASE_NAME} -n ${NAMESPACE}

echo ""
echo "Getting service endpoints..."
kubectl get svc -l app.kubernetes.io/instance=${RELEASE_NAME} -n ${NAMESPACE}

echo ""
echo "Waiting for LoadBalancer IP..."
sleep 5

SSH_IP=$(kubectl get svc ${RELEASE_NAME}-ssh -n ${NAMESPACE} -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "pending")
GNMI_IP=$(kubectl get svc ${RELEASE_NAME}-gnmi -n ${NAMESPACE} -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "pending")

echo ""
echo "================================================"
echo "  Connection Information"
echo "================================================"
echo ""
echo "SSH Bastion:"
echo "  IP: ${SSH_IP}"
echo "  Command: ssh -i demo/ssh-keys/private/client_key -p 22 user@${SSH_IP}"
echo ""
echo "gNMI:"
echo "  IP: ${GNMI_IP}"
echo "  Command: gnmic -a ${GNMI_IP}:57400 --insecure \\"
echo "    --target srl1.safabayar.net:admin:NokiaSrl1! \\"
echo "    get --path /system/name --encoding json_ietf"
echo ""
echo "gRPC (port-forward):"
echo "  kubectl port-forward svc/${RELEASE_NAME}-grpc 50051:50051 -n ${NAMESPACE}"
echo ""
