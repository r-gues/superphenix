package dbaas

import (
	"context"
	"strings"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo/view"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

const (
	testLocalId  = "test-db"
	testLocation = "test-loc"
)

func testConfig() DBaaSConfig {
	return DBaaSConfig{
		StorageClasses: []ClassMapping{
			{Shortname: "sc1", Fullname: "storage-class-1"},
			{Shortname: "sc2", Fullname: "storage-class-2"},
		},
	}
}

func setupVersions(t *testing.T) {
	t.Helper()
	config.Global.ProductsConfig.ArgoApp.Database.Repo = config.RepoArgoAppConfig{
		RepoURL:        "ghcr.io/super-phenix/charts",
		TargetRevision: "1.0.0",
		Chart:          "sfs-dbaas",
	}
	config.Global.ProductsConfig.ArgoApp.Database.PostgresVersions = []config.PostgresVersionConfig{
		{Version: "16"},
		{Version: "17", Image: "ghcr.io/cloudnative-pg/postgresql:17.2"},
	}
}

func validSpec() DBaaSSpec {
	return DBaaSSpec{
		Engine:    EnginePostgreSQL,
		Version:   "17",
		Instances: 3,
		Cpu:       2,
		Memory:    8,
		Storage:   StorageSpec{Size: 50, StorageClass: "sc1"},
		Bootstrap: BootstrapSpec{Database: "app", Owner: "app"},
	}
}

func TestCreateAppValues(t *testing.T) {
	setupVersions(t)
	ctx := context.Background()

	withSpec := func(mutate func(*DBaaSSpec)) DBaaSSpec {
		s := validSpec()
		mutate(&s)
		return s
	}

	tests := []struct {
		name            string
		spec            DBaaSSpec
		oldSpec         *DBaaSSpec
		wantErr         bool
		errContains     string
		expectedStrings []string
	}{
		{
			name: "valid spec renders the database",
			spec: validSpec(),
			expectedStrings: []string{
				"databases:",
				"  " + testLocalId + ":",
				"    name: " + testLocalId,
				"    location: " + testLocation,
				"    engine: postgresql",
				`    version: "17"`,
				"    image: ghcr.io/cloudnative-pg/postgresql:17.2",
				"    instances: 3",
				`      cpu: "2"`,
				"      memory: 8Gi",
				"      size: 50Gi",
				"      storageClass: sc1",
				"      database: app",
				"      owner: app",
			},
		},
		{
			name:            "empty engine defaults to postgresql",
			spec:            withSpec(func(s *DBaaSSpec) { s.Engine = "" }),
			expectedStrings: []string{"engine: postgresql"},
		},
		{
			name:            "empty bootstrap is defaulted",
			spec:            withSpec(func(s *DBaaSSpec) { s.Bootstrap = BootstrapSpec{} }),
			expectedStrings: []string{"database: app", "owner: app"},
		},
		{
			name:            "empty storage class falls back to the first configured one",
			spec:            withSpec(func(s *DBaaSSpec) { s.Storage.StorageClass = "" }),
			expectedStrings: []string{"storageClass: sc1"},
		},
		{
			name:            "wal storage is rendered when requested",
			spec:            withSpec(func(s *DBaaSSpec) { s.WalStorage = &StorageSpec{Size: 10, StorageClass: "sc1"} }),
			expectedStrings: []string{"walStorage:", "size: 10Gi"},
		},
		{
			name: "allowed parameters are rendered",
			spec: withSpec(func(s *DBaaSSpec) {
				s.Parameters = map[string]string{"max_connections": "200", "work_mem": "16MB"}
			}),
			expectedStrings: []string{"parameters:", `max_connections: "200"`, "work_mem: 16MB"},
		},
		{
			name:            "vip renders the network block",
			spec:            withSpec(func(s *DBaaSSpec) { s.Network = NetworkSpec{Vip: "198.18.0.10"} }),
			expectedStrings: []string{"network:", "vip: 198.18.0.10"},
		},
		{
			name: "public access renders the eip reference and defaults the port",
			spec: withSpec(func(s *DBaaSSpec) {
				s.Network = NetworkSpec{
					Vip:          "198.18.0.10",
					PublicAccess: &PublicAccessSpec{EipLocalID: "my-eip"},
				}
			}),
			expectedStrings: []string{"publicAccess:", "eipLocalId: my-eip", "externalPort: 5432"},
		},
		{
			name:        "unsupported engine is rejected",
			spec:        withSpec(func(s *DBaaSSpec) { s.Engine = "mysql" }),
			wantErr:     true,
			errContains: "engine not supported",
		},
		{
			name:        "unsupported version is rejected",
			spec:        withSpec(func(s *DBaaSSpec) { s.Version = "12" }),
			wantErr:     true,
			errContains: "postgres version not supported",
		},
		{
			name:        "too many instances is rejected",
			spec:        withSpec(func(s *DBaaSSpec) { s.Instances = maxInstances + 1 }),
			wantErr:     true,
			errContains: "instances count value",
		},
		{
			name:        "zero instances is rejected",
			spec:        withSpec(func(s *DBaaSSpec) { s.Instances = 0 }),
			wantErr:     true,
			errContains: "instances count value",
		},
		{
			name:        "cpu outside the enum is rejected",
			spec:        withSpec(func(s *DBaaSSpec) { s.Cpu = 3 }),
			wantErr:     true,
			errContains: "cpu value",
		},
		{
			name:        "memory outside the enum is rejected",
			spec:        withSpec(func(s *DBaaSSpec) { s.Memory = 3 }),
			wantErr:     true,
			errContains: "memory value",
		},
		{
			name:        "storage below the minimum is rejected",
			spec:        withSpec(func(s *DBaaSSpec) { s.Storage.Size = 0 }),
			wantErr:     true,
			errContains: "storage size",
		},
		{
			name:        "storage above the maximum is rejected",
			spec:        withSpec(func(s *DBaaSSpec) { s.Storage.Size = maxStorageGi + 1 }),
			wantErr:     true,
			errContains: "storage size",
		},
		{
			name:        "unknown storage class is rejected",
			spec:        withSpec(func(s *DBaaSSpec) { s.Storage.StorageClass = "nope" }),
			wantErr:     true,
			errContains: "storage class not available",
		},
		{
			name: "parameter outside the allow-list is rejected",
			spec: withSpec(func(s *DBaaSSpec) {
				s.Parameters = map[string]string{"shared_buffers": "1GB", "archive_mode": "on"}
			}),
			wantErr:     true,
			errContains: "parameters not allowed: archive_mode, shared_buffers",
		},
		{
			name:        "vip outside the load balancer CIDR is rejected",
			spec:        withSpec(func(s *DBaaSSpec) { s.Network = NetworkSpec{Vip: "10.0.0.1"} }),
			wantErr:     true,
			errContains: "vip must be within",
		},
		{
			name:        "malformed vip is rejected",
			spec:        withSpec(func(s *DBaaSSpec) { s.Network = NetworkSpec{Vip: "not-an-ip"} }),
			wantErr:     true,
			errContains: "vip is not a valid IP address",
		},
		{
			name:        "malformed allowed CIDR is rejected",
			spec:        withSpec(func(s *DBaaSSpec) { s.Network = NetworkSpec{AllowedCidrs: []string{"10.0.0.1"}} }),
			wantErr:     true,
			errContains: "not a valid CIDR",
		},
		{
			name: "public access without a vip is rejected",
			spec: withSpec(func(s *DBaaSSpec) {
				s.Network = NetworkSpec{PublicAccess: &PublicAccessSpec{EipLocalID: "my-eip"}}
			}),
			wantErr:     true,
			errContains: "publicAccess requires a vip",
		},
		{
			name: "public access without an eip is rejected",
			spec: withSpec(func(s *DBaaSSpec) {
				s.Network = NetworkSpec{Vip: "198.18.0.10", PublicAccess: &PublicAccessSpec{}}
			}),
			wantErr:     true,
			errContains: "publicAccess requires an eipLocalId",
		},
		{
			name:            "growing storage is allowed",
			spec:            withSpec(func(s *DBaaSSpec) { s.Storage.Size = 100 }),
			oldSpec:         ptr(validSpec()),
			expectedStrings: []string{"size: 100Gi"},
		},
		{
			name:        "shrinking storage is rejected",
			spec:        withSpec(func(s *DBaaSSpec) { s.Storage.Size = 10 }),
			oldSpec:     ptr(validSpec()),
			wantErr:     true,
			errContains: "storage size cannot be reduced",
		},
		{
			name:        "changing the storage class is rejected",
			spec:        withSpec(func(s *DBaaSSpec) { s.Storage.StorageClass = "sc2" }),
			oldSpec:     ptr(validSpec()),
			wantErr:     true,
			errContains: "storage class cannot be changed",
		},
		{
			name:    "removing wal storage is rejected",
			spec:    validSpec(),
			oldSpec: ptr(withSpec(func(s *DBaaSSpec) { s.WalStorage = &StorageSpec{Size: 10, StorageClass: "sc1"} })),
			wantErr: true, errContains: "walStorage cannot be removed",
		},
		{
			name:        "changing the bootstrap database is rejected",
			spec:        withSpec(func(s *DBaaSSpec) { s.Bootstrap.Database = "other" }),
			oldSpec:     ptr(validSpec()),
			wantErr:     true,
			errContains: "bootstrap database and owner cannot be changed",
		},
		{
			name:            "scaling instances up is allowed",
			spec:            withSpec(func(s *DBaaSSpec) { s.Instances = 5 }),
			oldSpec:         ptr(validSpec()),
			expectedStrings: []string{"instances: 5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateAppValues(ctx, testLocalId, testLocation, tt.spec, testConfig(), tt.oldSpec)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got none (values: %s)", got)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, want := range tt.expectedStrings {
				if !strings.Contains(got, want) {
					t.Errorf("expected values to contain %q, got:\n%s", want, got)
				}
			}
		})
	}
}

