package dbaas

import (
	"context"
	"fmt"
	"net"
	"slices"
	"sort"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	// Use v2 because v3 indent with 4 spaces instead of 2
	"gopkg.in/yaml.v2"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// CreateAppValues validates spec and renders the sfs-dbaas Helm values for a
// single database. oldSpec is the currently deployed spec when updating, and is
// nil on creation; it is used to reject changes the underlying storage cannot
// perform.
func CreateAppValues(
	ctx context.Context,
	localId, location string,
	spec DBaaSSpec,
	dbConfig DBaaSConfig,
	oldSpec *DBaaSSpec,
) (string, error) {
	log := logger.GetLogger(ctx)

	if err := validateEngine(spec); err != nil {
		log.Error().Err(err).Any("spec", spec).Msg("Invalid engine")
		return "", err
	}

	image, err := resolveVersion(spec.Version)
	if err != nil {
		log.Error().Err(err).Str("version", spec.Version).Msg("Unsupported PostgreSQL version")
		return "", err
	}

	if err := validateSizing(spec); err != nil {
		log.Error().Err(err).Any("spec", spec).Msg("Invalid sizing")
		return "", err
	}

	storage, err := buildStorage(spec.Storage, storageOf(oldSpec), dbConfig, "storage")
	if err != nil {
		log.Error().Err(err).Any("spec", spec).Msg("Invalid storage")
		return "", err
	}

	var walStorage *Storage
	if spec.WalStorage != nil {
		w, err := buildStorage(*spec.WalStorage, walStorageOf(oldSpec), dbConfig, "walStorage")
		if err != nil {
			log.Error().Err(err).Any("spec", spec).Msg("Invalid WAL storage")
			return "", err
		}
		walStorage = &w
	} else if oldSpec != nil && oldSpec.WalStorage != nil {
		// CloudNativePG cannot drop a WAL volume from a running cluster.
		err := fmt.Errorf("walStorage cannot be removed once enabled")
		log.Error().Err(err).Any("spec", spec).Msg("Invalid WAL storage")
		return "", err
	}

	parameters, err := buildParameters(spec.Parameters)
	if err != nil {
		log.Error().Err(err).Any("parameters", spec.Parameters).Msg("Invalid parameters")
		return "", err
	}

	network, err := buildNetwork(spec.Network)
	if err != nil {
		log.Error().Err(err).Any("network", spec.Network).Msg("Invalid network")
		return "", err
	}

	bootstrap := resolveBootstrap(spec.Bootstrap)
	if oldSpec != nil {
		// initdb only runs once; letting the values drift would silently do nothing.
		if bootstrap != resolveBootstrap(oldSpec.Bootstrap) {
			err := fmt.Errorf("bootstrap database and owner cannot be changed after creation")
			log.Error().Err(err).Any("spec", spec).Msg("Invalid bootstrap")
			return "", err
		}
	}

	valuesObj := Values{
		Databases: map[string]Database{
			localId: {
				Name:      localId,
				Location:  location,
				Engine:    EnginePostgreSQL,
				Version:   spec.Version,
				Image:     image,
				Instances: spec.Instances,
				Resources: Resources{
					Cpu:    fmt.Sprintf(cpuFormat, spec.Cpu),
					Memory: fmt.Sprintf(quantityFormat, spec.Memory),
				},
				Storage:    storage,
				WalStorage: walStorage,
				Bootstrap:  bootstrap,
				Parameters: parameters,
				Network:    network,
			},
		},
	}

	valuesBytes, err := yaml.Marshal(valuesObj)
	if err != nil {
		return "", err
	}

	return string(valuesBytes), nil
}

// CreateArgoApp builds the ArgoCD Application description for a DBaaS product.
func CreateArgoApp(
	ctx context.Context,
	localId string,
	az config.AZConfig,
	spec DBaaSSpec,
	metadata spxId.Metadata,
	dbConfig DBaaSConfig,
	oldSpec *DBaaSSpec,
) (argo.CreateAppInfo, error) {
	log := logger.GetLogger(ctx)

	values, err := CreateAppValues(ctx, localId, az.Code, spec, dbConfig, oldSpec)
	if err != nil {
		log.Err(err).Msg("Failed to create app values")
		return argo.CreateAppInfo{}, err
	}

	repo, _, ok := config.ResolvePostgresVersion(
		config.Global.ProductsConfig.ArgoApp.Database.PostgresVersions,
		config.Global.ProductsConfig.ArgoApp.Database.Repo,
		spec.Version,
	)
	if !ok {
		err := fmt.Errorf("postgres version not supported: %s", spec.Version)
		log.Error().Err(err).Str("version", spec.Version).Msg("Failed to resolve repo")
		return argo.CreateAppInfo{}, err
	}

	helmParams := fmt.Sprintf(
		"--set location=%s  --set organizationID=%s  --set projectID=%s",
		az.Code, metadata.OrgId, metadata.ProjectId,
	)

	appName := fmt.Sprintf("%s-%s", AppPrefix, metadata.GetResourceEffectiveID())

	return argo.CreateAppInfo{
		Metadata: metadata,
		General: argo.AppGeneral{
			AppName:     appName,
			Destination: az.Destination,
		},
		Spec: argo.AppSpec{
			Source: argo.AppSource{
				RepoURL:        repo.RepoURL,
				TargetRevision: repo.TargetRevision,
				Chart:          repo.Chart,
				Path:           repo.Path,
				Plugin: v1alpha1.ApplicationSourcePlugin{
					Name: "uuidv5",
					Env: v1alpha1.Env{
						{Name: "RELEASE", Value: appName},
						{Name: "REPO", Value: ""},
						{Name: "HELM_PARAMS", Value: helmParams},
						{Name: "HELM_VALUEFILES", Value: ""},
						{Name: "HELM_VALUES", Value: values},
					},
				},
			},
		},
	}, nil
}

func validateEngine(spec DBaaSSpec) error {
	// An empty engine is accepted and defaulted: PostgreSQL is the only one.
	if spec.Engine != "" && spec.Engine != EnginePostgreSQL {
		return fmt.Errorf("engine not supported: %s", spec.Engine)
	}
	return nil
}

// resolveVersion checks the version against the configured catalogue and
// returns the image override for it, if any.
func resolveVersion(version string) (string, error) {
	_, image, ok := config.ResolvePostgresVersion(
		config.Global.ProductsConfig.ArgoApp.Database.PostgresVersions,
		config.Global.ProductsConfig.ArgoApp.Database.Repo,
		version,
	)
	if !ok {
		return "", fmt.Errorf("postgres version not supported: %s", version)
	}
	return image, nil
}

func validateSizing(spec DBaaSSpec) error {
	if spec.Instances < minInstances || spec.Instances > maxInstances {
		return fmt.Errorf("instances count value (%d) must be between %d and %d",
			spec.Instances, minInstances, maxInstances)
	}
	if !slices.Contains(CpuValueList, spec.Cpu) {
		return fmt.Errorf("cpu value (%d) is not one of %v", spec.Cpu, CpuValueList)
	}
	if !slices.Contains(MemoryValueList, spec.Memory) {
		return fmt.Errorf("memory value (%d) is not one of %v", spec.Memory, MemoryValueList)
	}
	return nil
}

// buildStorage validates one volume request and renders it. old is the
// previously deployed request for the same volume, or nil on creation.
func buildStorage(spec StorageSpec, old *StorageSpec, dbConfig DBaaSConfig, field string) (Storage, error) {
	if spec.Size < minStorageGi || spec.Size > maxStorageGi {
		return Storage{}, fmt.Errorf("%s size (%dGi) must be between %dGi and %dGi",
			field, spec.Size, minStorageGi, maxStorageGi)
	}

	// PVC-backed storage cannot be shrunk.
	if old != nil && spec.Size < old.Size {
		return Storage{}, fmt.Errorf("%s size cannot be reduced (current %dGi, requested %dGi)",
			field, old.Size, spec.Size)
	}

	storageClass := spec.StorageClass
	if storageClass == "" {
		if len(dbConfig.StorageClasses) == 0 {
			return Storage{}, fmt.Errorf("no storage class configured on this availability zone")
		}
		storageClass = dbConfig.StorageClasses[0].Shortname // default = first, mirrors KaaS essentials
	} else if !slices.ContainsFunc(dbConfig.StorageClasses, func(c ClassMapping) bool {
		return c.Shortname == storageClass
	}) {
		return Storage{}, fmt.Errorf("%s storage class not available: %s", field, storageClass)
	}

	// Changing the storage class of a bound PVC has no effect.
	if old != nil && old.StorageClass != "" && old.StorageClass != storageClass {
		return Storage{}, fmt.Errorf("%s storage class cannot be changed after creation", field)
	}

	q, err := resource.ParseQuantity(fmt.Sprintf(quantityFormat, spec.Size))
	if err != nil {
		return Storage{}, fmt.Errorf("invalid %s size: %w", field, err)
	}

	return Storage{Size: q.String(), StorageClass: storageClass}, nil
}

func buildParameters(params map[string]string) (map[string]string, error) {
	if len(params) == 0 {
		return nil, nil
	}

	// Sort so the error message is deterministic and the rendered values are stable.
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	rejected := make([]string, 0)
	for _, k := range keys {
		if !slices.Contains(AllowedParameters, k) {
			rejected = append(rejected, k)
		}
	}
	if len(rejected) > 0 {
		return nil, fmt.Errorf("parameters not allowed: %s", strings.Join(rejected, ", "))
	}

	out := make(map[string]string, len(params))
	for _, k := range keys {
		out[k] = params[k]
	}
	return out, nil
}

func buildNetwork(spec NetworkSpec) (Network, error) {
	network := Network{AllowedCidrs: spec.AllowedCidrs}

	for _, c := range spec.AllowedCidrs {
		if _, _, err := net.ParseCIDR(c); err != nil {
			return Network{}, fmt.Errorf("allowedCidrs entry is not a valid CIDR: %s", c)
		}
	}

	if spec.Vip != "" {
		vip := net.ParseIP(spec.Vip)
		if vip == nil {
			return Network{}, fmt.Errorf("vip is not a valid IP address: %s", spec.Vip)
		}
		_, cidr, err := net.ParseCIDR(LoadBalancerCIDR)
		if err != nil {
			return Network{}, err
		}
		if !cidr.Contains(vip) {
			return Network{}, fmt.Errorf("vip must be within %s", LoadBalancerCIDR)
		}
		network.Vip = spec.Vip
	}

	if spec.PublicAccess != nil {
		if network.Vip == "" {
			return Network{}, fmt.Errorf("publicAccess requires a vip")
		}
		if spec.PublicAccess.EipLocalID == "" {
			return Network{}, fmt.Errorf("publicAccess requires an eipLocalId")
		}
		port := spec.PublicAccess.ExternalPort
		if port == 0 {
			port = PostgresPort
		}
		if port < 1 || port > 65535 {
			return Network{}, fmt.Errorf("publicAccess externalPort (%d) is out of range", port)
		}
		network.PublicAccess = &PublicAccess{
			EipLocalID:   spec.PublicAccess.EipLocalID,
			ExternalPort: port,
		}
	}

	return network, nil
}

func storageOf(spec *DBaaSSpec) *StorageSpec {
	if spec == nil {
		return nil
	}
	s := spec.Storage
	return &s
}

func walStorageOf(spec *DBaaSSpec) *StorageSpec {
	if spec == nil {
		return nil
	}
	return spec.WalStorage
}

// resolveBootstrap fills in the defaults CloudNativePG would otherwise pick
// itself, so a spec and its stored counterpart compare equal whether or not the
// caller spelled them out.
func resolveBootstrap(spec BootstrapSpec) Bootstrap {
	bootstrap := Bootstrap(spec)
	if bootstrap.Database == "" {
		bootstrap.Database = defaultBootstrapDatabase
	}
	if bootstrap.Owner == "" {
		bootstrap.Owner = defaultBootstrapOwner
	}
	return bootstrap
}
