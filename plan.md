DRA Driver for Cluster-Wide Resources — Initial PoC Plan
Goal
Build a minimal DRA (Dynamic Resource Allocation) driver that exposes a finite pool of cluster-scoped resources (not tied to any specific node) via Kubernetes ResourceSlices. When a pod claims one of these resources, it's consumed from the entire cluster's pool, not just from the node the pod lands on.
Key Concepts

ResourceSlice is the DRA API object that advertises available devices to the scheduler.
ResourceSlices support three node-scoping modes: nodeName, nodeSelector, and allNodes. For cluster-wide resources, use allNodes: true, which tells the scheduler that any node can access the devices in this pool.
The scheduler tracks allocations globally — once a device is claimed, it's removed from the available pool cluster-wide, regardless of which node the pod runs on.
The driver doesn't need to be a per-node DaemonSet. A centralized controller can publish and manage the ResourceSlices.

What to Build

A centralized controller/driver (single replica Deployment, not a DaemonSet) that:

Publishes one or more ResourceSlice objects with allNodes: true, advertising a pool of named devices with whatever attributes make sense for the resource being modelled.
Updates the ResourceSlices when the pool changes (e.g. devices added/removed).
Increments pool.generation and keeps resourceSliceCount consistent across all slices in the pool when updating.


A DeviceClass that selects devices from this driver using a CEL expression matching on device.driver == "<your-driver-name>".
Test manifests:

A ResourceClaimTemplate requesting one device from the DeviceClass.
A test Pod referencing that claim.
Deploy two pods and verify that the second pod either gets a different device from the pool or goes pending if the pool is exhausted — confirming cluster-wide consumption.



What to Skip for the PoC

Kubelet plugin / device preparation: For a first pass, if the resource doesn't need actual on-node setup (e.g. it's a logical resource like a license or a remote API slot), you can skip implementing the kubelet gRPC plugin. The pod will be scheduled and the claim allocated, which is enough to prove the model works.
Network-attach logic: No need to implement dynamic fabric attach for the PoC.
DeviceTaintRules, counters, per-device node selection: These are advanced features; ignore for now.

Key API Details

API group: resource.k8s.io/v1 (stable in Kubernetes 1.34+)
The ResourceSlice spec must set exactly one of nodeName, nodeSelector, or allNodes.
Each device in a pool must have a unique name.
A pool can span multiple ResourceSlice objects (max 128 devices per slice).
The driver name should be a DNS subdomain you control (e.g. myresource.example.com).

Validation Criteria
The PoC is successful if:

kubectl get resourceslices shows the driver's slices with the correct pool and device count.
A pod with a ResourceClaim gets scheduled and the claim shows as allocated.
A second pod claiming from the same pool gets a different device.
When the pool is fully consumed, further pods go Pending with a scheduling failure related to insufficient resources.