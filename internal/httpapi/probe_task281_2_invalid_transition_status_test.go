package httpapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"task281-citereview/internal/model"
	"task281-citereview/internal/service"
	"task281-citereview/internal/store"
)

func newProbeServer(t *testing.T) http.Handler {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "probe-http.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return NewMux(service.New(st))
}

func TestInvalidBatchTransitionMapsConflict(t *testing.T) {
	h := newProbeServer(t)
	body := strings.NewReader(`{"code":"TR-1","title":"probe"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/batches", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create=%d body=%s", rec.Code, rec.Body.String())
	}
	body = strings.NewReader(`{"status":"` + string(model.BatchPublished) + `"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/batches/1/status", body)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("invalid transition status=%d body=%s", rec.Code, rec.Body.String())
	}
}
