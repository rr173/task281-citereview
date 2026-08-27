package scope

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"task281-citereview/internal/model"
	"task281-citereview/internal/store"
)

func newTestScope(t *testing.T) (*Service, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "probe.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return New(st), st
}

func mustBatchWithSegment(t *testing.T, st *store.Store, code, text string) (int64, int64) {
	t.Helper()
	ctx := context.Background()
	b, err := st.CreateBatch(ctx, code, "probe")
	if err != nil {
		t.Fatal(err)
	}
	seg, err := st.CreateSegment(ctx, &model.JudgmentSegment{
		BatchID: b.ID, SourceDoc: "doc", SeqNo: 1, Text: text,
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.ID, seg.ID
}

func TestCheckScopeRespectsCancellation(t *testing.T) {
	svc, st := newTestScope(t)
	ctx := context.Background()
	b, err := st.CreateBatch(ctx, "CTX-1", "probe")
	if err != nil {
		t.Fatal(err)
	}
	cited, err := st.CreateSegment(ctx, &model.JudgmentSegment{
		BatchID: b.ID, SourceDoc: "doc", SeqNo: 1,
		Text: "本院判令限于A省行政区域内适用本规则。",
	})
	if err != nil {
		t.Fatal(err)
	}
	citing, err := st.CreateSegment(ctx, &model.JudgmentSegment{
		BatchID: b.ID, SourceDoc: "doc", SeqNo: 2,
		Text: "本院认为前述判例可予适用。",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ParseLimitations(ctx, cited.ID); err != nil {
		t.Fatal(err)
	}
	edge, err := st.CreateEdge(ctx, &model.CitationEdge{
		BatchID: b.ID, CitingSegmentID: citing.ID, CitedSegmentID: cited.ID, Relation: "cite",
	})
	if err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := svc.CheckScope(runCtx, edge.ID)
		done <- err
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()
	select {
	case <-time.After(200 * time.Millisecond):
		t.Fatal("CheckScope ignored context cancellation")
	case err := <-done:
		if err == nil {
			t.Fatal("expected cancellation error")
		}
	}
}
