package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeResStore struct {
	resource *models.Resource
	backends []models.ResourceBackend
}

func (f *fakeResStore) ListByUserID(context.Context, string, string) ([]models.Resource, error) {
	return []models.Resource{*f.resource}, nil
}
func (f *fakeResStore) FindByID(context.Context, string, string) (*models.Resource, error) {
	cp := *f.resource
	return &cp, nil
}
func (f *fakeResStore) CreateWithBackendsTx(context.Context, *models.Resource, []models.ResourceBackend) error {
	return nil
}
func (f *fakeResStore) Update(_ context.Context, res *models.Resource) error {
	cp := *res
	f.resource = &cp
	return nil
}
func (f *fakeResStore) Delete(context.Context, string, string) error { return nil }
func (f *fakeResStore) GetResourceWithBackendsByID(context.Context, string, string) (*models.ResourceWithBackends, error) {
	return &models.ResourceWithBackends{Resource: *f.resource, Backends: f.backends}, nil
}

type fakeResBackendStore struct {
	backends []models.ResourceBackend
}

func (f *fakeResBackendStore) ListByResourceID(context.Context, string) ([]models.ResourceBackend, error) {
	return append([]models.ResourceBackend(nil), f.backends...), nil
}
func (f *fakeResBackendStore) DeleteByResourceID(context.Context, string) error {
	f.backends = nil
	return nil
}
func (f *fakeResBackendStore) Create(_ context.Context, b *models.ResourceBackend) error {
	f.backends = append(f.backends, *b)
	return nil
}

func TestResourceHandler_UpdatePreservesSchemaAndSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	existingSecret := "enc_secret_tok"
	endpoint := "https://93.184.216.34/v1/search"
	existingSchema := json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`)

	rStore := &fakeResStore{
		resource: &models.Resource{
			ID:               "res-1",
			UserID:           "u1",
			Name:             "docs_search",
			DisplayName:      "Docs",
			ParametersSchema: existingSchema,
			Enabled:          true,
		},
	}
	bStore := &fakeResBackendStore{
		backends: []models.ResourceBackend{{
			ID:                 "rb-1",
			ResourceID:         "res-1",
			Name:               "primary",
			BackendType:        "http",
			EndpointURL:        &endpoint,
			AuthTokenEncrypted: &existingSecret,
			AuthHeaderName:     "X-API-Key",
			AuthHeaderPrefix:   "",
		}},
	}
	h := &ResourceHandler{resources: rStore, backends: bStore}

	// Partial update omitting parametersSchema and backends must NOT wipe schema or delete backends
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/resources/res-1", bytes.NewBufferString(`{"displayName":"Docs V2"}`))
	c.Params = gin.Params{{Key: "id", Value: "res-1"}}
	c.Set("userId", "u1")
	h.Update(c)
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, string(existingSchema), string(rStore.resource.ParametersSchema))
	assert.Equal(t, "Docs V2", rStore.resource.DisplayName)
	require.Len(t, bStore.backends, 1)

	// Replacing backends without a new authToken preserves prior AuthTokenEncrypted
	updateBackendsPayload := `{"backends":[{"id":"rb-1","name":"primary","backendType":"http","endpointUrl":"https://93.184.216.34/v2/search","authToken":""}]}`
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodPut, "/api/resources/res-1", bytes.NewBufferString(updateBackendsPayload))
	c2.Params = gin.Params{{Key: "id", Value: "res-1"}}
	c2.Set("userId", "u1")
	h.Update(c2)
	require.Equal(t, http.StatusOK, w2.Code)
	require.Len(t, bStore.backends, 1)
	require.NotNil(t, bStore.backends[0].AuthTokenEncrypted)
	assert.Equal(t, existingSecret, *bStore.backends[0].AuthTokenEncrypted)
	assert.Equal(t, "X-API-Key", bStore.backends[0].AuthHeaderName)
}