func TestCreateArgoApp(t *testing.T) {
	setupVersions(t)
	ctx := context.Background()

	az := config.AZConfig{Code: testLocation, Destination: "spx-test-virt01"}
	metadata := spxId.Metadata{
		OrgId:     "6f5902ac-237c-4c1a-8f0c-1f4b1f4b1f4b",
		ProjectId: "0a8b1c2d-3e4f-4a5b-8c9d-0e1f2a3b4c5d",
	}
	if err := metadata.GenerateMetadata(metadata.ProjectId, metadata.OrgId, testLocalId); err != nil {
		t.Fatalf("failed to generate metadata: %v", err)
	}

	tests := []struct {
		name        string
		spec        DBaaSSpec
		wantErr     bool
		errContains string
	}{
		{name: "valid spec builds the application", spec: validSpec()},
		{
			name:        "unsupported version fails before the application is built",
			spec:        func() DBaaSSpec { s := validSpec(); s.Version = "9"; return s }(),
			wantErr:     true,
			errContains: "postgres version not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateArgoApp(ctx, testLocalId, az, tt.spec, metadata, testConfig(), nil)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error, got none")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			wantName := AppPrefix + "-" + metadata.GetResourceEffectiveID()
			if got.General.AppName != wantName {
				t.Errorf("expected app name %q, got %q", wantName, got.General.AppName)
			}
			if got.General.Destination != az.Destination {
				t.Errorf("expected destination %q, got %q", az.Destination, got.General.Destination)
			}
			if got.Spec.Source.Chart != "sfs-dbaas" {
				t.Errorf("expected chart sfs-dbaas, got %q", got.Spec.Source.Chart)
			}
			if got.Spec.Source.Plugin.Name != "uuidv5" {
				t.Errorf("expected the uuidv5 plugin, got %q", got.Spec.Source.Plugin.Name)
			}

			env := map[string]string{}
			for _, e := range got.Spec.Source.Plugin.Env {
				env[e.Name] = e.Value
			}
			if env["RELEASE"] != wantName {
				t.Errorf("expected RELEASE %q, got %q", wantName, env["RELEASE"])
			}
			if !strings.Contains(env["HELM_PARAMS"], "--set projectID="+metadata.ProjectId) {
				t.Errorf("expected HELM_PARAMS to carry the project ID, got %q", env["HELM_PARAMS"])
			}
			if !strings.Contains(env["HELM_VALUES"], "engine: postgresql") {
				t.Errorf("expected HELM_VALUES to carry the rendered database, got %q", env["HELM_VALUES"])
			}
		})
	}
}

