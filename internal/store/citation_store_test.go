package store

import (
	"context"
	"path/filepath"
	"testing"

	"task281-citereview/internal/model"
)

func TestCreateEdgeRejectsSelfCitation(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "edge.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	b, err := st.CreateBatch(ctx, "E1", "edge test")
	if err != nil {
		t.Fatal(err)
	}
	seg, err := st.CreateSegment(ctx, &model.JudgmentSegment{BatchID: b.ID, SourceDoc: "d", SeqNo: 1, Text: "t", SummaryHash: "h1"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.CreateEdge(ctx, &model.CitationEdge{
		BatchID: b.ID, CitingSegmentID: seg.ID, CitedSegmentID: seg.ID,
	})
	if err != model.ErrSelfCitation {
		t.Fatalf("want ErrSelfCitation, got %v", err)
	}
}
