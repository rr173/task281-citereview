package scope

import (
	"context"
	"path/filepath"
	"testing"

	"task281-citereview/internal/material"
	"task281-citereview/internal/store"
)

func TestParseLimitationsTerritorial(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "scope.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	b, err := st.CreateBatch(ctx, "S1", "scope")
	if err != nil {
		t.Fatal(err)
	}
	seg, err := material.New(st).ImportSegment(ctx, b.ID, "doc", 1, "限于A省行政区域内适用本规则。")
	if err != nil {
		t.Fatal(err)
	}
	lims, err := New(st).ParseLimitations(ctx, seg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(lims) == 0 {
		t.Fatal("expected territorial limitation")
	}
}
