package httpapi

import (
	"net/http"
)

// health 健康检查：GET /api/health
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// stats 聚合统计：GET /api/stats
func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var batches, segments, elements, limits, edges, decisions, versions int
	_ = s.Svc.Store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM research_batch`).Scan(&batches)
	_ = s.Svc.Store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM judgment_segment`).Scan(&segments)
	_ = s.Svc.Store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM factual_element`).Scan(&elements)
	_ = s.Svc.Store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM limitation_clause`).Scan(&limits)
	_ = s.Svc.Store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM citation_edge`).Scan(&edges)
	_ = s.Svc.Store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM decision`).Scan(&decisions)
	_ = s.Svc.Store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM research_graph_version`).Scan(&versions)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"batches":   batches,
		"segments":  segments,
		"elements":  elements,
		"limits":    limits,
		"edges":     edges,
		"decisions": decisions,
		"versions":  versions,
	})
}

// indexPage 复核页：GET /
// 以最简 HTML 渲染引证图（段落、限制语、引证与裁决），供浏览器复核。
func (s *Server) indexPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeError(w, http.StatusNotFound, http.ErrBodyNotAllowed)
		return
	}
	ctx := r.Context()
	batchID := r.URL.Query().Get("batch")
	html := `<!doctype html><html lang="zh"><head><meta charset="utf-8">
<title>判例引证适用范围复核台</title>
<style>body{font-family:-apple-system,Segoe UI,Roboto,sans-serif;margin:24px;color:#1f2937}
h1{font-size:20px}.card{border:1px solid #e5e7eb;border-radius:8px;padding:12px;margin:8px 0}
.lim{color:#b45309}.wide{color:#b91c1c}.ok{color:#15803d}
code{background:#f3f4f6;padding:1px 4px;border-radius:4px}</style></head><body>
<h1>判例引证适用范围复核台</h1>
<p>使用 <code>GET /api/batches</code> 创建批次，<code>POST /api/batches/{id}/import</code> 导入判决段，
<code>POST /api/segments/{id}/limitations</code> 解析限制语，<code>POST /api/citations/{id}/check</code> 检查适用范围。</p>`
	if batchID != "" {
		view, err := s.Svc.Graph.BuildView(ctx, atoiSafe(batchID))
		if err == nil && view != nil {
			html += "<h2>引证图（批次 " + batchID + "）</h2>"
			for _, seg := range view.Segments {
				html += `<div class="card"><b>段落 #` + itoaSafe(seg.ID) + `</b> [` + string(seg.Status) + `] ` + escapeHTML(seg.Text) + `</div>`
			}
			for _, e := range view.Edges {
				cls := "ok"
				if e.Status == "scope_too_wide" {
					cls = "wide"
				}
				html += `<div class="card ` + cls + `">引证 #` + itoaSafe(e.ID) + `：段落 ` + itoaSafe(e.CitingSegmentID) + ` → 段落 ` + itoaSafe(e.CitedSegmentID) + ` 状态=<b>` + string(e.Status) + `</b></div>`
			}
			for _, d := range view.Decisions {
				html += `<div class="card">裁决 #` + itoaSafe(d.ID) + `：状态 ` + string(d.Status) + ` 理由=` + escapeHTML(d.DistinctionReason) + `</div>`
			}
		}
	}
	html += `</body></html>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}
