#!/bin/bash
# Generate Kyverno policies from templates using envsubst
# Usage: ./local-utilities/generate-policies.sh [DOCKERHUB_USERNAME]
#
# Example:
#   export DOCKERHUB_USERNAME=my-username
#   ./local-utilities/generate-policies.sh
#
#   OR
#
#   ./local-utilities/generate-policies.sh my-username

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT/kyverno-policies"

# Get Docker Hub username from argument or environment variable
DOCKERHUB_USERNAME="${1:-${DOCKERHUB_USERNAME}}"

if [ -z "$DOCKERHUB_USERNAME" ]; then
    echo "Error: DOCKERHUB_USERNAME not set"
    echo ""
    echo "Usage:"
    echo "  export DOCKERHUB_USERNAME=your-username"
    echo "  $0"
    echo ""
    echo "  OR"
    echo ""
    echo "  $0 your-username"
    exit 1
fi

echo "Generating Kyverno policies with DOCKERHUB_USERNAME=${DOCKERHUB_USERNAME}"
echo ""

# Generate policies from templates
# Note: This processes env-*.yaml.tpl files which may include:
# - Docker Hub username substitution (DOCKERHUB_USERNAME)
# - Log level configuration (ZAP_LOG_LEVEL, ZAP_DEVEL)
for template in env-*.yaml.tpl; do
    if [ ! -f "$template" ]; then
        echo "No template files found (env-*.yaml.tpl)"
        continue
    fi
    
    # Extract output filename (remove .tpl and env- prefix)
    output_file=$(echo "$template" | sed 's/^env-//' | sed 's/\.tpl$//')
    
    echo "  Generating: $output_file"
    export DOCKERHUB_USERNAME
    envsubst < "$template" > "$output_file"
    
    # Verify the replacement worked
    if grep -q '\${DOCKERHUB_USERNAME}' "$output_file" 2>/dev/null; then
        echo "    ⚠️  Warning: Some placeholders may not have been replaced"
    else
        echo "    ✅ Success"
    fi
done

echo ""
echo "Generated policies:"
ls -1 env-*.yaml.tpl 2>/dev/null | sed 's/^env-//' | sed 's/\.tpl$//' | while read file; do
    if [ -f "$file" ]; then
        echo "  - $file"
    fi
done

echo ""
echo "To apply policies (run from repository root):"
echo "  oc apply -f kyverno-policies/$(ls -1 env-*.yaml.tpl 2>/dev/null | sed 's/^env-//' | sed 's/\.tpl$//' | head -1)"
echo ""
echo "Or apply all generated policies:"
echo "  oc apply -f kyverno-policies/"

