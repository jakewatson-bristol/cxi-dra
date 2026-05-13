package cxi

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const sysfsClassPath = "/sys/class/cxi_user"

// Device represents a discovered CXI PCIe device on this node.
type Device struct {
	Name        string // "cxi0"
	Index       int    // 0
	DevPath     string // "/dev/cxi0" — container-visible device path
	HostDevPath string // host-side device path when it differs from DevPath (e.g. mock uses /dev/null)
	SysfsPath   string // host-side sysfs path; container sees it at /sys/class/cxi_user/<name>
	NUMANode    int    // NUMA node affinity (-1 if unknown)
	PCIeRoot    string // PCI slot name, e.g. "0000:81:00.0"
	HSNIface    string // paired HSN interface, e.g. "hsn2" (set by PairDevice)
}

// Discover walks /sys/class/cxi_user and returns one Device per cxi* entry.
func Discover() ([]*Device, error) {
	entries, err := os.ReadDir(sysfsClassPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", sysfsClassPath, err)
	}

	var devices []*Device
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "cxi") {
			continue
		}

		idx, err := strconv.Atoi(strings.TrimPrefix(name, "cxi"))
		if err != nil {
			continue
		}

		sysfsPath := filepath.Join(sysfsClassPath, name)
		dev := &Device{
			Name:      name,
			Index:     idx,
			DevPath:   filepath.Join("/dev", name),
			SysfsPath: sysfsPath,
			NUMANode:  readIntFile(filepath.Join(sysfsPath, "device", "numa_node"), -1),
			PCIeRoot:  readPCISlot(filepath.Join(sysfsPath, "device", "uevent")),
		}

		if err := PairDevice(dev); err != nil {
			// HSN pairing is best-effort; log but don't fail discovery.
			_ = err
		}

		devices = append(devices, dev)
	}
	return devices, nil
}

func readIntFile(path string, fallback int) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return fallback
	}
	return v
}

// readPCISlot extracts PCI_SLOT_NAME from a uevent file.
func readPCISlot(ueventPath string) string {
	data, err := os.ReadFile(ueventPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PCI_SLOT_NAME=") {
			return strings.TrimPrefix(line, "PCI_SLOT_NAME=")
		}
	}
	return ""
}
