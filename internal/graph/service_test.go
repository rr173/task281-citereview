package graph

import (
	"context"
	"path/filepath"
	"testing"

	"task281-citereview/internal/model"
	"task281-citereview/internal/store"
)

// newTestStore 打开一个临时 SQLite，返回就绪的 store 与清理函数。
func newTestStore(t *testing.T) (*store.Store, func()) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "graph_test.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return st, func() { _ = st.Close() }
}

// importTwoSegments 导入两个判决段，返回 (cited, citing)。
func importTwoSegments(t *testing.T, st *store.Store, batchID int64) (cited, citing *model.JudgmentSegment) {
	t.Helper()
	ctx := context.Background()
	segs, err := st.ListSegments(ctx, batchID)
	if err != nil {
		t.Fatalf("list segments: %v", err)
	}
	if len(segs) < 2 {
		t.Fatalf("need >=2 segments, got %d", len(segs))
	}
	return segs[0], segs[1]
}

// TestFreezeLocksEdgeSet 验证冻结版本的图视图锁定冻结时刻的引证边集合：
// 在草稿创建之后、冻结之前新增的引证边，必须被冻结快照捕获并出现在复核视图中；
// 冻结之后再新增的引证边不应出现在冻结版本的复核视图中，而实时表仍包含新增边。
// 该测试同时覆盖两条不变量：冻结时刻重建快照（捕获冻结前新增边）与冻结后视图锁定。
func TestFreezeLocksEdgeSet(t *testing.T) {
	st, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()
	svc := New(st)

	batch, err := st.CreateBatch(ctx, "B-1", "冻结边集合测试")
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	cited, err := st.CreateSegment(ctx, &model.JudgmentSegment{
		BatchID: batch.ID, SourceDoc: "doc", SeqNo: 1, Text: "原案判段", SummaryHash: "h1"})
	if err != nil {
		t.Fatalf("create cited: %v", err)
	}
	citing, err := st.CreateSegment(ctx, &model.JudgmentSegment{
		BatchID: batch.ID, SourceDoc: "doc", SeqNo: 2, Text: "后案判段", SummaryHash: "h2"})
	if err != nil {
		t.Fatalf("create citing: %v", err)
	}
	third, err := st.CreateSegment(ctx, &model.JudgmentSegment{
		BatchID: batch.ID, SourceDoc: "doc", SeqNo: 3, Text: "第三判段", SummaryHash: "h3"})
	if err != nil {
		t.Fatalf("create third: %v", err)
	}
	fourth, err := st.CreateSegment(ctx, &model.JudgmentSegment{
		BatchID: batch.ID, SourceDoc: "doc", SeqNo: 4, Text: "第四判段", SummaryHash: "h4"})
	if err != nil {
		t.Fatalf("create fourth: %v", err)
	}

	// 先建立 1 条引证边 e1（citing -> cited），随后创建草稿（快照此时仅含 e1）
	e1, err := st.CreateEdge(ctx, &model.CitationEdge{
		BatchID: batch.ID, CitingSegmentID: citing.ID, CitedSegmentID: cited.ID, Relation: "遵循先例",
	})
	if err != nil {
		t.Fatalf("create e1: %v", err)
	}
	ver, err := svc.CreateDraft(ctx, batch.ID)
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	// 草稿之后、冻结之前新增第 2 条边 e2（cited -> third）。
	// 冻结时刻必须重建快照以捕获 e2，否则复核视图会退回到草稿快照（仅 e1）。
	e2, err := st.CreateEdge(ctx, &model.CitationEdge{
		BatchID: batch.ID, CitingSegmentID: cited.ID, CitedSegmentID: third.ID, Relation: "扩展",
	})
	if err != nil {
		t.Fatalf("create e2 before freeze: %v", err)
	}
	if err := svc.Freeze(ctx, ver.ID); err != nil {
		t.Fatalf("freeze: %v", err)
	}

	// 冻结后再新增第 3 条边 e3（third -> fourth，与既有图不构成环）。
	if _, err := st.CreateEdge(ctx, &model.CitationEdge{
		BatchID: batch.ID, CitingSegmentID: third.ID, CitedSegmentID: fourth.ID, Relation: "补充",
	}); err != nil {
		t.Fatalf("create e3 after freeze: %v", err)
	}

	// 复核视图应恰好锁定冻结时刻的两条边（e1、e2），不含冻结后的 e3。
	view, err := svc.BuildView(ctx, batch.ID)
	if err != nil {
		t.Fatalf("build view: %v", err)
	}
	if view.Version == nil || view.Version.Status != model.GVFrozen {
		t.Fatalf("expected latest frozen version, got %+v", view.Version)
	}
	if len(view.Edges) != 2 {
		t.Fatalf("frozen view edges = %d, want 2 (locked at freeze time)", len(view.Edges))
	}
	got := map[int64]bool{view.Edges[0].ID: true, view.Edges[1].ID: true}
	if !got[e1.ID] || !got[e2.ID] {
		t.Fatalf("frozen view edges = %v, want {%d,%d}", got, e1.ID, e2.ID)
	}

	// 实时表应仍包含三条边，确认"冻结不阻止新增"，问题仅在于视图必须锁定。
	live, err := st.ListEdges(ctx, batch.ID)
	if err != nil {
		t.Fatalf("list live edges: %v", err)
	}
	if len(live) != 3 {
		t.Fatalf("live edges = %d, want 3", len(live))
	}
}

