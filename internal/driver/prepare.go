package driver

import (
	"context"
	"fmt"

	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"

	"github.com/slingshot/cxi-dra-driver/internal/cdi"
)

// PrepareResourceClaims is called by the kubelet plugin helper when a pod
// using CXI claims is scheduled to this node. The helper has already fetched
// the ResourceClaim objects and verified their allocation; we just need to
// return the CDI device IDs for each allocated device.
func (d *Driver) PrepareResourceClaims(
	_ context.Context,
	claims []*resourceapi.ResourceClaim,
) (map[types.UID]kubeletplugin.PrepareResult, error) {
	result := make(map[types.UID]kubeletplugin.PrepareResult, len(claims))
	for _, claim := range claims {
		result[claim.UID] = d.prepareClaim(claim)
	}
	return result, nil
}

func (d *Driver) prepareClaim(claim *resourceapi.ResourceClaim) kubeletplugin.PrepareResult {
	if claim.Status.Allocation == nil {
		return kubeletplugin.PrepareResult{
			Err: fmt.Errorf("ResourceClaim %s/%s has no allocation", claim.Namespace, claim.Name),
		}
	}

	var devices []kubeletplugin.Device
	for _, alloc := range claim.Status.Allocation.Devices.Results {
		if alloc.Driver != d.driverName {
			continue
		}
		devices = append(devices, kubeletplugin.Device{
			Requests:     []string{alloc.Request},
			PoolName:     alloc.Pool,
			DeviceName:   alloc.Device,
			CDIDeviceIDs: []string{cdi.DeviceID(d.driverName, alloc.Device)},
		})
	}
	return kubeletplugin.PrepareResult{Devices: devices}
}
