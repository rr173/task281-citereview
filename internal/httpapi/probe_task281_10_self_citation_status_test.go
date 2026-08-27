package httpapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

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

func TestSelfCitationMapsConflict(t *testing.T) {
	h := newProbeServer(t)
	body := strings.NewReader(`{"code":"SC-1","title":"probe"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/batches", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create batch=%d body=%s", rec.Code, rec.Body.String())
	}
	body = strings.NewReader(`{"source_doc":"doc","segments":["seg text"]}`)
	req = httptest.NewRequest(http.MethodPost, "/api/batches/1/import", body)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("import=%d body=%s", rec.Code, rec.Body.String())
	}
	body = strings.NewReader(`{"citing_segment_id":1,"cited_segment_id":1,"relation":"self"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/batches/1/citations", body)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("self citation status=%d body=%s", rec.Code, rec.Body.String())
	}
}
