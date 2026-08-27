package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"task281-citereview/internal/model"
	"task281-citereview/internal/service"
	"task281-citereview/internal/store"
)

// newTestServer 构造一个绑定临时 SQLite 的完整编排服务与 HTTP mux，
// 供引证关系 HTTP 层断言使用。
func newTestServer(t *testing.T) (*httptest.Server, *service.Service) {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "citereview_test.db")
	db, err := store.Open(tmp)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	svc := service.New(db)
	mux := NewMux(svc)
	return httptest.NewServer(mux), svc
}

// postJSON 向给定地址发起 POST，写回 JSON 请求体并返回响应。
func postJSON(t *testing.T, url string, body interface{}) *http.Response {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

// TestCreateCitationSelfCitationReturns409 断言自引（citing == cited）
// 经 POST /api/batches/{id}/citations 必须以 409 Conflict 返回，而非 400。
func TestCreateCitationSelfCitationReturns409(t *testing.T) {
	srv, svc := newTestServer(t)
	defer srv.Close()
	ctx := context.Background()

	batch, err := svc.Store.CreateBatch(ctx, "T-409", "自引冲突回归")
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	segs, err := svc.Material.ImportMaterial(ctx, batch.ID, "判例集", []string{
		"本院判令限于A省行政区域内适用本规则。",
		"本院认为前述判例可予适用，原告请求成立。",
	})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	// 自引：citing == cited
	resp := postJSON(t, srv.URL+"/api/batches/"+itoaSafe(batch.ID)+"/citations", map[string]interface{}{
		"citing_segment_id": segs[0].ID,
		"cited_segment_id":  segs[0].ID,
		"relation":          "遵循先例",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("self-citation: got status %d, want %d (409 Conflict)", resp.StatusCode, http.StatusConflict)
	}
}

// TestCreateCitationCycleAndDuplicateReturn409 断言环检测与重复对
// 同样以 409 返回，与哨兵错误的领域约定一致。
func TestCreateCitationCycleAndDuplicateReturn409(t *testing.T) {
	srv, svc := newTestServer(t)
	defer srv.Close()
	ctx := context.Background()

	batch, err := svc.Store.CreateBatch(ctx, "T-409C", "环与重复冲突回归")
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	segs, err := svc.Material.ImportMaterial(ctx, batch.ID, "判例集", []string{
		"原案判决段文本。",
		"后案判决段文本。",
	})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	cited, citing := segs[0], segs[1]

	// 正向边：后案 → 原案
	resp := postJSON(t, srv.URL+"/api/batches/"+itoaSafe(batch.ID)+"/citations", map[string]interface{}{
		"citing_segment_id": citing.ID,
		"cited_segment_id":  cited.ID,
		"relation":          "遵循先例",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create forward edge: got status %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	resp.Body.Close()

	// 重复对：同 (citing, cited) 应 409
	resp = postJSON(t, srv.URL+"/api/batches/"+itoaSafe(batch.ID)+"/citations", map[string]interface{}{
		"citing_segment_id": citing.ID,
		"cited_segment_id":  cited.ID,
		"relation":          "再次引用",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate edge: got status %d, want %d (409)", resp.StatusCode, http.StatusConflict)
	}

	// 反向边：原案 → 后案 会成环，应 409
	resp = postJSON(t, srv.URL+"/api/batches/"+itoaSafe(batch.ID)+"/citations", map[string]interface{}{
		"citing_segment_id": cited.ID,
		"cited_segment_id":  citing.ID,
		"relation":          "反向引用",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("cycle edge: got status %d, want %d (409)", resp.StatusCode, http.StatusConflict)
	}
}

// TestStatusForErrorSentinels 单元覆盖 statusForError 对哨兵错误的解包映射，
// 包括经 fmt.Errorf("%w") 包裹后仍应识别为 409/404，杜绝本 bug 回归。
func TestStatusForErrorSentinels(t *testing.T) {
	conflicts := []error{
		model.ErrSelfCitation, model.ErrCitationCycle, model.ErrDuplicateEdge,
		model.ErrFrozenImmutable, model.ErrVersionMismatch, model.ErrElementMissing,
		model.ErrLimitationConflict, model.ErrDuplicateLimitation, model.ErrInvalidTransition,
		model.ErrDuplicateSummary,
	}
	for _, e := range conflicts {
		if got := statusForError(e); got != http.StatusConflict {
			t.Errorf("statusForError(%v) = %d, want %d", e, got, http.StatusConflict)
		}
		// 经 fmt.Errorf("%w") 包裹后仍应识别为 409
		wrapped := fmt.Errorf("create citation: %w", e)
		if got := statusForError(wrapped); got != http.StatusConflict {
			t.Errorf("statusForError(wrapped %v) = %d, want %d", e, got, http.StatusConflict)
		}
	}
	notFound := []error{
		model.ErrBatchNotFound, model.ErrSegmentNotFound, model.ErrEdgeNotFound,
		model.ErrDecisionNotFound, model.ErrVersionNotFound,
	}
	for _, e := range notFound {
		if got := statusForError(e); got != http.StatusNotFound {
			t.Errorf("statusForError(%v) = %d, want %d", e, got, http.StatusNotFound)
		}
		wrapped := fmt.Errorf("load: %w", e)
		if got := statusForError(wrapped); got != http.StatusNotFound {
			t.Errorf("statusForError(wrapped %v) = %d, want %d", e, got, http.StatusNotFound)
		}
	}
	// 未知错误仍为 400
	if got := statusForError(errors.New("some unknown error")); got != http.StatusBadRequest {
		t.Errorf("statusForError(unknown) = %d, want %d", got, http.StatusBadRequest)
	}
}
