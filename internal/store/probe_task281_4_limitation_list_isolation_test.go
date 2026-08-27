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

func TestLimitationListsDoNotShareBackingArray(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	b, err := st.CreateBatch(ctx, "SLICE-A", "probe")
	if err != nil {
		t.Fatal(err)
	}
	b2, err := st.CreateBatch(ctx, "SLICE-B", "probe")
	if err != nil {
		t.Fatal(err)
	}
	segA := mustSegment(t, st, b.ID, 1, "限于A省境内适用本规则。")
	segB := mustSegment(t, st, b2.ID, 1, "自2020年1月1日起至2022年12月31日期间有效。")
	if _, err := st.CreateLimitation(ctx, &model.LimitationClause{
		SegmentID: segA, BatchID: b.ID, LType: model.LimTerritorial, Text: "限于A省",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateLimitation(ctx, &model.LimitationClause{
		SegmentID: segA, BatchID: b.ID, LType: model.LimTemporal, Text: "期间有效",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateLimitation(ctx, &model.LimitationClause{
		SegmentID: segB, BatchID: b2.ID, LType: model.LimTemporal, Text: "2020-2022",
	}); err != nil {
		t.Fatal(err)
	}
	first, err := st.ListLimitations(ctx, segA)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) < 2 {
		t.Fatalf("want 2 limitations in A, got %d", len(first))
	}
	aid := first[0].SegmentID
	second, err := st.ListLimitations(ctx, segB)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) == 0 {
		t.Fatal("segment B should have limitations")
	}
	if first[0].SegmentID != aid || first[0].SegmentID != segA {
		t.Fatalf("list A overwritten: first segment=%d want=%d", first[0].SegmentID, segA)
	}
}
