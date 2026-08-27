package model

// 本文件定义判例引证适用范围复核台的全部核心实体。
// 实体与 store 层表结构一一对应；服务层与 API 层共享这些类型。

// ResearchBatch 研究批次：一次判例引证复核工作的容器。
type ResearchBatch struct {
	ID        int64       `json:"id"`
	Code      string      `json:"code"`
	Title     string      `json:"title"`
	Status    BatchStatus `json:"status"`
	CreatedAt int64       `json:"created_at"`
	UpdatedAt int64       `json:"updated_at"`
}

// JudgmentSegment 判决段：从原始判决书/裁判要旨中切分出的可复核片段。
// SummaryHash 为文本归一化后的确定性摘要，用于导入幂等。
type JudgmentSegment struct {
	ID          int64         `json:"id"`
	BatchID     int64         `json:"batch_id"`
	SourceDoc   string        `json:"source_doc"`
	SeqNo       int           `json:"seq_no"`
	Text        string        `json:"text"`
	Status      SegmentStatus `json:"status"`
	SummaryHash string        `json:"summary_hash"`
	CreatedAt   int64         `json:"created_at"`
}

// FactualElement 事实要素：从判决段文本中抽取的事实点（主体/标的/地域/金额等）。
type FactualElement struct {
	ID          int64  `json:"id"`
	SegmentID   int64  `json:"segment_id"`
	BatchID     int64  `json:"batch_id"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	ElementType string `json:"element_type"`
	CreatedAt   int64  `json:"created_at"`
}

// LimitationClause 限制语：判决段中明确写出的适用范围限定（地域/时间/主体/事项）。
type LimitationClause struct {
	ID        int64          `json:"id"`
	SegmentID int64          `json:"segment_id"`
	BatchID   int64          `json:"batch_id"`
	LType     LimitationType `json:"ltype"`
	Text      string         `json:"text"`
	CreatedAt int64          `json:"created_at"`
}

// CitationEdge 引证关系：一条"后案判决段"对"原案判决段"的引用。
// CitingSegmentID 为引用方（后案），CitedSegmentID 为被引用方（原案）。
type CitationEdge struct {
	ID              int64         `json:"id"`
	BatchID         int64         `json:"batch_id"`
	CitingSegmentID int64         `json:"citing_segment_id"`
	CitedSegmentID  int64         `json:"cited_segment_id"`
	Relation        string        `json:"relation"`
	Status          CitationStatus `json:"status"`
	CreatedAt       int64         `json:"created_at"`
	UpdatedAt       int64         `json:"updated_at"`
}

// Decision 裁决：研究者对一条引证关系给出的适用范围结论与区分理由。
// GraphVersionID 指向作出裁决时的研究图版本，用于版本校验。
type Decision struct {
	ID               int64         `json:"id"`
	BatchID          int64         `json:"batch_id"`
	EdgeID           int64         `json:"edge_id"`
	Status           CitationStatus `json:"status"`
	DistinctionReason string       `json:"distinction_reason"`
	GraphVersionID   int64         `json:"graph_version_id"`
	CreatedAt        int64         `json:"created_at"`
	UpdatedAt        int64         `json:"updated_at"`
}

// ResearchGraphVersion 研究图版本：将批次内段落、要素、限制语、引证与裁决整体快照化。
type ResearchGraphVersion struct {
	ID               int64              `json:"id"`
	BatchID          int64             `json:"batch_id"`
	VersionNo        int               `json:"version_no"`
	Status           GraphVersionStatus `json:"status"`
	MaterialSnapshot string            `json:"material_snapshot"`
	CreatedAt        int64             `json:"created_at"`
}

// GraphView 引证图视图：Web 复核页与 /api/graph 返回的聚合结构。
type GraphView struct {
	Batch       *ResearchBatch        `json:"batch"`
	Segments    []*JudgmentSegment    `json:"segments"`
	Elements    []*FactualElement     `json:"elements"`
	Limitations []*LimitationClause   `json:"limitations"`
	Edges       []*CitationEdge       `json:"edges"`
	Decisions   []*Decision           `json:"decisions"`
	Version     *ResearchGraphVersion `json:"version"`
}

// ScopeReport 范围检查报告：针对单条引证关系给出的适用结论与冲突明细。
type ScopeReport struct {
	EdgeID            int64            `json:"edge_id"`
	Status            CitationStatus   `json:"status"`
	CitingSegmentID   int64            `json:"citing_segment_id"`
	CitedSegmentID    int64            `json:"cited_segment_id"`
	Unacknowledged    []LimitationType `json:"unacknowledged_limitations"`
	Conflicts         []string         `json:"conflicts"`
	Note              string           `json:"note"`
}
