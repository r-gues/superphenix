package audit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	auditEvent "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/audit-event"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type fakeStore struct {
	insertErr error

	inserted  []model.AuditEvent
	finalized []auditEvent.Completion
}

func (s *fakeStore) Insert(_ context.Context, event model.AuditEvent) (uuid.UUID, error) {
	if s.insertErr != nil {
		err := s.insertErr
		s.insertErr = nil
		return uuid.Nil, err
	}
	s.inserted = append(s.inserted, event)
	return uuid.New(), nil
}

func (s *fakeStore) Finalize(_ context.Context, _ uuid.UUID, completion auditEvent.Completion) error {
	s.finalized = append(s.finalized, completion)
	return nil
}

func TestMiddleware(t *testing.T) {
	orgId := uuid.New()
	projectId := uuid.New()
	userId := uuid.New()

	authenticate := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Begin(r.Context(), userId.String(), "JwtBearer")
			next.ServeHTTP(w, r)
		})
	}
	reject := func(code int) router.Middleware {
		return func(http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "", code) })
		}
	}

	tests := []struct {
		name        string
		chain       []router.Middleware
		handler     http.HandlerFunc
		insertErr   error
		path        string
		wantPanic   bool
		wantInserts []string // status of each inserted row
		wantFinal   *auditEvent.Completion
		wantResId   string
	}{
		{
			name:        "success finalizes the attempted row",
			chain:       []router.Middleware{authenticate},
			handler:     func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) },
			wantInserts: []string{model.AuditStatusAttempted},
			wantFinal:   &auditEvent.Completion{Status: model.AuditStatusSuccess, StatusCode: http.StatusOK},
			wantResId:   "spx-1",
		},
		{
			name:        "handler that writes nothing counts as 200",
			chain:       []router.Middleware{authenticate},
			handler:     func(http.ResponseWriter, *http.Request) {},
			wantInserts: []string{model.AuditStatusAttempted},
			wantFinal:   &auditEvent.Completion{Status: model.AuditStatusSuccess, StatusCode: http.StatusOK},
			wantResId:   "spx-1",
		},
		{
			name:        "denied after authentication is a failed event",
			chain:       []router.Middleware{authenticate, reject(http.StatusForbidden)},
			handler:     func(http.ResponseWriter, *http.Request) {},
			wantInserts: []string{model.AuditStatusAttempted},
			wantFinal:   &auditEvent.Completion{Status: model.AuditStatusFailed, StatusCode: http.StatusForbidden},
			wantResId:   "spx-1",
		},
		{
			name:    "unauthenticated request leaves no event",
			chain:   []router.Middleware{reject(http.StatusUnauthorized)},
			handler: func(http.ResponseWriter, *http.Request) {},
		},
		{
			name:  "handler reports the resource and project",
			chain: []router.Middleware{authenticate},
			handler: func(w http.ResponseWriter, r *http.Request) {
				SetResource(r.Context(), "spx-new")
				SetProject(r.Context(), projectId)
				w.WriteHeader(http.StatusOK)
			},
			path:        "/" + orgId.String() + "/disk",
			wantInserts: []string{model.AuditStatusAttempted},
			wantFinal:   &auditEvent.Completion{Status: model.AuditStatusSuccess, StatusCode: http.StatusOK},
			wantResId:   "spx-new",
		},
		{
			name:        "panic is a failed event and keeps propagating",
			chain:       []router.Middleware{authenticate},
			handler:     func(http.ResponseWriter, *http.Request) { panic("boom") },
			wantPanic:   true,
			wantInserts: []string{model.AuditStatusAttempted},
			wantFinal:   &auditEvent.Completion{Status: model.AuditStatusFailed, StatusCode: http.StatusInternalServerError},
			wantResId:   "spx-1",
		},
		{
			name:        "failed attempted write falls back to one final insert",
			chain:       []router.Middleware{authenticate},
			handler:     func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) },
			insertErr:   errors.New("boom"),
			wantInserts: []string{model.AuditStatusSuccess},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeStore{insertErr: tt.insertErr}
			declaration := router.Audit{ResourceType: "disk", Action: router.ActionDelete, ResourceParam: "effectiveId"}

			root := chi.NewRouter()
			chain := append([]router.Middleware{Middleware(store)(declaration)}, tt.chain...)
			root.With(chain...).Post("/{orgaId}/{projectId}/disk/{effectiveId}", tt.handler)
			root.With(chain...).Post("/{orgaId}/disk", tt.handler)

			path := tt.path
			if path == "" {
				path = "/" + orgId.String() + "/" + projectId.String() + "/disk/spx-1"
			}
			r := httptest.NewRequest(http.MethodPost, path, nil)
			r.RemoteAddr = "10.0.0.1:1234"
			r.Header.Set("X-Forwarded-For", "1.2.3.4, 10.0.0.1")

			serve := func() { root.ServeHTTP(httptest.NewRecorder(), r) }
			if tt.wantPanic {
				assert.Panics(t, serve)
			} else {
				serve()
			}

			statuses := make([]string, 0, len(store.inserted))
			for _, event := range store.inserted {
				statuses = append(statuses, event.Status)
				assert.Equal(t, "disk.delete", event.EventType)
				assert.Equal(t, "disk", event.ResourceType)
				assert.Equal(t, &orgId, event.OrganizationId)
				assert.Equal(t, &userId, event.UserId)
				assert.Equal(t, "1.2.3.4", event.SourceIp)
				assert.Equal(t, "10.0.0.1", event.RemoteAddr)
				assert.False(t, event.StartedAt.IsZero())
			}
			assert.ElementsMatch(t, tt.wantInserts, statuses)

			if tt.wantFinal == nil {
				assert.Empty(t, store.finalized)
				return
			}
			if assert.Len(t, store.finalized, 1) {
				got := store.finalized[0]
				assert.Equal(t, tt.wantFinal.Status, got.Status)
				assert.Equal(t, tt.wantFinal.StatusCode, got.StatusCode)
				assert.Equal(t, &projectId, got.ProjectId)
				assert.False(t, got.CompletedAt.IsZero())
				if assert.NotNil(t, got.ResourceId) {
					assert.Equal(t, tt.wantResId, *got.ResourceId)
				}
			}
		})
	}
}

func TestRecordFunctionsWithoutRecord(t *testing.T) {
	tests := []struct {
		name string
		call func(ctx context.Context)
	}{
		{name: "Begin", call: func(ctx context.Context) { Begin(ctx, uuid.NewString(), "JwtBearer") }},
		{name: "SetResource", call: func(ctx context.Context) { SetResource(ctx, "spx-1") }},
		{name: "SetProject", call: func(ctx context.Context) { SetProject(ctx, uuid.New()) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() { tt.call(context.Background()) })
		})
	}
}
