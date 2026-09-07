package paas

import (
	"encoding/json"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/dbaas"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/utils"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	spxIdMiddleware "github.com/super-phenix/superphenix/pkg/superphenix-id/middleware"

	ch "github.com/super-phenix/superphenix/pkg/chi-helper"

	"github.com/go-chi/chi/v5"
	"k8s.io/apimachinery/pkg/api/errors"
)

const baseDbaasEndpoint = "/dbaas"

// DBaaSEndpoint mounts the read-only DBaaS routes. Databases are provisioned by
// ArgoCD from the sfs-dbaas chart, so the controller never creates or mutates
// them; it only reports what is deployed.
func DBaaSEndpoint(router chi.Router) {
	router.Route(baseDbaasEndpoint, func(r chi.Router) {
		r.Get("/", listDatabases)

		r.With(spxIdMiddleware.AddEffectiveIdToContext()).Get("/localId/{localId}", getDatabaseByLocalId)
		r.Route("/{effectiveId}", func(r chi.Router) {
			r.Get("/", getDatabaseByEffectiveId)
			r.Get("/credentials", getDatabaseCredentials)
		})
	})
}

// listDatabases
//
//	@Summary		Retrieve all databases
//	@Description	Retrieve all managed databases of a project
//	@Tags			v1, Database
//	@Produce		json
//	@Param			orgId		path	string		true	"Organization ID"
//	@Param			projectId	path	string		true	"Project ID"
//	@Success		200			{array}	view.DBaaS	"Databases"
//	@Failure		500
//	@Router			/{orgId}/{projectId}/dbaas [get]
func listDatabases(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)

	databases, err := dbaas.ListDatabases(r.Context(), namespaceParam)
	if err != nil {
		log.Err(err).Msg("failed to retrieve resource")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource")
		return
	}

	resourceList := make([]view.DBaaS, 0, len(databases))
	for _, database := range databases {
		resourceList = append(resourceList, view.DBaaSToResource(database))
	}

	b, _ := json.Marshal(resourceList)

	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// getDatabaseByLocalId
//
//	@Summary		Get database by local ID
//	@Description	Get a managed database by local ID
//	@Tags			v1, Database
//	@Produce		json
//	@Param			orgId		path		string		true	"Organization ID"
//	@Param			projectId	path		string		true	"Project ID"
//	@Param			localId		path		string		true	"Database Local ID"
//	@Success		200			{object}	view.DBaaS	"Database"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/dbaas/localId/{localId} [get]
func getDatabaseByLocalId(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	effectiveId := r.Context().Value(spxId.EffectiveIdContext())
	if effectiveId == nil {
		log.Error().Ctx(r.Context()).Msg("failed to retrieve Resource Effective Id")
		http.Error(w, "failed to retrieve Resource Effective Id", http.StatusInternalServerError)
		return
	}
	getDatabase(w, r, effectiveId.(string))
}

// getDatabaseByEffectiveId
//
//	@Summary		Get database by effective ID
//	@Description	Get a managed database by effective ID
//	@Tags			v1, Database
//	@Produce		json
//	@Param			orgId		path		string		true	"Organization ID"
//	@Param			projectId	path		string		true	"Project ID"
//	@Param			effectiveId	path		string		true	"Database Effective ID"
//	@Success		200			{object}	view.DBaaS	"Database"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/dbaas/{effectiveId} [get]
func getDatabaseByEffectiveId(w http.ResponseWriter, r *http.Request) {
	effectiveId := chi.URLParam(r, "effectiveId")
	getDatabase(w, r, effectiveId)
}

func getDatabase(w http.ResponseWriter, r *http.Request, effectiveId string) {
	log := logger.GetLogger(r.Context())

	namespaceParam := utils.GetRequestNamespace(r)
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	database, err := dbaas.GetDatabase(r.Context(), namespaceParam, effectiveId)
	if errors.IsNotFound(err) {
		log.Err(err).Msg("Resource not found")
		httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
		return
	}
	if err != nil {
		log.Err(err).Msg("failed to retrieve resource")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource")
		return
	}

	b, _ := json.Marshal(view.DBaaSToResource(database))

	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// getDatabaseCredentials
//
//	@Summary		Get database credentials
//	@Description	Get the connection credentials of a managed database
//	@Tags			v1, Database
//	@Produce		json
//	@Param			orgId		path		string							true	"Organization ID"
//	@Param			projectId	path		string							true	"Project ID"
//	@Param			effectiveId	path		string							true	"Database Effective ID"
//	@Success		200			{object}	view.DatabaseCredentialsView	"Credentials"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/dbaas/{effectiveId}/credentials [get]
func getDatabaseCredentials(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())

	namespaceParam := utils.GetRequestNamespace(r)
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	credentials, err := dbaas.GetCredentials(r.Context(), namespaceParam, effectiveId)
	if errors.IsNotFound(err) {
		log.Err(err).Msg("Resource not found")
		httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
		return
	}
	if err != nil {
		log.Err(err).Msg("failed to retrieve resource")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource")
		return
	}

	b, _ := json.Marshal(credentials)

	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}
