# cxi-dra-driver

A Kubernetes [Dynamic Resource Allocation (DRA)](https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/) node driver for HPE Slingshot CXI (Cassini eXpress Interface) PCIe devices.

The driver runs as a DaemonSet on each Slingshot-equipped node. It discovers CXI devices via sysfs, publishes them as a `ResourceSlice` so the scheduler can allocate them to pods, and injects each device into its container via CDI (Container Device Interface).

## Architecture

```
Scheduler ──allocates──► ResourceSlice (cxi0, cxi1, …)
                              │
                         DaemonSet (this driver)
                         ├── discovers /sys/class/cxi_user/*
                         ├── pairs CXI↔HSN via sysfs PCIe topology
                         ├── writes CDI specs → /var/run/cdi/
                         └── NodePrepareResources → returns CDI device ID
                                                        │
                                                   Container runtime
                                                   mounts /dev/cxi0
                                                   + sysfs subtree
```

## Requirements

- Kubernetes 1.34+ (stable `resource.k8s.io/v1` DRA API)
- Nodes labelled `feature.node.kubernetes.io/cxi=true`
- Container runtime with CDI support (containerd ≥ 1.7 or CRI-O ≥ 1.28)
- Go 1.23+ (to build)

## Quick Start

### 1. Build and push the image

```bash
make image IMAGE=<your-registry>/cxi-dra-driver TAG=v0.1.0
make push  IMAGE=<your-registry>/cxi-dra-driver TAG=v0.1.0
```

Update the image reference in [deploy/kubernetes/daemonset.yaml](deploy/kubernetes/daemonset.yaml).

### 2. Label Slingshot nodes

```bash
kubectl label node <node-name> feature.node.kubernetes.io/cxi=true
```

### 3. Deploy the driver

```bash
make deploy
```

This applies RBAC, the `DeviceClass`, and the `DaemonSet` in order.

### 4. Verify the ResourceSlice

```bash
kubectl get resourceslices -o wide
```

You should see one slice per node, with devices `cxi0`, `cxi1`, … and attributes `numa.node`, `pcie.root`, `hsn.iface`.

### 5. Run the test pod

```bash
make test-deploy
kubectl get pods cxi-test
kubectl logs cxi-test
```

The pod logs will list `/dev/cxi*` visible inside the container. A second pod will receive a different device; a third pod goes `Pending` if the pool is exhausted.

```bash
make test-clean   # remove test resources
```

## Project Layout

```
cmd/cxi-dra-driver/      Binary entrypoint
internal/cxi/            Sysfs device discovery and CXI↔HSN pairing
internal/cdi/            CDI spec generation
internal/driver/         DRA kubelet plugin (ResourceSlice, NodePrepare/Unprepare)
deploy/kubernetes/       DaemonSet, DeviceClass, RBAC, test manifests
```

## Configuration

| Flag | Default | Description |
|---|---|---|
| `--node-name` | `$NODE_NAME` | Kubernetes node name |
| `--driver-name` | `cxi.slingshot.hpe.com` | DRA driver identifier |
| `--cdi-root` | `/var/run/cdi` | CDI spec directory |
| `--plugin-socket` | `/var/lib/kubelet/plugins/cxi.slingshot.hpe.com/plugin.sock` | Kubelet plugin socket |
| `--registrar-socket` | `/var/lib/kubelet/plugins_registry/cxi.slingshot.hpe.com.sock` | Plugin registration socket |
| `--kubeconfig` | *(in-cluster)* | Path to kubeconfig for out-of-cluster use |

## Device Attributes

Each device in the `ResourceSlice` exposes the following attributes for use in `DeviceClass` selectors or `ResourceClaim` constraints:

| Attribute | Type | Example | Source |
|---|---|---|---|
| `numa_node` | int | `1` | `/sys/class/cxi_user/cxi0/device/numa_node` |
| `pcie_root` | string | `"0000:81:00.0"` | `uevent PCI_SLOT_NAME` |
| `hsn_iface` | string | `"hsn2"` | sysfs PCIe topology walk |

Example — claim a CXI device on NUMA node 1:

