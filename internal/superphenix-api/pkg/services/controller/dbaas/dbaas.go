package dbaas

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo/view"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/az"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/product"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/proxy"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"
	argodbaas "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/argo-app/dbaas"
	ctrlutils "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/utils"
	"github.com/super-phenix/superphenix/pkg/utils/decoder"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

// ListDBaaS
//
//	@Summary		Retrieve all DBaaS
//	@Description	Retrieve all managed databases of a project
//	@Tags			v1, SPX Argo Ctrl
//	@Produce		json
//	@Param			orgaId		path	string				true	"Organization ID"
//	@Param			projectId	path	string				true	"Project ID"
//	@Success		200			{array}	DBaaSFullResponse	"DBaaS"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{projectId}/dbaas [get]
//	@Security		Bearer[OrganizationRead, ProjectDBaaSRead]
func (h *Service) ListDBaaS(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgDb, projectDb, code, errMsg := ctrlutils.CheckListPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	urls := az.FindAll(orgDb.ID.String())

	responses, err := proxy.SendBatchProxy(r, urls, config.ApiPrefix)
	if err != nil {
		log.Err(err).Msg(consts.SpxProxyToAZFailure)
		httpError.Http(w, r, consts.SpxProxyToAZFailureCode).Msg(consts.SpxProxyToAZFailure)
		return
	}
	concatResults := ctrlutils.ConcatResponses(r.Context(), responses)

	resourcesDb, err := product.FindAllByProjectIdAndResourceType(projectDb.ID.String(), model.ProductTypeDBaaS)
	if err != nil {
		log.Err(err).
			Str("projectId", projectDb.ID.String()).
			Str("resourceType", model.ProductTypeDBaaS).
			Msg(consts.SpxFindAllResourcesError)
		httpError.Http(w, r, consts.SpxFindAllResourcesErrorCode).Msg(consts.SpxFindAllResourcesError)
		return
	}

	mapResourceCheck := make(map[uuid.UUID]bool)
	for _, p := range resourcesDb {
		mapResourceCheck[p.ID] = false
	}

	combineResults := combineListResult(concatResults, resourcesDb, mapResourceCheck)

	writeJSON(w, r, combineResults)
}

// GetPostgresVersions
//
//	@Summary		Retrieve supported PostgreSQL versions
//	@Description	Retrieve the PostgreSQL major versions this deployment supports
//	@Tags			v1, SPX Argo Ctrl
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Success		200			{array}	string	"PostgreSQL versions"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{projectId}/dbaas/postgres-versions [get]
//	@Security		Bearer[OrganizationRead, ProjectDBaaSRead]
func (h *Service) GetPostgresVersions(w http.ResponseWriter, r *http.Request) {
	versions := h.cfg.ProductsConfig.ArgoApp.Database.PostgresVersions

	result := make([]string, 0, len(versions))
	for _, v := range versions {
		result = append(result, v.Version)
	}

	writeJSON(w, r, result)
}

// GetDBaaS
//
//	@Summary		Get DBaaS
//	@Description	Get a managed database by effective ID
//	@Tags			v1, SPX Argo Ctrl
//	@Produce		json
//	@Param			orgaId		path		string				true	"Organization ID"
//	@Param			az			path		string				true	"AZ Code"
//	@Param			projectId	path		string				true	"Project ID"
//	@Param			effectiveId	path		string				true	"DBaaS EID"
//	@Success		200			{object}	DBaaSFullResponse	"DBaaS"
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/dbaas/{effectiveId} [get]
//	@Security		Bearer[OrganizationRead, ProjectDBaaSRead]
func (h *Service) GetDBaaS(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	azDb, _, projectEntity, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	if !ctrlutils.CheckProductBelongsToProject(w, r, projectEntity.ID, azDb.Code) {
		return
	}

	resp, err := proxy.SendProxy(r, azDb, config.ApiPrefix, http.NoBody)
	if err != nil {
		log.Error().Err(err).Str("az", azDb.Code).Msg(consts.SpxProxyToAZFailure)
	} else {
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			log.Error().
				Str("path", r.URL.Path).
				Str("status", resp.Status).
				Int("statusCode", resp.StatusCode).
				Str("az", azDb.Code).
				Msg("Request on superphenix-controller failed")
		}
	}

	resourceEId := chi.URLParam(r, "effectiveId")
	dbProduct, dbErr := product.FindByEId(resourceEId)

	productResponse, azResult, outcome := controller.ResolveProductResponse(r.Context(), azDb.Code, resp, dbProduct, dbErr)

	// If we can't find either spx-ctrl or db info
	if outcome == controller.MergeNotFound {
		log.Error().Str("effectiveId", resourceEId).Msg(consts.SpxResourceNotFound)
		httpError.Http(w, r, http.StatusNotFound).Str("eid", resourceEId).Msg(consts.SpxResourceNotFound)
		return
	}

	result := DBaaSFullResponse{ProductResponse: productResponse}
	if azResult != nil {
		result.Database = azResult["database"]
	}

	writeJSON(w, r, result)
}

