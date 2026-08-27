package store

import (
	"context"
	"path/filepath"
	"testing"

	"task281-citereview/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "probe.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func mustSegment(t *testing.T, st *Store, batchID int64, seq int, text string) int64 {
	t.Helper()
	ctx := context.Background()
	seg, err := st.CreateSegment(ctx, &model.JudgmentSegment{
		BatchID: batchID, SourceDoc: "doc", SeqNo: seq, Text: text,
	})
	if err != nil {
		t.Fatal(err)
	}
	return seg.ID
}

func TestReachableStaleVisitedAllowsCycle(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	b, err := st.CreateBatch(ctx, "CYCLE-1", "probe")
	if err != nil {
		t.Fatal(err)
	}
	s1 := mustSegment(t, st, b.ID, 1, "seg1")
	s2 := mustSegment(t, st, b.ID, 2, "seg2")
	s3 := mustSegment(t, st, b.ID, 3, "seg3")
	if _, err := st.CreateEdge(ctx, &model.CitationEdge{
		BatchID: b.ID, CitingSegmentID: s1, CitedSegmentID: s2, Relation: "a",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateEdge(ctx, &model.CitationEdge{
		BatchID: b.ID, CitingSegmentID: s2, CitedSegmentID: s3, Relation: "b",
	}); err != nil {
		t.Fatal(err)
	}
	reachable, err := st.Reachable(ctx, b.ID, s1, s3)
	if err != nil {
		t.Fatal(err)
	}
	if !reachable {
		t.Fatal("sanity: s1 should reach s3")
	}
	_, err = st.CreateEdge(ctx, &model.CitationEdge{
		BatchID: b.ID, CitingSegmentID: s3, CitedSegmentID: s1, Relation: "cycle",
	})
	if err != model.ErrCitationCycle {
		t.Fatalf("closing cycle must fail with ErrCitationCycle, got %v", err)
	}
}
