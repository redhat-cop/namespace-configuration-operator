#!/bin/bash

set -e

SCRIPT_BASE_DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )

command -v yq >/dev/null 2>&1 || { echo "yq is not installed. Aborting."; exit 1; }

command -v skopeo >/dev/null 2>&1 || { echo "skopeo is not installed. Aborting."; exit 1; }

if [[ ! ${QUAY_REGISTRY} ]] || [[ ! ${OPERATOR_IMAGE_TAG} ]] || [[ ! ${REPOSITORY_NAME} ]]; then echo "Required Environment Variables Not Provided. Aborting"; exit 1; fi

CSV_FILE="${SCRIPT_BASE_DIR}/../bundle/manifests/${REPOSITORY_NAME}.clusterserviceversion.yaml"

if [ ! -f "${CSV_FILE}" ]; then
  echo "Bundle Directory Not Present. Aborting"
  exit 1
fi

CREATED_TIME=`date +"%FT%H:%M:%SZ"`
OPERATOR_IMAGE_DIGEST=$(skopeo inspect docker://quay.io/${QUAY_REGISTRY}:${OPERATOR_IMAGE_TAG} | jq -r ".Digest")
OPERATOR_IMAGE="quay.io/${QUAY_REGISTRY}@${OPERATOR_IMAGE_DIGEST}"
KUBE_RBAC_PROXY_IMAGE_DIGEST=$(skopeo inspect docker://$(yq r bundle/manifests/${REPOSITORY_NAME}.clusterserviceversion.yaml 'spec.install.spec.deployments[0].spec.template.spec.containers[0].image') | jq -r ".Digest")
KUBE_RBAC_PROXY_IMAGE="$(yq r bundle/manifests/${REPOSITORY_NAME}.clusterserviceversion.yaml 'spec.install.spec.deployments[0].spec.template.spec.containers[0].image' | cut -d':' -f1)@${KUBE_RBAC_PROXY_IMAGE_DIGEST}"

yq write --inplace "${CSV_FILE}" 'metadata.annotations.containerImage' ${OPERATOR_IMAGE}

yq write --inplace "${CSV_FILE}" 'metadata.annotations.createdAt' ${CREATED_TIME}

cat << EOF | yq write --inplace --script - "${CSV_FILE}"
- command: update
  path: spec.relatedImages[+]
  value:
    name: "${QUAY_REGISTRY}"
    image: "${OPERATOR_IMAGE}"
EOF

cat << EOF | yq write --inplace --script - "${CSV_FILE}"
- command: update
  path: spec.relatedImages[+]
  value:
    name: "kube-rbac-proxy"
    image: "${KUBE_RBAC_PROXY_IMAGE}"
EOF

yq write --inplace "${CSV_FILE}" "spec.install.spec.deployments[0].spec.template.spec.containers[0].image" "${KUBE_RBAC_PROXY_IMAGE}"
yq write --inplace "${CSV_FILE}" "spec.install.spec.deployments[0].spec.template.spec.containers[1].image" "${OPERATOR_IMAGE}"
yq write --inplace "${CSV_FILE}" "metadata.annotations.containerImage" "${OPERATOR_IMAGE}"
