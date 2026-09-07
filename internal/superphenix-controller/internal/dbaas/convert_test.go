package dbaas

import (
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	testEid       = "spx-11111111-1111-1111-1111-111111111111"
	testNamespace = "spx-22222222-2222-2222-2222-222222222222"
)

func setupStorageClasses(t *testing.T) {
	t.Helper()
	config.Global.ProductsConfig.BlockStorage.StorageClassMapping = map[string]string{
		"sc1": "spx-rbd-nvme-3x",
	}
}

// clusterObject builds a representative CloudNativePG Cluster, optionally
// mutated to cover a variant.
func clusterObject(mutate func(map[string]interface{})) unstructured.Unstructured {
	obj := map[string]interface{}{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata": map[string]interface{}{
			"name":      testEid,
			"namespace": testNamespace,
			"labels": map[string]interface{}{
				AppNameLabelKey:               ChartName,
				spxId.SpxLabelProjectID:       testNamespace,
				spxId.SpxLabelResourceLocalID: "prod-db",
				spxId.SpxLabelResourceName:    "Production database",
				spxId.SpxLabelGitops:          "false",
			},
		},
		"spec": map[string]interface{}{
			"imageName": "ghcr.io/cloudnative-pg/postgresql:17.2-standard-bookworm",
			"instances": int64(3),
			"resources": map[string]interface{}{
				"requests": map[string]interface{}{"cpu": "2", "memory": "8Gi"},
			},
			"storage": map[string]interface{}{
				"size":         "50Gi",
				"storageClass": "spx-rbd-nvme-3x",
			},
			"postgresql": map[string]interface{}{
				"parameters": map[string]interface{}{"max_connections": "200"},
			},
		},
		"status": map[string]interface{}{
			"phase":          "Cluster in healthy state",
			"instances":      int64(3),
			"readyInstances": int64(3),
			"currentPrimary": testEid + "-1",
			"targetPrimary":  testEid + "-1",
		},
	}
	if mutate != nil {
		mutate(obj)
	}
	return unstructured.Unstructured{Object: obj}
}

func withWalStorage(obj map[string]interface{}) {
	obj["spec"].(map[string]interface{})["walStorage"] = map[string]interface{}{
		"size":         "10Gi",
		"storageClass": "spx-rbd-nvme-3x",
	}
}

func withEmptySpec(obj map[string]interface{}) {
	obj["spec"] = map[string]interface{}{}
	obj["status"] = map[string]interface{}{}
}

func withUnmappedStorageClass(obj map[string]interface{}) {
	obj["spec"].(map[string]interface{})["storage"].(map[string]interface{})["storageClass"] = "some-infra-class"
}