// GetDBaaSCredentials
//
//	@Summary		Get DBaaS credentials
//	@Description	Get the connection credentials of a managed database
//	@Tags			v1, SPX Argo Ctrl
//	@Produce		json
//	@Param			orgaId		path		string	true	"Organization ID"
//	@Param			az			path		string	true	"AZ Code"
//	@Param			projectId	path		string	true	"Project ID"
//	@Param			effectiveId	path		string	true	"DBaaS EID"
//	@Success		200			{object}	object	"Credentials"
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/dbaas/{effectiveId}/credentials [get]
//	@Security		Bearer[OrganizationRead, ProjectDBaaSCredentials]
func (h *Service) GetDBaaSCredentials(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	azDb, orgDb, projectDb, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	if !ctrlutils.CheckProductBelongsToProject(w, r, projectDb.ID, azDb.Code) {
		return
	}

	effectiveId := chi.URLParam(r, "effectiveId")

	url := fmt.Sprintf("%s/%s/%s/dbaas/%s/credentials",
		azDb.ControllerUrl, orgDb.ID.String(), projectDb.ID.String(), effectiveId)
	resp, err := proxy.SendRequest(r.Context(), url, "GET", http.NoBody, azDb.AuthSecret)
	if err != nil {
		log.Err(err).Str("az", azDb.Code).Msg(consts.SpxProxyToAZFailure)
		httpError.Http(w, r, consts.SpxProxyToAZFailureCode).Str("eid", effectiveId).Msg(consts.SpxProxyToAZFailure)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		httpError.Http(w, r, http.StatusNotFound).Str("eid", effectiveId).Msg(consts.SpxResourceNotFound)
		return
	}
	if resp.StatusCode != http.StatusOK {
		log.Error().Int("status", resp.StatusCode).Str("eid", effectiveId).Msg("Failed to fetch database credentials from AZ")
		httpError.Http(w, r, http.StatusInternalServerError).Str("eid", effectiveId).Msg(consts.SpxProxyToAZFailure)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Err(err).Str("eid", effectiveId).Msg("Failed to read credentials response")
		httpError.Http(w, r, http.StatusInternalServerError).Str("eid", effectiveId).Msg(consts.SpxProxyToAZFailure)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// CreateDBaaS
//
//	@Summary		Create DBaaS
//	@Description	Create a new managed database
//	@Tags			v1, SPX Argo Ctrl
//	@Accept			json
//	@Produce		json
//	@Param			orgaId		path		string			true	"Organization ID"
//	@Param			az			path		string			true	"AZ Code"
//	@Param			projectId	path		string			true	"Project ID"
//	@Param			Body		body		CreateDBaaSBody	true	"DBaaS info"
//	@Success		200			{object}	controller.CreateResponse
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/dbaas [post]
//	@Security		Bearer[OrganizationRead, ProjectDBaaSWrite]
func (h *Service) CreateDBaaS(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	azDb, orgDb, projectDb, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	dbConfig, err := getDBaaSConfig(r.Context(), azDb, orgDb.ID.String(), projectDb.ID.String())
	if err != nil {
		log.Err(err).Msg("Failed to get dbaas config")
		httpError.Http(w, r, consts.SpxProxyToAZFailureCode).Msg(consts.SpxProxyToAZFailure)
		return
	}

	var body CreateDBaaSBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	dbaasDb, m, err := controller.CreateIntoDb(
		r.Context(), body.General.ProductName, model.ProductTypeDBaaS, azDb.Code, orgDb.ID, projectDb.ID)
	if err != nil {
		log.Err(err).Msg("Failed to save product into database")
		httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Msg(consts.SpxResourceCreationFailure)
		return
	}

	newBody, err := argodbaas.CreateArgoApp(r.Context(), dbaasDb.ID.String(), azDb, body.Spec, m, dbConfig, nil)
	if err != nil {
		ctrlutils.CleanDb(r.Context(), dbaasDb.ID)
		log.Err(err).Msg("Failed to create argo app")
		httpError.Http(w, r, http.StatusBadRequest).Msg(err.Error())
		return
	}

	if err := h.argo.CreateApp(r.Context(), newBody); err != nil {
		ctrlutils.CleanDb(r.Context(), dbaasDb.ID)
		ctrlutils.HandleArgoError(w, r, err, consts.SpxResourceCreationFailureCode, consts.SpxResourceCreationFailure)
		return
	}

	controller.WriteCreateResponse(w, dbaasDb.EffectiveID)
}

// GetForUpdateDBaaS
//
//	@Summary		Get DBaaS App
//	@Description	Get the DBaaS application spec, for the update form
//	@Tags			v1, SPX Argo Ctrl
//	@Produce		json
//	@Param			orgaId		path		string				true	"Organization ID"
//	@Param			az			path		string				true	"AZ Code"
//	@Param			projectId	path		string				true	"Project ID"
//	@Param			effectiveId	path		string				true	"DBaaS EID"
//	@Success		200			{object}	AppSpecFullResponse	"DBaaS"
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/dbaas/{effectiveId}/app [get]
//	@Security		Bearer[OrganizationRead, ProjectDBaaSWrite]
func (h *Service) GetForUpdateDBaaS(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	azDb, _, projectDb, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	if !ctrlutils.CheckProductBelongsToProject(w, r, projectDb.ID, azDb.Code) {
		return
	}

	resourceEId := chi.URLParam(r, "effectiveId")
	dbProduct, dbErr := product.FindByEId(resourceEId)

	spec, isGitops, appFound, err := h.fetchDBaaSApp(r.Context(), projectDb.ID.String(), resourceEId)
	if err != nil {
		log.Err(err).Msg("Failed to fetch app")
		httpError.Http(w, r, consts.SpxProxyToAZFailureCode).Str("eid", resourceEId).Msg(consts.SpxProxyToAZFailure)
		return
	}

	// If we can't find either spx-ctrl or db info
	if !appFound || dbErr != nil {
		log.Error().Str("effectiveId", resourceEId).Msg(consts.SpxResourceNotFound)
		httpError.Http(w, r, http.StatusNotFound).Str("eid", resourceEId).Msg(consts.SpxResourceNotFound)
		return
	}

	result := AppSpecFullResponse{
		ProductResponse: ProductResponse{
			ID:            dbProduct.ID.String(),
			EId:           dbProduct.EffectiveID,
			ProductName:   dbProduct.ProductName,
			CodeAZ:        azDb.Code,
			ProductTypeId: dbProduct.ProductTypeId,
			Gitops:        isGitops,
		},
		Spec: spec,
	}

	writeJSON(w, r, result)
}

// UpdateDBaaS
//
//	@Summary		Update DBaaS
//	@Description	Update a managed database
//	@Tags			v1, SPX Argo Ctrl
//	@Accept			json
//	@Produce		json
//	@Param			orgaId		path	string			true	"Organization ID"
//	@Param			az			path	string			true	"AZ Code"
//	@Param			projectId	path	string			true	"Project ID"
//	@Param			effectiveId	path	string			true	"DBaaS EID"
//	@Param			Body		body	UpdateDBaaSBody	true	"DBaaS info"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/dbaas/{effectiveId} [post]
//	@Security		Bearer[OrganizationRead, ProjectDBaaSWrite]
func (h *Service) UpdateDBaaS(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	azDb, orgDb, projectDb, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	dbConfig, err := getDBaaSConfig(r.Context(), azDb, orgDb.ID.String(), projectDb.ID.String())
	if err != nil {
		log.Err(err).Msg("Failed to get dbaas config")
		httpError.Http(w, r, consts.SpxProxyToAZFailureCode).Msg(consts.SpxProxyToAZFailure)
		return
	}

	var body UpdateDBaaSBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	productEid := chi.URLParam(r, "effectiveId")
	dbaasDb, err := controller.UpdateIntoDb(
		r.Context(), productEid, body.General.ProductName, model.ProductTypeDBaaS, azDb.Code, projectDb.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to save product in database")
		httpError.Http(w, r, consts.SpxResourceUpdateFailureCode).Msg(consts.SpxResourceUpdateFailure)
		return
	}

	m := spxId.Metadata{}
	if err = m.GenerateMetadata(projectDb.ID.String(), orgDb.ID.String(), dbaasDb.ID.String()); err != nil {
		log.Err(err).Msg("Failed to generate metadata")
		httpError.Http(w, r, consts.SpxResourceUpdateFailureCode).Msg(consts.SpxResourceUpdateFailure)
		return
	}

	spec, _, appFound, err := h.fetchDBaaSApp(r.Context(), projectDb.ID.String(), productEid)
	if err != nil {
		log.Err(err).Msg("Failed to fetch app")
		httpError.Http(w, r, consts.SpxProxyToAZFailureCode).Str("eid", productEid).Msg(consts.SpxProxyToAZFailure)
		return
	}

	if !appFound {
		log.Error().Str("eid", productEid).Msg("App not found")
		httpError.Http(w, r, http.StatusNotFound).Str("eid", productEid).Msg(consts.SpxResourceNotFound)
		return
	}

	newBody, err := argodbaas.CreateArgoApp(r.Context(), dbaasDb.ID.String(), azDb, body.Spec, m, dbConfig, &spec)
	if err != nil {
		log.Err(err).Msg("Failed to create argo app")
		httpError.Http(w, r, http.StatusBadRequest).Msg(err.Error())
		return
	}

	// We only apply the spec part
	appName := fmt.Sprintf("%s-%s", argodbaas.AppPrefix, dbaasDb.EffectiveID)
	if err := h.argo.UpdateApp(r.Context(), appName, h.argo.Namespace(projectDb.ID.String()), newBody.Spec); err != nil {
		ctrlutils.HandleArgoError(w, r, err, consts.SpxResourceUpdateFailureCode, consts.SpxResourceUpdateFailure)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteDBaaS
//
//	@Summary		Delete DBaaS
//	@Description	Delete a managed database by effective ID
//	@Tags			v1, SPX Argo Ctrl
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"DBaaS EID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/dbaas/{effectiveId} [delete]
//	@Security		Bearer[OrganizationRead, ProjectDBaaSWrite]
func (h *Service) DeleteDBaaS(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	azDb, _, projectDb, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	if !ctrlutils.CheckProductBelongsToProject(w, r, projectDb.ID, azDb.Code) {
		return
	}

	productEId := chi.URLParam(r, "effectiveId")

	// Deleting the Application prunes the CloudNativePG Cluster with it, so there
	// is nothing left for the controller to clean up afterwards. A missing app is
	// not an error: the DB row still has to go.
	appName := fmt.Sprintf("%s-%s", argodbaas.AppPrefix, productEId)
	deleteErr := h.argo.DeleteApp(r.Context(), appName, h.argo.Namespace(projectDb.ID.String()))
	if deleteErr != nil && !k8serrors.IsNotFound(deleteErr) {
		ctrlutils.HandleArgoError(w, r, deleteErr, consts.SpxResourceDeletionFailureCode, consts.SpxResourceDeletionFailure)
		return
	}

	rowsAffected, err := product.DeleteByEIdAndAZCodeAndProject(productEId, azDb.Code, projectDb.ID)
	if err != nil {
		log.Err(err).Msg(consts.SpxResourceDeletionFailure)
		httpError.Http(w, r, consts.SpxResourceDeletionFailureCode).Msg(consts.SpxResourceDeletionFailure)
		return
	}
	if k8serrors.IsNotFound(deleteErr) && rowsAffected == 0 {
		httpError.Http(w, r, http.StatusNotFound).Str("eid", productEId).Msg(consts.SpxResourceNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// fetchDBaaSApp fetches the application spec from the self-service ArgoCD.
//
// Returns:
//   - spec: the specification of the DBaaS application.
//   - isGitops: "true" or "false", whether the application is managed via GitOps.
//   - found: whether the application exists.
//   - err: an error if the lookup or the conversion failed.
func (h *Service) fetchDBaaSApp(
	ctx context.Context,
	projectId, resourceEId string,
) (spec argodbaas.DBaaSSpec, isGitops string, found bool, err error) {
	log := logger.GetLogger(ctx)

	appName := fmt.Sprintf("%s-%s", argodbaas.AppPrefix, resourceEId)
	appView, err := h.argo.GetApp(ctx, appName, h.argo.Namespace(projectId))
	if k8serrors.IsNotFound(err) {
		return argodbaas.DBaaSSpec{}, "", false, nil
	}
	if err != nil {
		log.Error().Err(err).Str("effectiveId", resourceEId).Msg("Failed to get argo app")
		return argodbaas.DBaaSSpec{}, "false", false, fmt.Errorf("failed to get argo app")
	}

	spec, err = argodbaas.ConvertAppToUpdateDBaaSSpec(appView)
	if err != nil {
		log.Error().Str("effectiveId", resourceEId).Msg("Failed to read app spec")
		return argodbaas.DBaaSSpec{}, "", true, err
	}

	isGitops = view.AppToResource(appView).Gitops
	if isGitops == "" {
		isGitops = "false"
	}

	return spec, isGitops, true, nil
}

// getDBaaSConfig reads the AZ's DBaaS configuration (its storage classes) from
// the controller, so the spec can be validated against what the AZ can offer.
func getDBaaSConfig(ctx context.Context, azCfg config.AZConfig, orgId, projectId string) (argodbaas.DBaaSConfig, error) {
	log := logger.GetLogger(ctx)

	url := fmt.Sprintf("%s/%s/%s/dbaas-config", azCfg.ControllerUrl, orgId, projectId)
	resp, err := proxy.SendRequest(ctx, url, "GET", http.NoBody, azCfg.AuthSecret)
	if err != nil {
		log.Err(err).Str("az", azCfg.Code).Msg(consts.SpxProxyToAZFailure)
		return argodbaas.DBaaSConfig{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorBody := httpError.RetrieveHttpError(resp)
		log.Error().Any("responseBody", errorBody).
			Str("status", resp.Status).Int("statusCode", resp.StatusCode).
			Str("azCode", azCfg.Code).
			Msg("Failed to get DBaaS config")
		return argodbaas.DBaaSConfig{}, fmt.Errorf("failed to get dbaas config")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err).Msg("Error reading response body")
		return argodbaas.DBaaSConfig{}, fmt.Errorf("error reading response body")
	}

	var res argodbaas.DBaaSConfig
	if err := json.Unmarshal(body, &res); err != nil {
		log.Error().Err(err).Str("request-url", url).Msg("Failed to unmarshal dbaas config")
		return argodbaas.DBaaSConfig{}, fmt.Errorf("error unmarshaling dbaas config")
	}

	return res, nil
}

// combineListResult regroups the results coming from the database and from the
// AZ controllers. Databases with no product row are GitOps-managed.
func combineListResult(
	concatResults map[string][]interface{},
	resources []model.Product,
	mapResourceCheck map[uuid.UUID]bool,
) []DBaaSFullResponse {
	combineResults := make([]DBaaSFullResponse, 0)

	for azCode, results := range concatResults {
		for _, result := range results {
			mapResult, ok := result.(map[string]interface{})
			if !ok {
				continue
			}

			found := false
			for _, p := range resources {
				if p.ID.String() != mapResult["id"] {
					continue
				}
				combineResults = append(combineResults, DBaaSFullResponse{
					ProductResponse: ProductResponse{
						ID:            p.ID.String(),
						EId:           asString(mapResult["eid"]),
						ProductName:   p.ProductName,
						CodeAZ:        azCode, // we use az code to handle products under PRA
						ProductTypeId: p.ProductTypeId,
						Gitops:        asString(mapResult["gitops"]),
					},
					Database: mapResult["database"],
				})
				mapResourceCheck[p.ID] = true
				found = true
				break
			}

			// If only gitops
			if !found {
				combineResults = append(combineResults, DBaaSFullResponse{
					ProductResponse: ProductResponse{
						ID:          asString(mapResult["id"]),
						EId:         asString(mapResult["eid"]),
						ProductName: asString(mapResult["productName"]),
						CodeAZ:      azCode,
						Gitops:      asString(mapResult["gitops"]),
					},
					Database: mapResult["database"],
				})
			}
		}
	}

	// Check for not found resources
	for _, p := range resources {
		if !mapResourceCheck[p.ID] {
			combineResults = append(combineResults, DBaaSFullResponse{
				ProductResponse: ProductResponse{
					ID:            p.ID.String(),
					EId:           p.EffectiveID,
					ProductName:   p.ProductName,
					CodeAZ:        p.CodeAZ,
					ProductTypeId: p.ProductTypeId,
					Gitops:        "false",
				},
			})
		}
	}

	slices.SortFunc(combineResults, func(a, b DBaaSFullResponse) int {
		return controller.CompareProductResult(a.ProductResponse, b.ProductResponse)
	})

	return combineResults
}

func asString(v interface{}) string {
	s, _ := v.(string)
	return s
}

func writeJSON(w http.ResponseWriter, r *http.Request, payload interface{}) {
	log := logger.GetLogger(r.Context())

	w.Header().Set("Content-Type", "application/json")
	marshal, err := json.Marshal(payload)
	if err != nil {
		log.Err(err).Msg(consts.SpxResponseParseFailure)
		httpError.Http(w, r, consts.SpxResponseParseFailureCode).Msg(consts.SpxResponseParseFailure)
		return
	}
	_, _ = w.Write(marshal)
}