```yaml
apiVersion: resource.k8s.io/v1
kind: ResourceClaimTemplate
metadata:
  name: cxi-numa1
spec:
  spec:
    devices:
      requests:
        - name: cxi
          deviceClassName: cxi-device
          selectors:
            - cel:
                expression: "device.attributes['numa_node'].int == 1"
```

## Scope and Deferred Work

This is an initial implementation covering CXI device allocation and CDI injection.

Not yet implemented (planned):
- **VNI pool** — cluster-wide `ResourceSlice` with `allNodes: true` advertising a VNI range (`deviceClassName: cxi-vni`)
- **CXI service programming** — `NodePrepareResources` querying CRI for the pod netns inode and calling the CXI kernel interface to bind VNI + netns
- **CXI service teardown** — `NodeUnprepareResources` removing the service entry
- **Admission webhook** — validating that any pod claiming `cxi-vni` also claims a `cxi-device`
- **Hot-plug** — fsnotify watcher on `/sys/class/cxi_user` to update `ResourceSlice` when devices appear or disappear

## Local Testing with kind

The driver supports a `--mock-devices=N` flag that generates N synthetic CXI
devices without requiring real hardware. `/dev/null` is used as the device node
stand-in (always present in any container), and temporary sysfs directories are
created under `/tmp/mock-cxi/`. The full scheduling → allocation →
NodePrepareResources loop runs normally.

### Prerequisites

```bash
# kind 0.24+ and a Kubernetes 1.34-capable node image
brew install kind kubectl
```

### One-command setup

```bash
./deploy/kind/setup.sh
```

This creates a two-node kind cluster with CDI enabled, builds the driver image,
loads it into kind, and deploys in mock mode with 2 synthetic devices. When it
finishes you'll see the ResourceSlices and can run the test pod immediately.

### Manual steps (if you prefer)

```bash
# 1. Create cluster
kind create cluster --name cxi-dra-dev --config deploy/kind/kind-config.yaml

# 2. Build and load image
docker build -t cxi-dra-driver:dev .
kind load docker-image cxi-dra-driver:dev --name cxi-dra-dev

# 3. Deploy (without nodeSelector, with mock flag)
kubectl apply -f deploy/kubernetes/clusterrole.yaml
kubectl apply -f deploy/kubernetes/deviceclass.yaml
# Edit daemonset.yaml: add --mock-devices=2 and remove nodeSelector, then:
kubectl apply -f deploy/kubernetes/daemonset.yaml

# 4. Verify
kubectl get resourceslices -o wide

# 5. Run test pod
kubectl apply -f deploy/kubernetes/test/
kubectl logs cxi-test
```

### What mock mode exercises

| Component | Behaviour |
|---|---|
| Device discovery | Skipped — synthetic devices used |
| CDI spec writing | Real — spec written to `/var/run/cdi/` inside kind node |
| ResourceSlice | Real — published to API server with NUMA/PCIe/HSN attrs |
| Scheduler allocation | Real — scheduler allocates from the slice |
| NodePrepareResources | Real — CDI device ID returned to kubelet |
| Container mount | Real — `/dev/null` bind-mounted as device |

### Environment variables for setup.sh

| Variable | Default | Description |
|---|---|---|
| `CLUSTER_NAME` | `cxi-dra-dev` | kind cluster name |
| `IMAGE` | `cxi-dra-driver:dev` | Docker image tag |
| `MOCK_DEVICES` | `2` | Number of synthetic CXI devices per node |

---

## Notes

- The kubelet plugin gRPC proto is imported from `k8s.io/kubelet/pkg/apis/dra/v1alpha4`. Verify this path for your exact 1.34 build — it may have been promoted to `v1beta1` or `v1` in that release. The import appears in [internal/driver/prepare.go](internal/driver/prepare.go) and [internal/driver/unprepare.go](internal/driver/unprepare.go).
- Run `go mod tidy` after cloning to resolve the full dependency graph.
- The module path is `github.com/slingshot/cxi-dra-driver` — update in `go.mod` and all imports if you fork under a different org.
