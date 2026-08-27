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
// 使用 errors.Is 解包错误链，确保即使错误被上层 fmt.Errorf("%w") 包裹，
// 哨兵错误（如自引 ErrSelfCitation）仍能正确映射到 409，而非误降级为 400。
func statusForError(err error) int {
	notFound := []error{
		model.ErrBatchNotFound, model.ErrSegmentNotFound, model.ErrEdgeNotFound,
		model.ErrDecisionNotFound, model.ErrVersionNotFound,
	}
	for _, e := range notFound {
		if errors.Is(err, e) {
			return http.StatusNotFound
		}
	}
	conflict := []error{
		model.ErrSelfCitation, model.ErrCitationCycle, model.ErrDuplicateEdge,
		model.ErrFrozenImmutable, model.ErrVersionMismatch, model.ErrElementMissing,
		model.ErrLimitationConflict, model.ErrDuplicateLimitation, model.ErrInvalidTransition,
		model.ErrDuplicateSummary,
	}
	for _, e := range conflict {
		if errors.Is(err, e) {
			return http.StatusConflict
		}
	}
	return http.StatusBadRequest
}
