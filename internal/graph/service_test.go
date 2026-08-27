package graph

import (
	"context"
	"path/filepath"
	"testing"

	"task281-citereview/internal/material"
	"task281-citereview/internal/store"
)

func TestCreateDraftSnapshot(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	b, err := st.CreateBatch(ctx, "G1", "graph")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := material.New(st).ImportSegment(ctx, b.ID, "doc", 1, "测试段落。"); err != nil {
		t.Fatal(err)
	}
	ver, err := New(st).CreateDraft(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ver.MaterialSnapshot == "" {
		t.Fatal("expected non-empty snapshot")
	}
}
