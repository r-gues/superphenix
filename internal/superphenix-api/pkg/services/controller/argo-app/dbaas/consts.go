package dbaas

const (
	// AppPrefix is the ArgoCD Application name prefix for DBaaS products.
	AppPrefix = "dbaas"

	// EnginePostgreSQL is the only engine supported today.
	EnginePostgreSQL = "postgresql"

	// PostgresPort is the port a PostgreSQL database listens on.
	PostgresPort = 5432

	// LoadBalancerCIDR is the range VIPs must belong to. It matches the
	// constraint enforced by the load balancer product in superphenix-controller.
	LoadBalancerCIDR = "198.18.0.0/16"

	quantityFormat = "%dGi"
	cpuFormat      = "%d"

	minInstances = 1
	maxInstances = 5

	minStorageGi = 1
	maxStorageGi = 2048

	defaultBootstrapDatabase = "app"
	defaultBootstrapOwner    = "app"
)

var (
	// CpuValueList and MemoryValueList mirror the KaaS enums so a database sizes
	// like every other compute product on the platform. Memory is in GiB.
	CpuValueList    = []int{1, 2, 4, 8, 16, 32}
	MemoryValueList = []int{1, 2, 4, 8, 16, 32, 64}

	// AllowedParameters is the allow-list of postgresql.conf settings a tenant
	// may override. Anything touching storage layout, replication or the file
	// system is deliberately absent: CloudNativePG owns those.
	AllowedParameters = []string{
		"autovacuum_max_workers",
		"autovacuum_vacuum_cost_limit",
		"effective_cache_size",
		"effective_io_concurrency",
		"log_min_duration_statement",
		"maintenance_work_mem",
		"max_connections",
		"max_parallel_workers",
		"max_parallel_workers_per_gather",
		"max_worker_processes",
		"random_page_cost",
		"statement_timeout",
		"work_mem",
	}
)
