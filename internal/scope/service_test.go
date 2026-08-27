package scope

import (
	"context"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"task281-citereview/internal/model"
	"task281-citereview/internal/store"
)

// newStoreWithSegment 构造一个临时 SQLite 库、一个批次与一个含地域限定的判决段。
func newStoreWithSegment(t *testing.T) (*store.Store, int64) {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "scope_test.db")
	st, err := store.Open(tmp)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	ctx := context.Background()
	b, err := st.CreateBatch(ctx, "T", "并发解析测试")
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	now := time.Now().UnixNano()
	seg, err := st.CreateSegment(ctx, &model.JudgmentSegment{
		BatchID:     b.ID,
		SourceDoc:   "判例集",
		SeqNo:       1,
		Text:        "本院判令限于A省行政区域内适用本规则，被告应于期内履行给付义务。",
		SummaryHash: "h" + strconv.FormatInt(now, 10),
	})
	if err != nil {
		t.Fatalf("create segment: %v", err)
	}
	return st, seg.ID
}

// TestParseLimitationsConcurrentStable 模拟二十位同事对同一段已解析的判决反复解析。
// 先解析一次建立基线（1 条地域限定），随后并发重复解析期间持续读取：
// 修复后任一时刻读到的条数恒为预期值，绝不会出现"已清空尚未插入"的中间空列表。
func TestParseLimitationsConcurrentStable(t *testing.T) {
	st, segID := newStoreWithSegment(t)
	svc := New(st)
	ctx := context.Background()

	// 先解析一次建立基线
	if _, err := svc.ParseLimitations(ctx, segID); err != nil {
		t.Fatalf("seed parse: %v", err)
	}

	const colleagues = 20
	const rounds = 5
	want := 1 // 该段仅含一条地域限定

	stop := make(chan struct{})
	var parseErr error
	var parseWg sync.WaitGroup
	parseWg.Add(colleagues)
	for i := 0; i < colleagues; i++ {
		go func() {
			defer parseWg.Done()
			for r := 0; r < rounds; r++ {
				if _, err := svc.ParseLimitations(ctx, segID); err != nil && parseErr == nil {
					parseErr = err
				}
				select {
				case <-stop:
					return
				default:
				}
			}
		}()
	}

	// 并发解析期间持续读取条数，读到空列表即为回归
	deadline := time.Now().Add(2 * time.Second)
	var readWg sync.WaitGroup
	readWg.Add(1)
	go func() {
		defer readWg.Done()
		for time.Now().Before(deadline) {
			lims, err := st.ListLimitations(ctx, segID)
			if err != nil {
				t.Errorf("read: %v", err)
				return
			}
			if len(lims) == 0 {
				t.Errorf("读取到空列表：解析完成前后该段限制语不应为空")
				return
			}
			if len(lims) != want {
				t.Errorf("条数波动：got %d, want %d", len(lims), want)
				return
			}
			select {
			case <-stop:
				return
			default:
			}
		}
	}()

	parseWg.Wait()
	close(stop)
	readWg.Wait()
	if parseErr != nil {
		t.Fatalf("parse: %v", parseErr)
	}

	// 解析全部完成后条数必须稳定一致
	lims, err := st.ListLimitations(ctx, segID)
	if err != nil {
		t.Fatalf("final list: %v", err)
	}
	if len(lims) != want {
		t.Fatalf("条数不稳定：got %d, want %d", len(lims), want)
	}
	if lims[0].LType != model.LimTerritorial {
		t.Fatalf("类型不符：got %s, want %s", lims[0].LType, model.LimTerritorial)
	}
}

// TestParseLimitationsConflictRollbackAtomic 并发解析含冲突限制语的段落时，
// 每次都应稳定返回冲突错误，且不留下部分写入的限制语。
func TestParseLimitationsConflictRollbackAtomic(t *testing.T) {
	st, segID := newStoreWithSegment(t)
	svc := New(st)
	ctx := context.Background()

	// 改写为含肯定+否定冲突的文本
	seg, err := st.GetSegment(ctx, segID)
	if err != nil {
		t.Fatalf("get segment: %v", err)
	}
	seg.Text = "本院判令限于A省行政区域内适用，但不包括境外情形。"
	if _, err := st.DB.ExecContext(ctx,
		`UPDATE judgment_segment SET text = ? WHERE id = ?`, seg.Text, segID); err != nil {
		t.Fatalf("update text: %v", err)
	}

	const colleagues = 20
	var wg sync.WaitGroup
	var conflictCount int
	var mu sync.Mutex
	wg.Add(colleagues)
	for i := 0; i < colleagues; i++ {
		go func() {
			defer wg.Done()
			_, err := svc.ParseLimitations(ctx, segID)
			if err == nil {
				t.Errorf("期望冲突错误，得到 nil")
				return
			}
			mu.Lock()
			conflictCount++
			mu.Unlock()
		}()
	}
	wg.Wait()

	if conflictCount != colleagues {
		t.Fatalf("冲突返回次数不符：got %d, want %d", conflictCount, colleagues)
	}
	lims, err := st.ListLimitations(ctx, segID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(lims) != 0 {
		t.Fatalf("冲突解析不应留下部分限制语：got %d 条", len(lims))
	}
}
