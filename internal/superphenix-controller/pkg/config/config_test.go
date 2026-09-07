package config

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

// loadTestDefaults resets Global and loads the hardcoded defaults.
func loadTestDefaults(t *testing.T) {
	t.Helper()

	Global = Config{}
	v.SetConfigType("yaml")
	if err := loadDefaults(); err != nil {
		t.Fatalf("loadDefaults() returned an unexpected error: %v", err)
	}
}

// loadTestUserConfig loads raw as the user supplied config.yaml.
func loadTestUserConfig(t *testing.T, raw string) error {
	t.Helper()

	if err := v.ReadConfig(strings.NewReader(raw)); err != nil {
		t.Fatalf("ReadConfig() returned an unexpected error: %v", err)
	}
	return v.Unmarshal(&Global)
}

// TestConfigCaseSensitiveKeys guards against viper lowercasing the keys of
// the annotation and label entry lists.
func TestConfigCaseSensitiveKeys(t *testing.T) {
	defaultAnnotations := map[string]string{
		"v1.multus-cni.io/default-network":   "kube-system/system-isolated-egress",
		"cdi.kubevirt.io/allowClaimAdoption": "true",
	}
	// Compared as a list, not a map: several entries legitimately share a key
	// (every self-service chart is identified by app.kubernetes.io/name), and a
	// map would silently drop all but the last of them.
	defaultLabels := KeyValueList{
		{Key: "app.kubernetes.io/name", Value: "sfs-kaas"},
		{Key: "app.kubernetes.io/name", Value: "sfs-dbaas"},
	}

	tests := []struct {
		name            string
		raw             string
		wantAnnotations map[string]string
		wantLabels      KeyValueList
		wantMtu         int
		wantMtuAuto     bool
	}{
		{
			name:            "defaults preserve key case",
			raw:             "azName: az1\n",
			wantAnnotations: defaultAnnotations,
			wantLabels:      defaultLabels,
			wantMtuAuto:     true,
		},
		{
			name: "user annotations preserve key case and replace the defaults",
			raw: `
productsConfig:
  datavolume:
    defaultAnnotations:
      - key: "my.custom/Annotation"
        value: "yes"
`,
			wantAnnotations: map[string]string{"my.custom/Annotation": "yes"},
			wantLabels:      defaultLabels,
			wantMtuAuto:     true,
		},
		{
			name: "user labels preserve key case and replace the defaults",
			raw: `
disableEditionForResourcesByLabels:
  - key: "superphenix.net/managedBy"
    value: "operator"
`,
			wantAnnotations: defaultAnnotations,
			wantLabels:      KeyValueList{{Key: "superphenix.net/managedBy", Value: "operator"}},
			wantMtuAuto:     true,
		},
		{
			name: "user mtu override",
			raw: `
productsConfig:
  subnets:
    mtu: 1400
    mtuAutodetection: false
`,
			wantAnnotations: defaultAnnotations,
			wantLabels:      defaultLabels,
			wantMtu:         1400,
			wantMtuAuto:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loadTestDefaults(t)

			if tt.wantMtu == 0 {
				tt.wantMtu = 1500
			}

			if err := loadTestUserConfig(t, tt.raw); err != nil {
				t.Fatalf("Unmarshal() returned an unexpected error: %v", err)
			}

			if got := Global.ProductsConfig.Subnets.Mtu; got != tt.wantMtu {
				t.Errorf("mtu = %d, want %d", got, tt.wantMtu)
			}
			if got := Global.ProductsConfig.Subnets.MtuAutodetection; got != tt.wantMtuAuto {
				t.Errorf("mtuAutodetection = %v, want %v", got, tt.wantMtuAuto)
			}

			if got := Global.ProductsConfig.Datavolume.DefaultAnnotations.Map(); !maps.Equal(got, tt.wantAnnotations) {
				t.Errorf("datavolume annotations = %v, want %v", got, tt.wantAnnotations)
			}
			if got := Global.DisableEditionForResourcesByLabels; !slices.Equal(got, tt.wantLabels) {
				t.Errorf("disableEditionForResourcesByLabels = %v, want %v", got, tt.wantLabels)
			}
		})
	}
}
