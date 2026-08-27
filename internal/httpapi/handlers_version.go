package httpapi

import (
	"net/http"

	"task281-citereview/internal/model"
)

// createVersion 创建研究图草稿版本：POST /api/batches/{id}/versions
func (s *Server) createVersion(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ver, err := s.Svc.Graph.CreateDraft(r.Context(), id)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, ver)
}

// listVersions 列出批次下研究图版本：GET /api/batches/{id}/versions
func (s *Server) listVersions(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	vers, err := s.Svc.Store.ListVersions(r.Context(), id)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, vers)
}

// freezeVersion 冻结研究图版本：POST /api/versions/{id}/freeze
func (s *Server) freezeVersion(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.Svc.Graph.Freeze(r.Context(), id); err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	ver, _ := s.Svc.Store.GetVersion(r.Context(), id)
	writeJSON(w, http.StatusOK, ver)
}

// shareVersion 共享研究图版本：POST /api/versions/{id}/share
func (s *Server) shareVersion(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.Svc.Graph.Share(r.Context(), id); err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	ver, _ := s.Svc.Store.GetVersion(r.Context(), id)
	writeJSON(w, http.StatusOK, ver)
}

// supersedeVersion 替代研究图版本并新建草稿：POST /api/versions/{id}/supersede
func (s *Server) supersedeVersion(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ver, err := s.Svc.Graph.Supersede(r.Context(), id)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, ver)
}

// graphView 返回引证图聚合视图：GET /api/batches/{id}/graph
func (s *Server) graphView(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	view, err := s.Svc.Graph.BuildView(r.Context(), id)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

var _ = model.GVDraft // 确保 model 包被引用（版本状态枚举供前端展示）
