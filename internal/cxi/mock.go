package cxi

import "fmt"

// MockDevices returns n synthetic CXI devices for use in local testing
// environments (e.g. kind) where no real hardware is present.
//
// DevPath is set to /dev/null so CDI can bind-mount a valid file into the
// container. SysfsPath points to /tmp/mock-cxi/cxiN which the driver creates
// at startup. All other attributes are plausible but fabricated.
func MockDevices(n int) []*Device {
	devices := make([]*Device, n)
	for i := range n {
		devices[i] = &Device{
			Name:        fmt.Sprintf("cxi%d", i),
			Index:       i,
			DevPath:     fmt.Sprintf("/dev/cxi%d", i),
			HostDevPath: "/dev/null",
			SysfsPath:   fmt.Sprintf("/tmp/mock-cxi/cxi%d", i),
			NUMANode:    i % 2,
			PCIeRoot:    fmt.Sprintf("0000:%02x:00.0", 0x81+i),
			HSNIface:    fmt.Sprintf("hsn%d", i*2),
		}
	}
	return devices
}
