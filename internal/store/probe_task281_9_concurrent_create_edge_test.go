package store

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
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

func TestConcurrentCreateEdgeRejected(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	b, err := st.CreateBatch(ctx, "EDGE-RACE", "probe")
	if err != nil {
		t.Fatal(err)
	}
	s1 := mustSegment(t, st, b.ID, 1, "seg1")
	s2 := mustSegment(t, st, b.ID, 2, "seg2")
	s3 := mustSegment(t, st, b.ID, 3, "seg3")
	if _, err := st.CreateEdge(ctx, &model.CitationEdge{
		BatchID: b.ID, CitingSegmentID: s1, CitedSegmentID: s2, Relation: "base",
	}); err != nil {
		t.Fatal(err)
	}
	const workers = 20
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	okCh := make(chan struct{}, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := st.CreateEdge(ctx, &model.CitationEdge{
				BatchID: b.ID, CitingSegmentID: s2, CitedSegmentID: s3, Relation: fmt.Sprintf("w%d", i),
			})
			if err == nil {
				okCh <- struct{}{}
				return
			}
			if err != model.ErrDuplicateEdge {
				errCh <- fmt.Errorf("worker %d: %w", i, err)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	close(okCh)
	for err := range errCh {
		t.Fatal(err)
	}
	success := 0
	for range okCh {
		success++
	}
	if success != 1 {
		t.Fatalf("want exactly 1 successful create, got %d", success)
	}
	edges, err := st.ListEdges(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 2 {
		t.Fatalf("want 2 edges total, got %d", len(edges))
	}
}
