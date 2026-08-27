package model

import "errors"

// 领域错误集合。所有错误均为哨兵错误，便于服务层与 API 层按类型映射 HTTP 状态码。

var (
	// 实体未找到
	ErrBatchNotFound   = errors.New("research batch not found")
	ErrSegmentNotFound = errors.New("judgment segment not found")
	ErrEdgeNotFound    = errors.New("citation edge not found")
	ErrDecisionNotFound = errors.New("decision not found")
	ErrVersionNotFound = errors.New("research graph version not found")

	// 引证图约束
	ErrSelfCitation      = errors.New("self citation is not allowed")
	ErrCitationCycle     = errors.New("citation would create a cycle in the research graph")
	ErrDuplicateEdge     = errors.New("citation edge already exists for the same segment pair")

	// 冻结与版本约束
	ErrFrozenImmutable = errors.New("cannot modify entities within a frozen research graph version")
	ErrVersionMismatch = errors.New("decision references a non-existent or mismatched graph version")

	// 要素与限制语约束
	ErrElementMissing       = errors.New("cited or citing segment has no extracted factual elements")
	ErrLimitationConflict   = errors.New("conflicting limitation clauses detected on the same segment")
	ErrDuplicateLimitation  = errors.New("duplicate limitation clause of the same type on a segment")

	// 状态机约束
	ErrInvalidTransition = errors.New("invalid state transition")
	ErrDuplicateSummary  = errors.New("segment summary already exists (idempotent import)")
)
