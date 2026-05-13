package cdi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	specs "tags.cncf.io/container-device-interface/specs-go"

	"github.com/slingshot/cxi-dra-driver/internal/cxi"
)

const cdiVersion = "0.6.0"

// DeviceID returns the CDI device identifier for a CXI device, e.g.
// "cxi.slingshot.hpe.com/cxi=cxi0".
func DeviceID(driverName, devName string) string {
	return fmt.Sprintf("%s/cxi=%s", driverName, devName)
}

// WriteSpec generates a CDI spec for dev and writes it under cdiRoot.
// The spec mounts /dev/cxiN (character device) and the device's sysfs subtree
// into the container so libfabric can discover the HSN path.
func WriteSpec(cdiRoot, driverName string, dev *cxi.Device) error {
	if err := os.MkdirAll(cdiRoot, 0o755); err != nil {
		return err
	}

	hostDevPath := dev.DevPath
	if dev.HostDevPath != "" {
		hostDevPath = dev.HostDevPath
	}
	containerSysfsPath := "/run/cxi/" + dev.Name

	spec := &specs.Spec{
		Version: cdiVersion,
		Kind:    driverName + "/cxi",
		Devices: []specs.Device{
			{
				Name: dev.Name,
				ContainerEdits: specs.ContainerEdits{
					Mounts: []*specs.Mount{
						{
							HostPath:      hostDevPath,
							ContainerPath: dev.DevPath,
							Options:       []string{"bind", "rw"},
						},
						{
							HostPath:      dev.SysfsPath,
							ContainerPath: containerSysfsPath,
							Options:       []string{"bind", "ro"},
						},
					},
				},
			},
		},
	}

	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling CDI spec for %s: %w", dev.Name, err)
	}

	path := filepath.Join(cdiRoot, driverName+"-cxi-"+dev.Name+".json")
	return os.WriteFile(path, data, 0o644)
}

// RemoveSpec deletes the CDI spec file for dev.
func RemoveSpec(cdiRoot, driverName string, dev *cxi.Device) error {
	path := filepath.Join(cdiRoot, driverName+"-cxi-"+dev.Name+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
