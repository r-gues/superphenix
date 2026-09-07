package dbaas

import (
	"regexp"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// pgMajorFromImage pulls the PostgreSQL major out of an image reference such as
// "ghcr.io/cloudnative-pg/postgresql:17.2-standard-bookworm".
var pgMajorFromImage = regexp.MustCompile(`:(\d+)`)

func unstructuredToDatabase(item unstructured.Unstructured) view.DatabaseView {
	imageName, _, _ := unstructured.NestedString(item.Object, "spec", "imageName")
	instances, _, _ := unstructured.NestedInt64(item.Object, "spec", "instances")

	database := view.DatabaseView{
		ObjectMeta: view.ObjectMeta{
			Name:              item.GetName(),
			Namespace:         item.GetNamespace(),
			CreationTimestamp: item.GetCreationTimestamp(),
			Labels:            utils.FilterLabels(item.GetLabels()),
			Annotations:       utils.FilterAnnotations(item.GetAnnotations()),
		},
		Spec: view.DatabaseSpecView{
			Engine:     enginePostgreSQL,
			Version:    majorFromImage(imageName),
			ImageName:  imageName,
			Instances:  int(instances),
			Resources:  resourcesFromUnstructured(item),
			Storage:    storageFromUnstructured(item, "storage"),
			Parameters: parametersFromUnstructured(item),
		},
		Status: statusFromUnstructured(item),
	}

	if wal, ok := optionalStorageFromUnstructured(item, "walStorage"); ok {
		database.Spec.WalStorage = &wal
	}

	return database
}

const enginePostgreSQL = "postgresql"

func majorFromImage(image string) string {
	match := pgMajorFromImage.FindStringSubmatch(image)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func resourcesFromUnstructured(item unstructured.Unstructured) view.DatabaseResourcesView {
	cpu, _, _ := unstructured.NestedString(item.Object, "spec", "resources", "requests", "cpu")
	memory, _, _ := unstructured.NestedString(item.Object, "spec", "resources", "requests", "memory")
	return view.DatabaseResourcesView{Cpu: cpu, Memory: memory}
}

func storageFromUnstructured(item unstructured.Unstructured, field string) view.DatabaseStorageView {
	storage, _ := optionalStorageFromUnstructured(item, field)
	return storage
}

// optionalStorageFromUnstructured reports whether the volume is declared at all,
// so a missing walStorage stays absent instead of surfacing as an empty object.
func optionalStorageFromUnstructured(item unstructured.Unstructured, field string) (view.DatabaseStorageView, bool) {
	raw, found, err := unstructured.NestedMap(item.Object, "spec", field)
	if err != nil || !found || len(raw) == 0 {
		return view.DatabaseStorageView{}, false
	}

	size, _, _ := unstructured.NestedString(item.Object, "spec", field, "size")
	storageClass, _, _ := unstructured.NestedString(item.Object, "spec", field, "storageClass")

	return view.DatabaseStorageView{
		Size:         size,
		StorageClass: view.ConvertStorageClassName(storageClass),
	}, true
}

func parametersFromUnstructured(item unstructured.Unstructured) map[string]string {
	params, found, err := unstructured.NestedStringMap(item.Object, "spec", "postgresql", "parameters")
	if err != nil || !found || len(params) == 0 {
		return nil
	}
	return params
}

func statusFromUnstructured(item unstructured.Unstructured) view.DatabaseStatusView {
	phase, _, _ := unstructured.NestedString(item.Object, "status", "phase")
	phaseReason, _, _ := unstructured.NestedString(item.Object, "status", "phaseReason")
	instances, _, _ := unstructured.NestedInt64(item.Object, "status", "instances")
	readyInstances, _, _ := unstructured.NestedInt64(item.Object, "status", "readyInstances")
	currentPrimary, _, _ := unstructured.NestedString(item.Object, "status", "currentPrimary")
	targetPrimary, _, _ := unstructured.NestedString(item.Object, "status", "targetPrimary")

	return view.DatabaseStatusView{
		Phase:          phase,
		PhaseReason:    phaseReason,
		Instances:      int(instances),
		ReadyInstances: int(readyInstances),
		CurrentPrimary: currentPrimary,
		TargetPrimary:  targetPrimary,
		WriteService:   item.GetName() + readWriteServiceSuffix,
		ReadService:    item.GetName() + readOnlyServiceSuffix,
	}
}

// isManagedByDbaas keeps out CloudNativePG clusters that live in the project
// namespace but were not rendered by the sfs-dbaas chart.
func isManagedByDbaas(labels map[string]string) bool {
	return strings.EqualFold(labels[AppNameLabelKey], ChartName)
}
