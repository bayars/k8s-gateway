#!/bin/bash
# Cleanup Helm-deployed Gateway
set -e

NAMESPACE="${NAMESPACE:-default}"
RELEASE_NAME="${RELEASE_NAME:-gateway}"

echo "================================================"
echo "  Cleaning up Helm Demo"
echo "================================================"
echo ""

# Uninstall Helm release
echo "Uninstalling Helm release ${RELEASE_NAME}..."
helm uninstall ${RELEASE_NAME} -n ${NAMESPACE} 2>/dev/null || echo "Release not found or already removed"

# Optionally remove SR Linux topology
read -p "Remove SR Linux topology? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Removing SR Linux topology..."
    kubectl delete -f "$(dirname "$0")/../clabernetes-topology-containerlab.yaml" -n ${NAMESPACE} 2>/dev/null || echo "Topology not found"
fi

echo ""
echo "Cleanup complete!"
echo ""
