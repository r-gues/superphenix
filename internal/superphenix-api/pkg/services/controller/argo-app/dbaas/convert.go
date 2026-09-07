package dbaas

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo/view"

	// Use v2 because v3 indent with 4 spaces instead of 2
	"gopkg.in/yaml.v2"
)

// ConvertAppToUpdateDBaaSSpec converts the sfs-dbaas Helm values carried by a
// live ArgoCD Application back into a DBaaSSpec, so an update can be validated
// against what is actually deployed without persisting the spec ourselves.
func ConvertAppToUpdateDBaaSSpec(app view.AppView) (DBaaSSpec, error) {
	if app.Spec.Source == nil || app.Spec.Source.Plugin == nil {
		return DBaaSSpec{}, fmt.Errorf("failed to convert app to DBaaSSpec")
	}

	var helmValues Values
	for _, entry := range app.Spec.Source.Plugin.Env {
		if entry == nil || entry.Name != "HELM_VALUES" {
			continue
		}
		if err := yaml.Unmarshal([]byte(entry.Value), &helmValues); err != nil {
			return DBaaSSpec{}, err
		}
	}

	var result DBaaSSpec
	for _, db := range helmValues.Databases {
		result = DBaaSSpec{
			Engine:     db.Engine,
			Version:    db.Version,
			Instances:  db.Instances,
			Cpu:        parseInt(db.Resources.Cpu),
			Memory:     parseQuantityGi(db.Resources.Memory),
			Storage:    convertStorage(db.Storage),
			Bootstrap:  BootstrapSpec{Database: db.Bootstrap.Database, Owner: db.Bootstrap.Owner},
			Parameters: db.Parameters,
			Network:    convertNetwork(db.Network),
		}
		if db.WalStorage != nil {
			wal := convertStorage(*db.WalStorage)
			result.WalStorage = &wal
		}

		break // There is only ever one database described in the values
	}

	return result, nil
}

func convertStorage(s Storage) StorageSpec {
	return StorageSpec{Size: parseQuantityGi(s.Size), StorageClass: s.StorageClass}
}

func convertNetwork(n Network) NetworkSpec {
	spec := NetworkSpec{Vip: n.Vip, AllowedCidrs: n.AllowedCidrs}
	if n.PublicAccess != nil {
		spec.PublicAccess = &PublicAccessSpec{
			EipLocalID:   n.PublicAccess.EipLocalID,
			ExternalPort: n.PublicAccess.ExternalPort,
		}
	}
	return spec
}

// parseQuantityGi turns a rendered "<n>Gi" quantity back into its integer GiB
// value. Anything unparseable yields 0, which the validation then rejects.
func parseQuantityGi(v string) int {
	return parseInt(strings.TrimSuffix(v, "Gi"))
}

func parseInt(v string) int {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 0
	}
	return n
}
