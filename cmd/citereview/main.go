package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"task281-citereview/internal/model"
	"task281-citereview/internal/service"
	"task281-citereview/internal/store"
	"task281-citereview/internal/httpapi"
)

func main() {
	var addr, dbPath string
	var smoke bool
	flag.StringVar(&addr, "addr", ":8080", "HTTP 监听地址")
	flag.StringVar(&dbPath, "db", "citereview.db", "SQLite 数据库路径")
	flag.BoolVar(&smoke, "smoke-test", false, "执行自检后退出，不启动长驻服务")
	flag.Parse()

	if smoke {
		if err := runSmoke(dbPath); err != nil {
			fmt.Fprintln(os.Stderr, "smoke-test FAILED:", err)
			os.Exit(1)
		}
		fmt.Println("smoke-test PASSED")
		os.Exit(0)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open db:", err)
		os.Exit(1)
	}
	defer db.Close()

	svc := service.New(db)
	mux := httpapi.NewMux(svc)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	fmt.Println("citereview listening on", addr)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		os.Exit(1)
	}
}

// runSmoke 真实创建批次/段落/要素/限制语/引证，跑范围检查与裁决，
// 随后关闭并重新打开数据库校验持久化与重启恢复，最后以 nil 错误退出。
func runSmoke(dbPath string) error {
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("citereview_smoke_%d.db", time.Now().UnixNano()))
	defer os.Remove(tmp)

	// 首次打开并构建业务闭环
	db, err := store.Open(tmp)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	svc := service.New(db)
	ctx := context.Background()

	batch, err := svc.Store.CreateBatch(ctx, "SMOKE-1", "判例引证冒烟测试")
	if err != nil {
		return fmt.Errorf("create batch: %w", err)
	}

	// 导入原案（含地域限定）与后案（忽略地域限定）
	segs, err := svc.Material.ImportMaterial(ctx, batch.ID, "判例集", []string{
		"本院判令限于A省行政区域内适用本规则，被告应于期内履行给付义务。", // 原案：含地域限定
		"本院认为前述判例可予适用，原告请求成立，被告应承担责任。",         // 后案：忽略地域限定
	})
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}
	if len(segs) != 2 {
		return fmt.Errorf("expected 2 segments, got %d", len(segs))
	}
	cited, citing := segs[0], segs[1]

	// 抽取要素与解析限制语
	if _, err := svc.Element.ExtractElements(ctx, cited.ID); err != nil {
		return fmt.Errorf("extract cited: %w", err)
	}
	if _, err := svc.Element.ExtractElements(ctx, citing.ID); err != nil {
		return fmt.Errorf("extract citing: %w", err)
	}
	if _, err := svc.Scope.ParseLimitations(ctx, cited.ID); err != nil {
		return fmt.Errorf("parse cited limits: %w", err)
	}
	if _, err := svc.Scope.ParseLimitations(ctx, citing.ID); err != nil {
		return fmt.Errorf("parse citing limits: %w", err)
	}

	// 创建引证关系：后案 → 原案
	edge, err := svc.Store.CreateEdge(ctx, &model.CitationEdge{
		BatchID:         batch.ID,
		CitingSegmentID: citing.ID,
		CitedSegmentID:  cited.ID,
		Relation:        "遵循先例",
	})
	if err != nil {
		return fmt.Errorf("create edge: %w", err)
	}

	// 范围检查：应判定"范围过宽"
	report, err := svc.Scope.CheckScope(ctx, edge.ID)
	if err != nil {
		return fmt.Errorf("check scope: %w", err)
	}
	if report.Status != model.CiteTooWide {
		return fmt.Errorf("expected scope_too_wide, got %s", report.Status)
	}
	if len(report.Unacknowledged) == 0 {
		return fmt.Errorf("expected unacknowledged limitations, got none")
	}

	// 自引与环检测：反向边应被拒绝（会成环）
	if _, err := svc.Store.CreateEdge(ctx, &model.CitationEdge{
		BatchID:         batch.ID,
		CitingSegmentID: cited.ID,
		CitedSegmentID:  citing.ID,
	}); err != nil && !errors.Is(err, model.ErrCitationCycle) {
		return fmt.Errorf("expected citation cycle rejection, got %v", err)
	}
	if _, err := svc.Store.CreateEdge(ctx, &model.CitationEdge{
		BatchID:         batch.ID,
		CitingSegmentID: citing.ID,
		CitedSegmentID:  citing.ID,
	}); err != nil && !errors.Is(err, model.ErrSelfCitation) {
		return fmt.Errorf("expected self-citation rejection, got %v", err)
	}

	// 创建研究图草稿版本
	ver, err := svc.Graph.CreateDraft(ctx, batch.ID)
	if err != nil {
		return fmt.Errorf("create version: %w", err)
	}

	// 裁决：研究者补充区分理由，标记"区分"
	dec, err := svc.Decision.Decide(ctx, edge.ID, model.CiteDistinguished, "后案已另行说明属地例外，不构成误引", ver.ID)
	if err != nil {
		return fmt.Errorf("decide: %w", err)
	}
	if dec.Status != model.CiteDistinguished {
		return fmt.Errorf("expected distinguished decision, got %s", dec.Status)
	}

	// 冻结版本（快照定型）
	if err := svc.Graph.Freeze(ctx, ver.ID); err != nil {
		return fmt.Errorf("freeze: %w", err)
	}

	// 批次状态机推进至已发布
	for _, st := range []model.BatchStatus{model.BatchAnalyzing, model.BatchDeciding, model.BatchPublished} {
		if err := svc.Store.SetBatchStatus(ctx, batch.ID, st); err != nil {
			return fmt.Errorf("set batch status %s: %w", st, err)
		}
	}

	// 关闭数据库，模拟重启
	if err := db.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}

	// 重新打开，验证持久化与重启恢复
	db2, err := store.Open(tmp)
	if err != nil {
		return fmt.Errorf("reopen: %w", err)
	}
	defer db2.Close()
	svc2 := service.New(db2)

	rb, err := svc2.Store.GetBatch(ctx, batch.ID)
	if err != nil {
		return fmt.Errorf("reload batch: %w", err)
	}
	if rb.Status != model.BatchPublished {
		return fmt.Errorf("reloaded batch status = %s, want published", rb.Status)
	}
	rsegs, err := svc2.Store.ListSegments(ctx, batch.ID)
	if err != nil {
		return fmt.Errorf("reload segments: %w", err)
	}
	if len(rsegs) != 2 {
		return fmt.Errorf("reloaded segment count = %d, want 2", len(rsegs))
	}
	redge, err := svc2.Store.GetEdge(ctx, edge.ID)
	if err != nil {
		return fmt.Errorf("reload edge: %w", err)
	}
	if redge.Status != model.CiteDistinguished {
		return fmt.Errorf("reloaded edge status = %s, want distinguished", redge.Status)
	}
	rdec, err := svc2.Decision.Latest(ctx, edge.ID)
	if err != nil {
		return fmt.Errorf("reload decision: %w", err)
	}
	if rdec.Status != model.CiteDistinguished {
		return fmt.Errorf("reloaded decision status = %s, want distinguished", rdec.Status)
	}
	rver, err := svc2.Store.GetVersion(ctx, ver.ID)
	if err != nil {
		return fmt.Errorf("reload version: %w", err)
	}
	if rver.Status != model.GVFrozen {
		return fmt.Errorf("reloaded version status = %s, want frozen", rver.Status)
	}
	return nil
}
