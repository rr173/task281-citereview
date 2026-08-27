package httpapi

import (
	"net/http"

	"task281-citereview/internal/model"
)

// createBatch 创建研究批次：POST /api/batches
func (s *Server) createBatch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code  string `json:"code"`
		Title string `json:"title"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.Code == "" {
		writeError(w, http.StatusBadRequest, model.ErrDuplicateSummary)
		return
	}
	b, err := s.Svc.Store.CreateBatch(r.Context(), body.Code, body.Title)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

// listBatches 列出全部研究批次：GET /api/batches
func (s *Server) listBatches(w http.ResponseWriter, r *http.Request) {
	bs, err := s.Svc.Store.ListBatches(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, bs)
}

// getBatch 获取单个研究批次：GET /api/batches/{id}
func (s *Server) getBatch(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	b, err := s.Svc.Store.GetBatch(r.Context(), id)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// setBatchStatus 按状态机流转研究批次：POST /api/batches/{id}/status
func (s *Server) setBatchStatus(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var body struct {
		Status model.BatchStatus `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.Svc.Store.SetBatchStatus(r.Context(), id, body.Status); err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	b, err := s.Svc.Store.GetBatch(r.Context(), id)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// importMaterial 导入判例材料并切分判决段：POST /api/batches/{id}/import
func (s *Server) importMaterial(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var body struct {
		SourceDoc string   `json:"source_doc"`
		Segments  []string `json:"segments"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	segs, err := s.Svc.Material.ImportMaterial(r.Context(), id, body.SourceDoc, body.Segments)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, segs)
}

// listSegments 列出批次下判决段：GET /api/batches/{id}/segments
func (s *Server) listSegments(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	segs, err := s.Svc.Store.ListSegments(r.Context(), id)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, segs)
}
