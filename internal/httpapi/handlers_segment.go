package httpapi

import (
	"net/http"
)

// getSegment 获取单个判决段：GET /api/segments/{id}
func (s *Server) getSegment(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	seg, err := s.Svc.Store.GetSegment(r.Context(), id)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, seg)
}

// extractElements 抽取判决段事实要素：POST /api/segments/{id}/elements
func (s *Server) extractElements(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	els, err := s.Svc.Element.ExtractElements(r.Context(), id)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, els)
}

// parseLimitations 解析判决段限制语：POST /api/segments/{id}/limitations
func (s *Server) parseLimitations(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	lims, err := s.Svc.Scope.ParseLimitations(r.Context(), id)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, lims)
}

// listSegmentElements 列出判决段事实要素：GET /api/segments/{id}/elements
func (s *Server) listSegmentElements(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	els, err := s.Svc.Store.ListElements(r.Context(), id)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, els)
}

// listSegmentLimitations 列出判决段限制语：GET /api/segments/{id}/limitations
func (s *Server) listSegmentLimitations(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	lims, err := s.Svc.Store.ListLimitations(r.Context(), id)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, lims)
}