// TestDraftViewReflectsLiveEdges 验证未冻结（草稿）版本的图视图仍反映实时材料，
// 确保修复未误伤草稿态的实时性。
func TestDraftViewReflectsLiveEdges(t *testing.T) {
	st, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()
	svc := New(st)

	batch, err := st.CreateBatch(ctx, "B-2", "草稿实时视图测试")
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	cited, err := st.CreateSegment(ctx, &model.JudgmentSegment{
		BatchID: batch.ID, SourceDoc: "doc", SeqNo: 1, Text: "原案", SummaryHash: "h1"})
	if err != nil {
		t.Fatalf("create cited: %v", err)
	}
	citing, err := st.CreateSegment(ctx, &model.JudgmentSegment{
		BatchID: batch.ID, SourceDoc: "doc", SeqNo: 2, Text: "后案", SummaryHash: "h2"})
	if err != nil {
		t.Fatalf("create citing: %v", err)
	}
	if _, err := st.CreateEdge(ctx, &model.CitationEdge{
		BatchID: batch.ID, CitingSegmentID: citing.ID, CitedSegmentID: cited.ID, Relation: "遵循先例",
	}); err != nil {
		t.Fatalf("create edge: %v", err)
	}

	ver, err := svc.CreateDraft(ctx, batch.ID)
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	// 未冻结，草稿态
	if ver.Status != model.GVDraft {
		t.Fatalf("expected draft, got %s", ver.Status)
	}

	// 草稿后再新增一条边，视图应反映实时（含两条）
	third, err := st.CreateSegment(ctx, &model.JudgmentSegment{
		BatchID: batch.ID, SourceDoc: "doc", SeqNo: 3, Text: "第三", SummaryHash: "h3"})
	if err != nil {
		t.Fatalf("create third: %v", err)
	}
	if _, err := st.CreateEdge(ctx, &model.CitationEdge{
		BatchID: batch.ID, CitingSegmentID: cited.ID, CitedSegmentID: third.ID, Relation: "扩展",
	}); err != nil {
		t.Fatalf("create edge2: %v", err)
	}

	view, err := svc.BuildView(ctx, batch.ID)
	if err != nil {
		t.Fatalf("build view: %v", err)
	}
	if len(view.Edges) != 2 {
		t.Fatalf("draft view edges = %d, want 2 (live)", len(view.Edges))
	}
}

// TestFreezePersistsAcrossReopen 验证冻结版本锁定的边集合在重启后仍保持。
func TestFreezePersistsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "reopen_test.db")

	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	svc := New(st)

	batch, err := st.CreateBatch(ctx, "B-3", "重启锁定测试")
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	cited, err := st.CreateSegment(ctx, &model.JudgmentSegment{
		BatchID: batch.ID, SourceDoc: "doc", SeqNo: 1, Text: "原案", SummaryHash: "h1"})
	if err != nil {
		t.Fatalf("create cited: %v", err)
	}
	citing, err := st.CreateSegment(ctx, &model.JudgmentSegment{
		BatchID: batch.ID, SourceDoc: "doc", SeqNo: 2, Text: "后案", SummaryHash: "h2"})
	if err != nil {
		t.Fatalf("create citing: %v", err)
	}
	if _, err := st.CreateEdge(ctx, &model.CitationEdge{
		BatchID: batch.ID, CitingSegmentID: citing.ID, CitedSegmentID: cited.ID, Relation: "遵循先例",
	}); err != nil {
		t.Fatalf("create edge: %v", err)
	}
	ver, err := svc.CreateDraft(ctx, batch.ID)
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if err := svc.Freeze(ctx, ver.ID); err != nil {
		t.Fatalf("freeze: %v", err)
	}

	// 重启
	if err := st.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	st2, err := store.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer st2.Close()
	svc2 := New(st2)

	// 重启后新增边
	third, err := st2.CreateSegment(ctx, &model.JudgmentSegment{
		BatchID: batch.ID, SourceDoc: "doc", SeqNo: 3, Text: "第三", SummaryHash: "h3"})
	if err != nil {
		t.Fatalf("create third: %v", err)
	}
	if _, err := st2.CreateEdge(ctx, &model.CitationEdge{
		BatchID: batch.ID, CitingSegmentID: cited.ID, CitedSegmentID: third.ID, Relation: "扩展",
	}); err != nil {
		t.Fatalf("create edge2: %v", err)
	}

	view, err := svc2.BuildView(ctx, batch.ID)
	if err != nil {
		t.Fatalf("build view: %v", err)
	}
	if view.Version == nil || view.Version.Status != model.GVFrozen {
		t.Fatalf("expected frozen version after reopen, got %+v", view.Version)
	}
	if len(view.Edges) != 1 {
		t.Fatalf("reopened frozen view edges = %d, want 1", len(view.Edges))
	}
}
