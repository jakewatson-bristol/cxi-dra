#!/usr/bin/env bash
# Sets up a local kind cluster with the CXI DRA driver in mock mode.
# Requires: kind, kubectl, docker OR podman

set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-cxi-dra-dev}"
IMAGE="${IMAGE:-localhost/cxi-dra-driver:dev}"
MOCK_DEVICES="${MOCK_DEVICES:-2}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
KUBECONFIG_PATH="${REPO_ROOT}/.kube/config"

# ---------------------------------------------------------------------------
# Detect container runtime (docker preferred, podman fallback)
# ---------------------------------------------------------------------------
if command -v docker &>/dev/null; then
  CONTAINER_CLI=docker
elif command -v podman &>/dev/null; then
  CONTAINER_CLI=podman
  # Tell kind to use podman as its node provider
  export KIND_EXPERIMENTAL_PROVIDER=podman
else
  echo "error: neither docker nor podman found in PATH" >&2
  exit 1
fi
echo "==> Using container runtime: ${CONTAINER_CLI}"

# ---------------------------------------------------------------------------
# Cluster
# ---------------------------------------------------------------------------
echo "==> Creating kind cluster '${CLUSTER_NAME}'"
kind create cluster \
  --name "${CLUSTER_NAME}" \
  --config "${SCRIPT_DIR}/kind-config.yaml" \
  --wait 60s

# ---------------------------------------------------------------------------
# Kubeconfig — write to .kube/config inside the repo
# ---------------------------------------------------------------------------
echo "==> Writing kubeconfig to ${KUBECONFIG_PATH}"
mkdir -p "$(dirname "${KUBECONFIG_PATH}")"
kind get kubeconfig --name "${CLUSTER_NAME}" > "${KUBECONFIG_PATH}"
chmod 600 "${KUBECONFIG_PATH}"
export KUBECONFIG="${KUBECONFIG_PATH}"
echo "    export KUBECONFIG=${KUBECONFIG_PATH}"

# ---------------------------------------------------------------------------
# Build and load image
# ---------------------------------------------------------------------------
echo "==> Generating go.sum"
(cd "${REPO_ROOT}" && go mod tidy)

echo "==> Building driver image with ${CONTAINER_CLI}"
"${CONTAINER_CLI}" build --no-cache -t "${IMAGE}" "${REPO_ROOT}"

echo "==> Loading image into kind"
if [[ "${CONTAINER_CLI}" == "podman" ]]; then
  # kind's podman provider can pull directly from podman's image store
  "${CONTAINER_CLI}" save "${IMAGE}" -o /tmp/cxi-dra-image.tar
  kind load image-archive /tmp/cxi-dra-image.tar --name "${CLUSTER_NAME}"
  rm -f /tmp/cxi-dra-image.tar
else
  kind load docker-image "${IMAGE}" --name "${CLUSTER_NAME}"
fi

# ---------------------------------------------------------------------------
# Deploy
# ---------------------------------------------------------------------------
echo "==> Applying RBAC and DeviceClass"
kubectl apply -f "${REPO_ROOT}/deploy/kubernetes/clusterrole.yaml"
kubectl apply -f "${REPO_ROOT}/deploy/kubernetes/deviceclass.yaml"

echo "==> Patching DaemonSet for mock mode (${MOCK_DEVICES} devices, no nodeSelector)"
sed \
  -e "s|__IMAGE__|${IMAGE}|g" \
  -e "s|__MOCK_DEVICES__|${MOCK_DEVICES}|g" \
  "${SCRIPT_DIR}/daemonset-mock.yaml" \
  | kubectl apply -f -

echo "==> Waiting for DaemonSet to be ready"
kubectl -n kube-system rollout status daemonset/cxi-dra-driver --timeout=90s

echo "==> ResourceSlices published:"
kubectl get resourceslices -o wide

# ---------------------------------------------------------------------------
# Remind the user about .envrc
# ---------------------------------------------------------------------------
echo ""
echo "Done."
echo ""
echo "Your kubeconfig is at: ${KUBECONFIG_PATH}"
echo "Run 'direnv allow' in the repo root to activate it automatically via .envrc."
echo ""
echo "Run the test workload:"
echo "  kubectl apply -f ${REPO_ROOT}/deploy/kubernetes/test/"
echo ""
echo "Tear down:"
echo "  kind delete cluster --name ${CLUSTER_NAME}"