func TestUnstructuredToDatabase(t *testing.T) {
	setupStorageClasses(t)

	tests := []struct {
		name   string
		mutate func(map[string]interface{})
		verify func(t *testing.T, got view.DatabaseView)
	}{
		{
			name: "identity and labels are carried over",
			verify: func(t *testing.T, got view.DatabaseView) {
				if got.Name != testEid {
					t.Errorf("name = %q, want %q", got.Name, testEid)
				}
				if got.Namespace != testNamespace {
					t.Errorf("namespace = %q, want %q", got.Namespace, testNamespace)
				}
				if got.Labels[spxId.SpxLabelResourceLocalID] != "prod-db" {
					t.Errorf("labels = %v", got.Labels)
				}
			},
		},
		{
			name: "the spec is projected",
			verify: func(t *testing.T, got view.DatabaseView) {
				if got.Spec.Engine != enginePostgreSQL {
					t.Errorf("engine = %q, want %q", got.Spec.Engine, enginePostgreSQL)
				}
				if got.Spec.Version != "17" {
					t.Errorf("version = %q, want 17", got.Spec.Version)
				}
				if got.Spec.Instances != 3 {
					t.Errorf("instances = %d, want 3", got.Spec.Instances)
				}
				if got.Spec.Resources.Cpu != "2" || got.Spec.Resources.Memory != "8Gi" {
					t.Errorf("resources = %+v", got.Spec.Resources)
				}
				if got.Spec.Parameters["max_connections"] != "200" {
					t.Errorf("parameters = %+v", got.Spec.Parameters)
				}
			},
		},
		{
			name: "the infrastructure storage class is reported as its short name",
			verify: func(t *testing.T, got view.DatabaseView) {
				if got.Spec.Storage.Size != "50Gi" {
					t.Errorf("storage size = %q, want 50Gi", got.Spec.Storage.Size)
				}
				if got.Spec.Storage.StorageClass != "sc1" {
					t.Errorf("storage class = %q, want sc1", got.Spec.Storage.StorageClass)
				}
			},
		},
		{
			name:   "an unmapped storage class does not leak the infrastructure name",
			mutate: withUnmappedStorageClass,
			verify: func(t *testing.T, got view.DatabaseView) {
				if got.Spec.Storage.StorageClass == "some-infra-class" {
					t.Error("storage class leaked the infrastructure name")
				}
			},
		},
		{
			name: "the status is projected, service names included",
			verify: func(t *testing.T, got view.DatabaseView) {
				if got.Status.Phase == "" || got.Status.Instances != 3 || got.Status.ReadyInstances != 3 {
					t.Errorf("status = %+v", got.Status)
				}
				if got.Status.CurrentPrimary != testEid+"-1" {
					t.Errorf("currentPrimary = %q", got.Status.CurrentPrimary)
				}
				if got.Status.WriteService != testEid+readWriteServiceSuffix {
					t.Errorf("writeService = %q", got.Status.WriteService)
				}
				if got.Status.ReadService != testEid+readOnlyServiceSuffix {
					t.Errorf("readService = %q", got.Status.ReadService)
				}
			},
		},
		{
			name: "walStorage stays absent when it was not requested",
			verify: func(t *testing.T, got view.DatabaseView) {
				if got.Spec.WalStorage != nil {
					t.Errorf("walStorage = %+v, want nil", got.Spec.WalStorage)
				}
			},
		},
		{
			name:   "walStorage is surfaced when declared",
			mutate: withWalStorage,
			verify: func(t *testing.T, got view.DatabaseView) {
				if got.Spec.WalStorage == nil {
					t.Fatal("walStorage = nil, want a value")
				}
				if got.Spec.WalStorage.Size != "10Gi" || got.Spec.WalStorage.StorageClass != "sc1" {
					t.Errorf("walStorage = %+v", got.Spec.WalStorage)
				}
			},
		},
		{
			name:   "an empty spec and status yield a zero projection rather than a panic",
			mutate: withEmptySpec,
			verify: func(t *testing.T, got view.DatabaseView) {
				if got.Spec.Version != "" || got.Spec.Instances != 0 || got.Spec.WalStorage != nil {
					t.Errorf("spec = %+v, want a zero spec", got.Spec)
				}
				if got.Spec.Parameters != nil {
					t.Errorf("parameters = %+v, want nil", got.Spec.Parameters)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.verify(t, unstructuredToDatabase(clusterObject(tt.mutate)))
		})
	}
}

func TestMajorFromImage(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  string
	}{
		{name: "standard tag", image: "ghcr.io/cloudnative-pg/postgresql:17.2-standard-bookworm", want: "17"},
		{name: "major only tag", image: "ghcr.io/cloudnative-pg/postgresql:16", want: "16"},
		{name: "no tag", image: "ghcr.io/cloudnative-pg/postgresql", want: ""},
		{name: "empty", image: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := majorFromImage(tt.image); got != tt.want {
				t.Errorf("majorFromImage(%q) = %q, want %q", tt.image, got, tt.want)
			}
		})
	}
}

func TestIsManagedByDbaas(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   bool
	}{
		{name: "rendered by sfs-dbaas", labels: map[string]string{AppNameLabelKey: ChartName}, want: true},
		{name: "rendered by another chart", labels: map[string]string{AppNameLabelKey: "sfs-kaas"}, want: false},
		{name: "no chart label at all", labels: map[string]string{}, want: false},
		{name: "nil labels", labels: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isManagedByDbaas(tt.labels); got != tt.want {
				t.Errorf("isManagedByDbaas(%v) = %v, want %v", tt.labels, got, tt.want)
			}
		})
	}
}
