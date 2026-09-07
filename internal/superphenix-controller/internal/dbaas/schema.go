package dbaas

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// cnpgGroup is the CloudNativePG API group. The CRD types are read through the
// dynamic client rather than the upstream Go module, so the controller does not
// take a dependency on the operator's release cadence.
const cnpgGroup = "postgresql.cnpg.io"

const cnpgVersion = "v1"

var (
	clustersGVR = schema.GroupVersionResource{
		Group:    cnpgGroup,
		Version:  cnpgVersion,
		Resource: "clusters",
	}
)
