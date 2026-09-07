package view

import (
	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

// DatabaseView is the projection of a CloudNativePG Cluster returned to clients.
// It is hand-rolled rather than reusing the upstream type so the controller does
// not depend on the operator's Go module.
type DatabaseView struct {
	ObjectMeta `json:"metadata,omitempty"`

	Spec   DatabaseSpecView   `json:"spec"`
	Status DatabaseStatusView `json:"status"`
}

// DatabaseSpecView is the subset of the Cluster spec a tenant cares about.
type DatabaseSpecView struct {
	// Engine is always "postgresql" today; it is surfaced so clients can branch
	// on it once a second engine exists.
	Engine string `json:"engine"`
	// Version is the PostgreSQL major version, derived from the image.
	Version string `json:"version"`
	// ImageName is the container image actually running.
	ImageName string `json:"imageName,omitempty"`
	// Instances is the requested number of PostgreSQL instances.
	Instances int `json:"instances"`
	// Resources are the per-instance CPU/memory requests.
	Resources DatabaseResourcesView `json:"resources"`
	// Storage is the data volume of each instance.
	Storage DatabaseStorageView `json:"storage"`
	// WalStorage is the dedicated WAL volume, when one was requested.
	WalStorage *DatabaseStorageView `json:"walStorage,omitempty"`
	// Parameters are the effective postgresql.conf overrides.
	Parameters map[string]string `json:"parameters,omitempty"`
}

// DatabaseResourcesView is a per-instance resource request.
type DatabaseResourcesView struct {
	Cpu    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// DatabaseStorageView is a volume request. StorageClass is reported with the
// friendly short name, like every other storage-bearing product.
type DatabaseStorageView struct {
	Size         string `json:"size,omitempty"`
	StorageClass string `json:"storageClass,omitempty"`
}

// DatabaseStatusView is the subset of the Cluster status a tenant cares about.
type DatabaseStatusView struct {
	// Phase is CloudNativePG's human-readable cluster phase.
	Phase string `json:"phase,omitempty"`
	// PhaseReason details Phase when the cluster is not healthy.
	PhaseReason string `json:"phaseReason,omitempty"`
	// Instances is the number of instances that actually exist.
	Instances int `json:"instances"`
	// ReadyInstances is the number of instances currently serving.
	ReadyInstances int `json:"readyInstances"`
	// CurrentPrimary is the pod currently accepting writes.
	CurrentPrimary string `json:"currentPrimary,omitempty"`
	// TargetPrimary is the pod the operator is failing over to, when different.
	TargetPrimary string `json:"targetPrimary,omitempty"`
	// WriteService and ReadService are the in-cluster service names.
	WriteService string `json:"writeService,omitempty"`
	ReadService  string `json:"readService,omitempty"`
}

// DatabaseCredentialsView is the connection information of a database. It is
// only ever returned by the credentials endpoint, which carries its own
// permission.
type DatabaseCredentialsView struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
	// Uri is the ready-to-use libpq connection string.
	Uri string `json:"uri"`
}

// DBaaSToResource maps a database onto the generic product envelope.
func DBaaSToResource(database DatabaseView) DBaaS {
	return DBaaS{
		Resource: Resource{
			ID:          database.Labels[spxId.SpxLabelResourceLocalID],
			EId:         database.Name,
			ProductName: database.Labels[spxId.SpxLabelResourceName],
			Gitops:      database.Labels[spxId.SpxLabelGitops],
		},
		Database: database,
	}
}
