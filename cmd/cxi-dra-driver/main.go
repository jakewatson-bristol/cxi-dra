package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"

	"github.com/slingshot/cxi-dra-driver/internal/cxi"
	"github.com/slingshot/cxi-dra-driver/internal/driver"
)

const defaultDriverName = "cxi.slingshot.hpe.com"

func main() {
	var (
		nodeName     = flag.String("node-name", os.Getenv("NODE_NAME"), "Kubernetes node name (defaults to $NODE_NAME)")
		podUID       = flag.String("pod-uid", os.Getenv("POD_UID"), "Pod UID for rolling-update socket uniqueness (defaults to $POD_UID)")
		driverName   = flag.String("driver-name", defaultDriverName, "DRA driver name (DNS subdomain)")
		cdiRoot      = flag.String("cdi-root", "/var/run/cdi", "Directory for CDI spec files")
		pluginDir    = flag.String("plugin-dir", kubeletplugin.KubeletPluginsDir+"/"+defaultDriverName, "Kubelet plugin data directory")
		registrarDir = flag.String("registrar-dir", kubeletplugin.KubeletRegistryDir, "Kubelet plugin registrar directory")
		kubeconfig   = flag.String("kubeconfig", "", "Path to kubeconfig (omit for in-cluster config)")
		mockDevices  = flag.Int("mock-devices", 0, "Generate N synthetic CXI devices instead of real discovery (for kind/local testing)")
	)
	flag.Parse()

	if *nodeName == "" {
		fmt.Fprintln(os.Stderr, "error: --node-name or $NODE_NAME must be set")
		os.Exit(1)
	}

	kubeClient, err := buildKubeClient(*kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to build kube client: %v\n", err)
		os.Exit(1)
	}

	var mock []*cxi.Device
	if *mockDevices > 0 {
		mock = cxi.MockDevices(*mockDevices)
		for _, dev := range mock {
			if err := os.MkdirAll(dev.SysfsPath, 0o755); err != nil {
				fmt.Fprintf(os.Stderr, "error: creating mock sysfs dir %s: %v\n", dev.SysfsPath, err)
				os.Exit(1)
			}
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	d := driver.New(*nodeName, *podUID, *driverName, *cdiRoot, *pluginDir, *registrarDir, kubeClient, mock)
	if err := d.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: driver exited: %v\n", err)
		os.Exit(1)
	}
}

func buildKubeClient(kubeconfig string) (kubernetes.Interface, error) {
	var cfg *rest.Config
	var err error
	if kubeconfig != "" {
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		cfg, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}
