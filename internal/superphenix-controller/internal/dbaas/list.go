package dbaas

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ListDatabases returns every SPX-managed database of a project.
func ListDatabases(ctx context.Context, namespace string) ([]view.DatabaseView, error) {
	log := logger.GetLogger(ctx)

	resources := k8s.DynamicClientSet.Resource(clustersGVR).Namespace(namespace)
	unstructuredList, err := resources.List(ctx, metav1.ListOptions{})
	if apierrors.IsNotFound(err) {
		// The CRD is not installed on this AZ: no databases, not an error.
		return nil, nil
	}
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Error getting database list")
		return []view.DatabaseView{}, err
	}

	databases := make([]view.DatabaseView, 0)
	for _, item := range unstructuredList.Items {
		if err := utils.CheckProjectLabel(&item, namespace); err != nil {
			continue
		}
		if !isManagedByDbaas(item.GetLabels()) {
			continue
		}
		databases = append(databases, unstructuredToDatabase(item))
	}

	return databases, nil
}
