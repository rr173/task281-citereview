package graph

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"task281-citereview/internal/model"
	"task281-citereview/internal/store"
)

func newTestGraph(t *testing.T) (*Service, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "probe.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return New(st), st
}

func mustBatchSegments(t *testing.T, st *store.Store, n int) (int64, []int64) {
	t.Helper()
	ctx := context.Background()
	b, err := st.CreateBatch(ctx, "G-PROBE", "graph probe")
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		seg, err := st.CreateSegment(ctx, &model.JudgmentSegment{
			BatchID: b.ID, SourceDoc: "doc", SeqNo: i + 1,
			Text: fmt.Sprintf("seg-%d text", i+1),
		})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, seg.ID)
	}
	return b.ID, ids
}

func TestFrozenGraphViewIgnoresPostFreezeEdges(t *testing.T) {
	svc, st := newTestGraph(t)
	ctx := context.Background()
	batchID, segs := mustBatchSegments(t, st, 3)
	if _, err := st.CreateEdge(ctx, &model.CitationEdge{
		BatchID: batchID, CitingSegmentID: segs[0], CitedSegmentID: segs[1], Relation: "e1",
	}); err != nil {
		t.Fatal(err)
	}
	ver, err := svc.CreateDraft(ctx, batchID)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Freeze(ctx, ver.ID); err != nil {
		t.Fatal(err)
	}
	frozenCount := 1
	if _, err := st.CreateEdge(ctx, &model.CitationEdge{
		BatchID: batchID, CitingSegmentID: segs[1], CitedSegmentID: segs[2], Relation: "e2",
	}); err != nil {
		t.Fatal(err)
	}
	view, err := svc.BuildView(ctx, batchID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Edges) != frozenCount {
		t.Fatalf("frozen view edges=%d want=%d", len(view.Edges), frozenCount)
	}
}
