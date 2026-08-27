package scope

import (
	"context"
	"path/filepath"
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

func TestReparseLimitationsInvalidatesCache(t *testing.T) {
	svc, st := newTestScope(t)
	ctx := context.Background()
	b, err := st.CreateBatch(ctx, "CACHE-1", "probe")
	if err != nil {
		t.Fatal(err)
	}
	seg, err := st.CreateSegment(ctx, &model.JudgmentSegment{
		BatchID: b.ID, SourceDoc: "doc", SeqNo: 1,
		Text: "本院判令限于A省行政区域内适用本规则。",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ParseLimitations(ctx, seg.ID); err != nil {
		t.Fatal(err)
	}
	first, err := st.ListLimitations(ctx, seg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 {
		t.Fatal("expected limitations after first parse")
	}
	if _, err := st.DB.ExecContext(ctx, `UPDATE judgment_segment SET text = ? WHERE id = ?`,
		"该规则自2020年1月1日起至2022年12月31日期间有效。", seg.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ParseLimitations(ctx, seg.ID); err != nil {
		t.Fatal(err)
	}
	second, err := st.ListLimitations(ctx, seg.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, lim := range second {
		if lim.LType == model.LimTerritorial {
			t.Fatalf("stale cache still has territorial limitation: %+v", second)
		}
	}
}
