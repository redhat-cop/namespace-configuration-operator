#!/bin/bash
# Simple utility to create Docker Hub registry secret
# Usage: ./local-utilities/create-dockerhub-secret.sh

set -e

SECRET_NAME="dockerhub-secret"
NAMESPACE="namespace-configuration-operator"

# Get credentials from environment or prompt
DOCKERHUB_USERNAME="${DOCKERHUB_USERNAME:-}"
DOCKERHUB_PASSWORD="${DOCKERHUB_PASSWORD:-}"
DOCKERHUB_EMAIL="${DOCKERHUB_EMAIL:-}"

if [ -z "$DOCKERHUB_USERNAME" ]; then
    read -p "Docker Hub Username: " DOCKERHUB_USERNAME
fi

if [ -z "$DOCKERHUB_PASSWORD" ]; then
    read -s -p "Docker Hub Password: " DOCKERHUB_PASSWORD
    echo ""
fi

if [ -z "$DOCKERHUB_EMAIL" ]; then
    DOCKERHUB_EMAIL="${DOCKERHUB_USERNAME}@example.com"
fi

# Create namespace if it doesn't exist
oc get namespace "$NAMESPACE" &> /dev/null || oc create namespace "$NAMESPACE"

# Delete existing secret if it exists
oc delete secret "$SECRET_NAME" -n "$NAMESPACE" 2>/dev/null || true

# Create the secret
oc create secret docker-registry "$SECRET_NAME" \
    --docker-server=docker.io \
    --docker-username="$DOCKERHUB_USERNAME" \
    --docker-password="$DOCKERHUB_PASSWORD" \
    --docker-email="$DOCKERHUB_EMAIL" \
    -n "$NAMESPACE"

echo "✅ Secret '$SECRET_NAME' created in namespace '$NAMESPACE'"
