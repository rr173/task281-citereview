package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"task281-citereview/internal/model"
	"task281-citereview/internal/service"
)

// Server 持有编排服务，承载全部 HTTP 处理器。
type Server struct {
	Svc *service.Service
}

// NewServer 构造 HTTP 服务。
func NewServer(svc *service.Service) *Server { return &Server{Svc: svc} }

// writeJSON 以 JSON 写回响应体。
func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError 以统一错误结构写回错误。
func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]interface{}{"error": err.Error()})
}

// decodeJSON 解析请求体到目标结构。
func decodeJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}

// parseID 从路径通配符读取 int64 类型 ID。
func parseID(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}

// statusForError 将领域错误映射为 HTTP 状态码。
func statusForError(err error) int {
	switch {
	case errors.Is(err, model.ErrBatchNotFound), errors.Is(err, model.ErrSegmentNotFound), errors.Is(err, model.ErrEdgeNotFound),
		errors.Is(err, model.ErrDecisionNotFound), errors.Is(err, model.ErrVersionNotFound):
		return http.StatusNotFound
	case errors.Is(err, model.ErrSelfCitation), errors.Is(err, model.ErrCitationCycle), errors.Is(err, model.ErrDuplicateEdge),
		errors.Is(err, model.ErrFrozenImmutable), errors.Is(err, model.ErrVersionMismatch), errors.Is(err, model.ErrElementMissing),
		errors.Is(err, model.ErrLimitationConflict), errors.Is(err, model.ErrDuplicateLimitation), errors.Is(err, model.ErrInvalidTransition),
		errors.Is(err, model.ErrDuplicateSummary):
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}
