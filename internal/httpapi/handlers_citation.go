package httpapi

import (
	"fmt"
	"net/http"

	"task281-citereview/internal/model"
)

// createCitation 创建引证关系（含自引/环检测）：POST /api/batches/{id}/citations
func (s *Server) createCitation(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var body struct {
		CitingSegmentID int64  `json:"citing_segment_id"`
		CitedSegmentID  int64  `json:"cited_segment_id"`
		Relation        string `json:"relation"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	edge, err := s.Svc.Store.CreateEdge(r.Context(), &model.CitationEdge{
		BatchID:         id,
		CitingSegmentID: body.CitingSegmentID,
		CitedSegmentID:  body.CitedSegmentID,
		Relation:        body.Relation,
	})
	if err != nil {
		// 注意：statusForError 必须接收原始 err，否则经 fmt.Errorf 包裹后
		// 哨兵错误（自引/环/重复）的身份会被切断，误降级为 400。
		writeError(w, statusForError(err), fmt.Errorf("create citation: %w", err))
		return
	}
	writeJSON(w, http.StatusCreated, edge)
}

// listCitations 列出批次下引证关系：GET /api/batches/{id}/citations
func (s *Server) listCitations(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	edges, err := s.Svc.Store.ListEdges(r.Context(), id)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, edges)
}

// getCitation 获取单条引证关系：GET /api/citations/{id}
func (s *Server) getCitation(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	edge, err := s.Svc.Store.GetEdge(r.Context(), id)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, edge)
}

// checkScope 执行适用范围检查：POST /api/citations/{id}/check
func (s *Server) checkScope(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	report, err := s.Svc.Scope.CheckScope(r.Context(), id)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// decideCitation 对引证关系作出裁决：POST /api/citations/{id}/decide
func (s *Server) decideCitation(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var body struct {
		Status           model.CitationStatus `json:"status"`
		DistinctionReason string               `json:"distinction_reason"`
		GraphVersionID   int64                `json:"graph_version_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	dec, err := s.Svc.Decision.Decide(r.Context(), id, body.Status, body.DistinctionReason, body.GraphVersionID)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, dec)
}
