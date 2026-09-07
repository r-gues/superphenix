package dbaas

// ---------------------------------------------------------------------------
// Request DTOs (JSON) — what the API accepts from the console/CLI.
// ---------------------------------------------------------------------------

// DBaaSSpec is the user-facing description of a managed database.
type DBaaSSpec struct {
	// Engine is the database engine. Only "postgresql" is supported today.
	Engine string `json:"engine"`
	// Version is the engine major version, e.g. "17".
	Version string `json:"version"`
	// Instances is the number of PostgreSQL instances (1 primary + N replicas).
	Instances int `json:"instances"`
	// Cpu is the number of cores allocated to each instance.
	Cpu int `json:"cpu"`
	// Memory is the memory in GiB allocated to each instance.
	Memory int `json:"memory"`
	// Storage describes the data volume of each instance.
	Storage StorageSpec `json:"storage"`
	// WalStorage is the optional dedicated WAL volume of each instance.
	WalStorage *StorageSpec `json:"walStorage,omitempty"`
	// Bootstrap names the application database created at initialisation.
	Bootstrap BootstrapSpec `json:"bootstrap"`
	// Parameters are postgresql.conf overrides, restricted to an allow-list.
	Parameters map[string]string `json:"parameters,omitempty"`
	// Network describes how the database is reachable.
	Network NetworkSpec `json:"network,omitempty"`
}

// StorageSpec is a persistent volume request. Size is in GiB.
type StorageSpec struct {
	Size         int    `json:"size"`
	StorageClass string `json:"storageClass,omitempty"`
}

// BootstrapSpec is the initdb configuration.
type BootstrapSpec struct {
	Database string `json:"database"`
	Owner    string `json:"owner"`
}

// NetworkSpec describes the database's reachability. An empty Vip means the
// database is only reachable through its in-cluster service name.
type NetworkSpec struct {
	// Vip is an address in the load-balancer CIDR backing a kube-ovn
	// SwitchLBRule in front of the primary.
	Vip string `json:"vip,omitempty"`
	// AllowedCidrs are extra ingress CIDRs allowed on the PostgreSQL port, on
	// top of the project itself.
	AllowedCidrs []string `json:"allowedCidrs,omitempty"`
	// PublicAccess optionally publishes the Vip through an EIP the project
	// already owns.
	PublicAccess *PublicAccessSpec `json:"publicAccess,omitempty"`
}

// PublicAccessSpec references an existing EIP product by its local ID.
type PublicAccessSpec struct {
	EipLocalID   string `json:"eipLocalId"`
	ExternalPort int    `json:"externalPort,omitempty"`
}

// DBaaSConfig is the AZ-provided configuration the API needs to render a
// database: the storage classes available on that AZ.
type DBaaSConfig struct {
	StorageClasses []ClassMapping `json:"storageClasses"`
}

// ClassMapping is one storage class as exposed by the AZ controller.
type ClassMapping struct {
	Shortname string `json:"name"`
	Fullname  string `json:"fullname"`
}

// ---------------------------------------------------------------------------
// Helm values (YAML) — what is handed to the sfs-dbaas chart.
// ---------------------------------------------------------------------------

// Values is the root of the sfs-dbaas values document.
type Values struct {
	Databases map[string]Database `yaml:"databases,omitempty"`
}

// Database is one entry of `.Values.databases`, keyed by local ID.
type Database struct {
	Name       string            `yaml:"name"`
	Location   string            `yaml:"location"`
	Engine     string            `yaml:"engine"`
	Version    string            `yaml:"version"`
	Image      string            `yaml:"image,omitempty"`
	Instances  int               `yaml:"instances"`
	Resources  Resources         `yaml:"resources"`
	Storage    Storage           `yaml:"storage"`
	WalStorage *Storage          `yaml:"walStorage,omitempty"`
	Bootstrap  Bootstrap         `yaml:"bootstrap"`
	Parameters map[string]string `yaml:"parameters,omitempty"`
	Network    Network           `yaml:"network,omitempty"`
}

// Resources is the per-instance CPU/memory request, formatted as quantities.
type Resources struct {
	Cpu    string `yaml:"cpu"`
	Memory string `yaml:"memory"`
}

// Storage is a rendered volume request; Size is a resource.Quantity string.
type Storage struct {
	Size         string `yaml:"size"`
	StorageClass string `yaml:"storageClass,omitempty"`
}

// Bootstrap is the rendered initdb configuration.
type Bootstrap struct {
	Database string `yaml:"database"`
	Owner    string `yaml:"owner"`
}

// Network is the rendered reachability configuration.
type Network struct {
	Vip          string        `yaml:"vip,omitempty"`
	AllowedCidrs []string      `yaml:"allowedCidrs,omitempty"`
	PublicAccess *PublicAccess `yaml:"publicAccess,omitempty"`
}

// PublicAccess is the rendered EIP publication.
type PublicAccess struct {
	EipLocalID   string `yaml:"eipLocalId"`
	ExternalPort int    `yaml:"externalPort"`
}
