package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"task281-citereview/internal/model"
)

// TestStatusForError_SentinelWrapping 确保被 fmt.Errorf("%w", ...) 包装的领域哨兵错误
// 仍能正确映射到对应 HTTP 状态码。回归测试：非法批次状态流转此前用 %v 包装，
// 导致 errors.Is 失败、状态码错误地回退为 400。
func TestStatusForError_SentinelWrapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"bare invalid transition", model.ErrInvalidTransition, http.StatusConflict},
		{"wrapped invalid transition", fmt.Errorf("%w: organizing -> published", model.ErrInvalidTransition), http.StatusConflict},
		{"bare frozen immutable", model.ErrFrozenImmutable, http.StatusConflict},
		{"wrapped frozen immutable", fmt.Errorf("%w: version 1 is frozen", model.ErrFrozenImmutable), http.StatusConflict},
		{"bare not found", model.ErrBatchNotFound, http.StatusNotFound},
		{"wrapped not found", fmt.Errorf("%w: extra context", model.ErrSegmentNotFound), http.StatusNotFound},
		{"unknown error", errors.New("bogus"), http.StatusBadRequest},
	}
	for _, c := range cases {
		if got := statusForError(c.err); got != c.want {
			t.Errorf("%s: statusForError(%v) = %d, want %d", c.name, c.err, got, c.want)
		}
	}
}
