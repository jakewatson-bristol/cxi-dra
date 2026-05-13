package driver

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"

	"github.com/slingshot/cxi-dra-driver/internal/cdi"
	"github.com/slingshot/cxi-dra-driver/internal/cxi"
)

// Driver implements kubeletplugin.DRAPlugin for CXI devices.
type Driver struct {
	nodeName     string
	podUID       string
	driverName   string
	cdiRoot      string
	pluginDir    string
	registrarDir string
	kubeClient   kubernetes.Interface
	mockDevices  []*cxi.Device
	devices      []*cxi.Device
}

func New(
	nodeName, podUID, driverName, cdiRoot, pluginDir, registrarDir string,
	kubeClient kubernetes.Interface,
	mockDevices []*cxi.Device,
) *Driver {
	return &Driver{
		nodeName:     nodeName,
		podUID:       podUID,
		driverName:   driverName,
		cdiRoot:      cdiRoot,
		pluginDir:    pluginDir,
		registrarDir: registrarDir,
		kubeClient:   kubeClient,
		mockDevices:  mockDevices,
	}
}

// Run discovers devices, writes CDI specs, starts the kubelet plugin, and
// publishes a ResourceSlice. Blocks until ctx is cancelled.
func (d *Driver) Run(ctx context.Context) error {
	if d.mockDevices != nil {
		d.devices = d.mockDevices
	} else {
		devices, err := cxi.Discover()
		if err != nil {
			return fmt.Errorf("discovering CXI devices: %w", err)
		}
		d.devices = devices
	}

	for _, dev := range d.devices {
		if err := cdi.WriteSpec(d.cdiRoot, d.driverName, dev); err != nil {
			return fmt.Errorf("writing CDI spec for %s: %w", dev.Name, err)
		}
	}

	opts := []kubeletplugin.Option{
		kubeletplugin.KubeClient(d.kubeClient),
		kubeletplugin.NodeName(d.nodeName),
		kubeletplugin.DriverName(d.driverName),
		kubeletplugin.PluginDataDirectoryPath(d.pluginDir),
		kubeletplugin.RegistrarDirectoryPath(d.registrarDir),
	}
	if d.podUID != "" {
		opts = append(opts, kubeletplugin.RollingUpdate(types.UID(d.podUID)))
	}
	helper, err := kubeletplugin.Start(ctx, d, opts...)
	if err != nil {
		return fmt.Errorf("starting kubelet plugin: %w", err)
	}
	defer helper.Stop()

	if err := helper.PublishResources(ctx, d.buildDriverResources()); err != nil {
		return fmt.Errorf("publishing resources: %w", err)
	}

	<-ctx.Done()
	return nil
}

// HandleError is called by the kubelet plugin helper for background errors
// (e.g. ResourceSlice controller failures).
func (d *Driver) HandleError(_ context.Context, err error, msg string) {
	fmt.Printf("cxi-dra-driver background error: %s: %v\n", msg, err)
}
