#!/bin/bash
# Update the gateway-authorized-keys ConfigMap with all public keys from the public/ directory
# Usage: ./update-authorized-keys.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PUBLIC_DIR="$SCRIPT_DIR/public"

if [ ! -d "$PUBLIC_DIR" ]; then
    echo "Error: public/ directory not found"
    exit 1
fi

# Collect all public keys into a temp file
TMPFILE=$(mktemp)
for keyfile in "$PUBLIC_DIR"/*.pub; do
    if [ -f "$keyfile" ]; then
        keyname=$(basename "$keyfile" .pub)
        echo "Adding key: $keyname"
        echo "# $keyname" >> "$TMPFILE"
        cat "$keyfile" >> "$TMPFILE"
    fi
done

if [ ! -s "$TMPFILE" ]; then
    echo "Error: No public keys found in $PUBLIC_DIR"
    rm -f "$TMPFILE"
    exit 1
fi

# Create ConfigMap from file
echo ""
echo "Patching gateway-authorized-keys ConfigMap..."
kubectl create configmap gateway-authorized-keys \
    --from-file=authorized_keys="$TMPFILE" \
    --dry-run=client -o yaml | kubectl apply -f -

rm -f "$TMPFILE"

echo ""
echo "Done! Keys will be reloaded automatically within ~60 seconds."
echo "Check logs: kubectl logs -l app=gateway --tail=10 | grep -i authorized"
