package scope

import (
	"context"
	"testing"

	"task281-citereview/internal/model"
	"task281-citereview/internal/store"
)

// newStoreWithSegment 打开临时库，建一个批次与一条含限制语的判决段，返回 store、batchID、segmentID。
func newStoreWithSegment(t *testing.T, text string) (*store.Store, int64, int64) {
	t.Helper()
	db, err := store.Open(t.TempDir() + "/scope_test.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	batch, err := db.CreateBatch(ctx, "B-1", "t")
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	seg, err := db.CreateSegment(ctx, &model.JudgmentSegment{
		BatchID: batch.ID, SeqNo: 1, Text: text,
	})
	if err != nil {
		t.Fatalf("create segment: %v", err)
	}
	return db, batch.ID, seg.ID
}

// TestReparseLimitationsCacheConsistency 复现并锁定修复：
// 先解析限制语并经 ListLimitations 命中缓存，随后修改判决段正文并重新解析，
// 再读取列表必须返回与数据库（重新解析结果）一致的新限制语，而非陈旧缓存。
func TestReparseLimitationsCacheConsistency(t *testing.T) {
	ctx := context.Background()

	// 原案正文：含地域限定
	db, _, segID := newStoreWithSegment(t, "本院判令限于A省行政区域内适用本规则。被告应履行给付义务。")
	svc := New(db)

	first, err := svc.ParseLimitations(ctx, segID)
	if err != nil {
		t.Fatalf("parse first: %v", err)
	}
	if len(first) == 0 {
		t.Fatalf("expected limitations from first parse, got 0")
	}
	firstText := first[0].Text

	// 触发缓存填充：经 ListLimitations 命中缓存（不回源数据库）
	if cached, err := db.ListLimitations(ctx, segID); err != nil || len(cached) == 0 {
		t.Fatalf("warm cache: cached=%v err=%v", cached, err)
	}

	// 修改判决段正文：去除地域限定、改为时间限定
	if _, err := db.DB.ExecContext(ctx,
		`UPDATE judgment_segment SET text = ? WHERE id = ?`,
		"本规则自2024年起至2025年期间暂行适用。被告应履行给付义务。", segID); err != nil {
		t.Fatalf("update segment text: %v", err)
	}

	// 重新解析
	second, err := svc.ParseLimitations(ctx, segID)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if len(second) == 0 {
		t.Fatalf("expected limitations from reparse, got 0")
	}
	if second[0].Text == firstText {
		t.Fatalf("reparse returned the old limitation text %q, expected new content", firstText)
	}

	// 关键断言：列表（走缓存路径）必须与数据库/重新解析结果一致
	listed, err := db.ListLimitations(ctx, segID)
	if err != nil {
		t.Fatalf("list after reparse: %v", err)
	}
	if len(listed) != len(second) {
		t.Fatalf("cache/db mismatch: list=%d reparse=%d", len(listed), len(second))
	}
	for i := range second {
		if listed[i].ID != second[i].ID || listed[i].Text != second[i].Text ||
			listed[i].LType != second[i].LType {
			t.Fatalf("stale cache: listed[%d]=%+v != reparse[%d]=%+v", i, listed[i], i, second[i])
		}
	}
}
