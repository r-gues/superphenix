package view

import (
	"testing"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

func TestDBaaSToResource(t *testing.T) {
	tests := []struct {
		name     string
		database DatabaseView
		want     Resource
	}{
		{
			name: "a fully labelled database maps onto the product envelope",
			database: DatabaseView{
				ObjectMeta: ObjectMeta{
					Name: "spx-11111111-1111-1111-1111-111111111111",
					Labels: map[string]string{
						spxId.SpxLabelResourceLocalID: "prod-db",
						spxId.SpxLabelResourceName:    "Production database",
						spxId.SpxLabelGitops:          "false",
					},
				},
			},
			want: Resource{
				ID:          "prod-db",
				EId:         "spx-11111111-1111-1111-1111-111111111111",
				ProductName: "Production database",
				Gitops:      "false",
			},
		},
		{
			name: "a gitops database is reported as such",
			database: DatabaseView{
				ObjectMeta: ObjectMeta{
					Name: "spx-33333333-3333-3333-3333-333333333333",
					Labels: map[string]string{
						spxId.SpxLabelResourceLocalID: "from-git",
						spxId.SpxLabelResourceName:    "From git",
						spxId.SpxLabelGitops:          "true",
					},
				},
			},
			want: Resource{
				ID:          "from-git",
				EId:         "spx-33333333-3333-3333-3333-333333333333",
				ProductName: "From git",
				Gitops:      "true",
			},
		},
		{
			name:     "an unlabelled database still reports its effective ID",
			database: DatabaseView{ObjectMeta: ObjectMeta{Name: "spx-44444444-4444-4444-4444-444444444444"}},
			want:     Resource{EId: "spx-44444444-4444-4444-4444-444444444444"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DBaaSToResource(tt.database)
			if got.Resource != tt.want {
				t.Errorf("resource = %+v, want %+v", got.Resource, tt.want)
			}
			if got.Database.Name != tt.database.Name {
				t.Errorf("database was not carried through: %+v", got.Database)
			}
		})
	}
}
