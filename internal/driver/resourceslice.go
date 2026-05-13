package driver

import (
	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/dynamic-resource-allocation/resourceslice"
	"k8s.io/utils/ptr"

	"github.com/slingshot/cxi-dra-driver/internal/cxi"
)

// buildDriverResources constructs the DriverResources payload for
// helper.PublishResources. The helper's ResourceSlice controller takes care of
// creating/updating/deleting ResourceSlice objects on the API server.
func (d *Driver) buildDriverResources() resourceslice.DriverResources {
	return resourceslice.DriverResources{
		Pools: map[string]resourceslice.Pool{
			d.nodeName: {
				Slices: []resourceslice.Slice{
					{Devices: buildDevices(d.devices)},
				},
			},
		},
	}
}

func buildDevices(devices []*cxi.Device) []resourceapi.Device {
	out := make([]resourceapi.Device, 0, len(devices))
	for _, dev := range devices {
		out = append(out, resourceapi.Device{
			Name: dev.Name,
			Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
				"numa_node": {IntValue: ptr.To(int64(dev.NUMANode))},
				"pcie_root": {StringValue: ptr.To(dev.PCIeRoot)},
				"hsn_iface": {StringValue: ptr.To(dev.HSNIface)},
			},
		})
	}
	return out
}
