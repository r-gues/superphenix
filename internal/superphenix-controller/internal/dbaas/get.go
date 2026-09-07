package dbaas

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GetDatabase returns one SPX-managed database by effective ID. A NotFound is
// propagated so the handler answers 404 rather than reporting an empty database.
func GetDatabase(ctx context.Context, namespace, eid string) (view.DatabaseView, error) {
	log := logger.GetLogger(ctx)
	if namespace == "" {
		log.Error().Msg("No namespace provided")
		return view.DatabaseView{}, fmt.Errorf("no namespace provided")
	}

	resources := k8s.DynamicClientSet.Resource(clustersGVR).Namespace(namespace)
	item, err := resources.Get(ctx, eid, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return view.DatabaseView{}, err
	}
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("eid", eid).Msg("Error getting database")
		return view.DatabaseView{}, err
	}

	if err := utils.CheckProjectLabel(item, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("eid", eid).Msg("Database access denied")
		return view.DatabaseView{}, err
	}

	// A CloudNativePG cluster that is not ours is reported as missing rather than
	// exposed: the project may run its own operator-managed databases.
	if !isManagedByDbaas(item.GetLabels()) {
		log.Warn().Str("namespace", namespace).Str("eid", eid).Msg("Database is not managed by DBaaS")
		return view.DatabaseView{}, apierrors.NewNotFound(
			schema.GroupResource{Group: cnpgGroup, Resource: clustersGVR.Resource}, eid)
	}

	return unstructuredToDatabase(*item), nil
}
