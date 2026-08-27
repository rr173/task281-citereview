package httpapi

import (
	"net/http"

	"task281-citereview/internal/service"
)

// NewMux 构造 HTTP 路由，全部业务入口以 /api 为前缀，根路径提供复核页。
func NewMux(svc *service.Service) *http.ServeMux {
	s := NewServer(svc)
	m := http.NewServeMux()

	// 元数据
	m.HandleFunc("GET /api/health", s.health)
	m.HandleFunc("GET /api/stats", s.stats)

	// 批次与材料
	m.HandleFunc("POST /api/batches", s.createBatch)
	m.HandleFunc("GET /api/batches", s.listBatches)
	m.HandleFunc("GET /api/batches/{id}", s.getBatch)
	m.HandleFunc("POST /api/batches/{id}/status", s.setBatchStatus)
	m.HandleFunc("POST /api/batches/{id}/import", s.importMaterial)
	m.HandleFunc("GET /api/batches/{id}/segments", s.listSegments)
	m.HandleFunc("GET /api/batches/{id}/graph", s.graphView)
	m.HandleFunc("POST /api/batches/{id}/versions", s.createVersion)
	m.HandleFunc("GET /api/batches/{id}/versions", s.listVersions)

	// 判决段、要素、限制语
	m.HandleFunc("GET /api/segments/{id}", s.getSegment)
	m.HandleFunc("POST /api/segments/{id}/elements", s.extractElements)
	m.HandleFunc("POST /api/segments/{id}/limitations", s.parseLimitations)
	m.HandleFunc("GET /api/segments/{id}/elements", s.listSegmentElements)
	m.HandleFunc("GET /api/segments/{id}/limitations", s.listSegmentLimitations)

	// 引证关系与裁决
	m.HandleFunc("POST /api/batches/{id}/citations", s.createCitation)
	m.HandleFunc("GET /api/batches/{id}/citations", s.listCitations)
	m.HandleFunc("GET /api/citations/{id}", s.getCitation)
	m.HandleFunc("POST /api/citations/{id}/check", s.checkScope)
	m.HandleFunc("POST /api/citations/{id}/decide", s.decideCitation)

	// 研究图版本
	m.HandleFunc("POST /api/versions/{id}/freeze", s.freezeVersion)
	m.HandleFunc("POST /api/versions/{id}/share", s.shareVersion)
	m.HandleFunc("POST /api/versions/{id}/supersede", s.supersedeVersion)

	// 复核页
	m.HandleFunc("GET /", s.indexPage)

	return m
}