func TestConvertAppToUpdateDBaaSSpec(t *testing.T) {
	setupVersions(t)
	ctx := context.Background()

	spec := validSpec()
	spec.WalStorage = &StorageSpec{Size: 10, StorageClass: "sc1"}
	spec.Parameters = map[string]string{"max_connections": "200"}
	spec.Network = NetworkSpec{
		Vip:          "198.18.0.10",
		AllowedCidrs: []string{"10.0.0.0/8"},
		PublicAccess: &PublicAccessSpec{EipLocalID: "my-eip", ExternalPort: 6543},
	}

	values, err := CreateAppValues(ctx, testLocalId, testLocation, spec, testConfig(), nil)
	if err != nil {
		t.Fatalf("failed to render values: %v", err)
	}

	tests := []struct {
		name    string
		app     view.AppView
		wantErr bool
		want    *DBaaSSpec
	}{
		{
			name:    "application without a source is rejected",
			app:     view.AppView{},
			wantErr: true,
		},
		{
			name:    "application without a plugin is rejected",
			app:     view.AppView{Spec: view.ApplicationSpec{Source: &view.ApplicationSource{}}},
			wantErr: true,
		},
		{
			name: "rendered values round-trip back to the spec",
			app: view.AppView{Spec: view.ApplicationSpec{Source: &view.ApplicationSource{
				Plugin: &v1alpha1.ApplicationSourcePlugin{
					Env: v1alpha1.Env{{Name: "HELM_VALUES", Value: values}},
				},
			}}},
			want: &spec,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertAppToUpdateDBaaSSpec(tt.app)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			want := *tt.want
			if got.Engine != want.Engine || got.Version != want.Version || got.Instances != want.Instances {
				t.Errorf("identity mismatch: got %+v, want %+v", got, want)
			}
			if got.Cpu != want.Cpu || got.Memory != want.Memory {
				t.Errorf("sizing mismatch: got cpu=%d memory=%d, want cpu=%d memory=%d",
					got.Cpu, got.Memory, want.Cpu, want.Memory)
			}
			if got.Storage != want.Storage {
				t.Errorf("storage mismatch: got %+v, want %+v", got.Storage, want.Storage)
			}
			if got.WalStorage == nil || *got.WalStorage != *want.WalStorage {
				t.Errorf("wal storage mismatch: got %+v, want %+v", got.WalStorage, want.WalStorage)
			}
			if got.Bootstrap != want.Bootstrap {
				t.Errorf("bootstrap mismatch: got %+v, want %+v", got.Bootstrap, want.Bootstrap)
			}
			if got.Parameters["max_connections"] != "200" {
				t.Errorf("parameters mismatch: got %+v", got.Parameters)
			}
			if got.Network.Vip != want.Network.Vip {
				t.Errorf("vip mismatch: got %q, want %q", got.Network.Vip, want.Network.Vip)
			}
			if got.Network.PublicAccess == nil || *got.Network.PublicAccess != *want.Network.PublicAccess {
				t.Errorf("public access mismatch: got %+v, want %+v",
					got.Network.PublicAccess, want.Network.PublicAccess)
			}

			// The round-tripped spec must itself still be valid input.
			if _, err := CreateAppValues(ctx, testLocalId, testLocation, got, testConfig(), nil); err != nil {
				t.Errorf("round-tripped spec is no longer valid: %v", err)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }
