package driver

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"
)

// UnprepareResourceClaims is called when pods using CXI claims finish or are
// removed. CDI specs are node-global so there is nothing to tear down for the
// initial implementation. VNI service teardown will go here once implemented.
func (d *Driver) UnprepareResourceClaims(
	_ context.Context,
	claims []kubeletplugin.NamespacedObject,
) (map[types.UID]error, error) {
	result := make(map[types.UID]error, len(claims))
	for _, claim := range claims {
		result[claim.UID] = nil
	}
	return result, nil
}
