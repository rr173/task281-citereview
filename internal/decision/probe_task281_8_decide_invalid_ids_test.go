package decision

import (
	"context"
	"path/filepath"
	"testing"

	"task281-citereview/internal/model"
	"task281-citereview/internal/store"
)

func newTestDecision(t *testing.T) (*Service, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "probe.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return New(st), st
}

func TestDecideInvalidIDsNoPanic(t *testing.T) {
	svc, _ := newTestDecision(t)
	ctx := context.Background()
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("Decide panicked: %v", rec)
		}
	}()
	_, err := svc.Decide(ctx, 999, model.CiteApplicable, "reason", 999)
	if err == nil {
		t.Fatal("invalid ids must return an error")
	}
}
