package scope

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

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

func TestConcurrentParseLimitationsSameSegment(t *testing.T) {
	svc, st := newTestScope(t)
	_, segID := mustBatchWithSegment(t, st, "LIM-RACE",
		"本院判令限于A省行政区域内适用本规则。该规则自2020年1月1日起至2022年12月31日期间有效。")
	const workers = 20
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			lims, err := svc.ParseLimitations(context.Background(), segID)
			if err != nil {
				errCh <- fmt.Errorf("parse %d: %w", i, err)
				return
			}
			if len(lims) == 0 {
				errCh <- fmt.Errorf("parse %d returned empty limitations", i)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
	want, err := svc.ParseLimitations(context.Background(), segID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := st.ListLimitations(context.Background(), segID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("stored=%d want=%d", len(got), len(want))
	}
}
