package store

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"task281-citereview/internal/model"
)

// freshTestStore 打开一个临时文件级数据库并完成迁移。
// 不使用 :memory: —— 在连接池下每个连接会得到独立的内存库，无法反映真实并发。
// 临时文件忠实复现生产环境的 WAL + 连接池路径。
func freshTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// runConcurrent 在 n 个 goroutine 中同时执行 fn，全部经 start 屏障同时释放，
// 最大化并发以暴露"先检测后写入"竞态。
func runConcurrent(n int, fn func()) {
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			<-start
			fn()
		}()
	}
	close(start)
	wg.Wait()
}

// seedBatchWithSegments 建一个批次并返回 N 个已就绪的判决段。
func seedBatchWithSegments(t *testing.T, s *Store, n int) (int64, []*model.JudgmentSegment) {
	t.Helper()
	ctx := context.Background()
	batch, err := s.CreateBatch(ctx, "CONC-1", "并发建边")
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	var segs []*model.JudgmentSegment
	for i := 0; i < n; i++ {
		seg, err := s.CreateSegment(ctx, &model.JudgmentSegment{
			BatchID:     batch.ID,
			SourceDoc:   "判例集",
			SeqNo:       i + 1,
			Text:        "段",
			SummaryHash: "h",
		})
		if err != nil {
			t.Fatalf("create segment %d: %v", i, err)
		}
		segs = append(segs, seg)
	}
	return batch.ID, segs
}

// TestConcurrentDuplicateEdges 模拟二十位同事同时创建同一对引证关系：
// 恰有一条成功，其余全部以 ErrDuplicateEdge 失败，最终库里只存一条边。
func TestConcurrentDuplicateEdges(t *testing.T) {
	s := freshTestStore(t)
	batchID, segs := seedBatchWithSegments(t, s, 2)
	citing, cited := segs[0], segs[1]

	const n = 20
	var mu sync.Mutex
	var succ, dup, other int
	runConcurrent(n, func() {
		_, err := s.CreateEdge(context.Background(), &model.CitationEdge{
			BatchID:         batchID,
			CitingSegmentID: citing.ID,
			CitedSegmentID:  cited.ID,
			Relation:        "遵循先例",
		})
		mu.Lock()
		defer mu.Unlock()
		switch {
		case err == nil:
			succ++
		case errors.Is(err, model.ErrDuplicateEdge):
			dup++
		default:
			other++
			t.Logf("unexpected err: %v", err)
		}
	})

	if succ != 1 {
		t.Fatalf("expected exactly 1 success, got %d (dup=%d other=%d)", succ, dup, other)
	}
	if other != 0 {
		t.Fatalf("expected no other errors, got %d", other)
	}
	edges, err := s.ListEdges(context.Background(), batchID)
	if err != nil {
		t.Fatalf("list edges: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 persisted edge, got %d", len(edges))
	}
}

// TestConcurrentCycleRejection 先建一条 A→B，再让二十人并发建反向边 B→A：
// 既有 A→B 使 cited(A) 已可达 citing(B)，全部必须以 ErrCitationCycle 失败，
// 且最终不新增任何反向边（图不成环）。
func TestConcurrentCycleRejection(t *testing.T) {
	s := freshTestStore(t)
	batchID, segs := seedBatchWithSegments(t, s, 2)
	a, b := segs[0], segs[1]

	// 先建立 a→b
	if _, err := s.CreateEdge(context.Background(), &model.CitationEdge{
		BatchID: batchID, CitingSegmentID: a.ID, CitedSegmentID: b.ID,
	}); err != nil {
		t.Fatalf("seed forward edge: %v", err)
	}

	const n = 20
	var mu sync.Mutex
	var cycle, succ, other int
	runConcurrent(n, func() {
		_, err := s.CreateEdge(context.Background(), &model.CitationEdge{
			BatchID: batchID, CitingSegmentID: b.ID, CitedSegmentID: a.ID,
		})
		mu.Lock()
		defer mu.Unlock()
		switch {
		case err == nil:
			succ++
		case errors.Is(err, model.ErrCitationCycle):
			cycle++
		default:
			other++
			t.Logf("unexpected err: %v", err)
		}
	})

	if succ != 0 {
		t.Fatalf("expected no successful cycle-forming edges, got %d", succ)
	}
	if other != 0 {
		t.Fatalf("expected no other errors, got %d", other)
	}
	if cycle != n {
		t.Fatalf("expected %d cycle rejections, got %d", n, cycle)
	}
	edges, err := s.ListEdges(context.Background(), batchID)
	if err != nil {
		t.Fatalf("list edges: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected exactly 1 edge (no ring), got %d", len(edges))
	}
}
