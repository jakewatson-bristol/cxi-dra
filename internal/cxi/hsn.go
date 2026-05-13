package cxi

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const sysfsNetPath = "/sys/class/net"

// PairDevice walks sysfs PCIe topology to find the HSN network interface that
// shares the same PCIe root complex as dev, and sets dev.HSNIface.
//
// The pairing relies on the fact that a CXI NIC and its companion HSN ethernet
// function are siblings under the same PCIe root port. We climb the CXI
// device's sysfs path to the root-complex level, then scan net interfaces
// whose sysfs path is rooted under the same complex.
func PairDevice(dev *Device) error {
	cxiPCIPath, err := filepath.EvalSymlinks(filepath.Join(dev.SysfsPath, "device"))
	if err != nil {
		return fmt.Errorf("resolving CXI sysfs link for %s: %w", dev.Name, err)
	}

	// Walk up to the PCIe root complex (three levels: function → slot → root-port → bus).
	rootComplex := pciRootComplex(cxiPCIPath)

	entries, err := os.ReadDir(sysfsNetPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", sysfsNetPath, err)
	}

	for _, e := range entries {
		iface := e.Name()
		if !strings.HasPrefix(iface, "hsn") {
			continue
		}

		ifaceSysfs, err := filepath.EvalSymlinks(filepath.Join(sysfsNetPath, iface, "device"))
		if err != nil {
			continue
		}

		if strings.HasPrefix(ifaceSysfs, rootComplex) {
			dev.HSNIface = iface
			return nil
		}
	}

	return fmt.Errorf("no HSN interface found for %s under root complex %s", dev.Name, rootComplex)
}

// pciRootComplex returns the sysfs path of the PCIe root complex for a given
// PCI function path by walking up until the path segment looks like a PCI bus
// (e.g. "0000:80:" — two hex octets separated by a colon).
func pciRootComplex(pciPath string) string {
	dir := pciPath
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		base := filepath.Base(parent)
		// PCIe bus directories look like "0000:80" (domain:bus).
		if isPCIBus(base) {
			return parent
		}
		dir = parent
	}
	return pciPath
}

// isPCIBus returns true for sysfs directory names matching a PCIe bus segment
// like "0000:80" (domain:bus in hex).
func isPCIBus(name string) bool {
	parts := strings.Split(name, ":")
	if len(parts) != 2 {
		return false
	}
	return len(parts[0]) == 4 && len(parts[1]) == 2
}
