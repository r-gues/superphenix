package dbaas

import (
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"
	argodbaas "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/argo-app/dbaas"
)

// DTOs shared with / owned by the controller kit; aliased so handler code and
// swagger annotations reference them unqualified.
type (
	ProductResponse     = controller.ProductResponse
	DBaaSFullResponse   = controller.DBaaSFullResponse
	AppSpecFullResponse = controller.AppSpecFullResponse
)

type CreateDBaaSBody struct {
	General struct {
		ProductName string `json:"productName" validate:"max=63"`
	} `json:"general"`
	Spec argodbaas.DBaaSSpec `json:"spec"`
}

type UpdateDBaaSBody struct {
	General struct {
		ProductName string `json:"productName" validate:"max=63"`
	} `json:"general"`
	Spec argodbaas.DBaaSSpec `json:"spec"`
}
