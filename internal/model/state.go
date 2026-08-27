package model

// 本文件定义判例引证适用范围复核台的全部状态机枚举与合法流转。
// 状态命名采用英文枚举值，便于持久化与 API 序列化；中文注释给出业务语义。

// BatchStatus 研究批次状态：整理中→待分析→待裁决→已发布→封存。
type BatchStatus string

const (
	BatchOrganizing BatchStatus = "organizing" // 整理中
	BatchAnalyzing  BatchStatus = "analyzing"  // 待分析
	BatchDeciding   BatchStatus = "deciding"   // 待裁决
	BatchPublished  BatchStatus = "published"  // 已发布
	BatchSealed     BatchStatus = "sealed"     // 封存
)

// SegmentStatus 判决段状态：待解析→有效/限定/排除。
type SegmentStatus string

const (
	SegPendingParse SegmentStatus = "pending_parse" // 待解析
	SegValid        SegmentStatus = "valid"         // 有效
	SegLimited      SegmentStatus = "limited"       // 限定
	SegExcluded     SegmentStatus = "excluded"      // 排除
)

// CitationStatus 引证关系/裁决状态。
type CitationStatus string

const (
	CiteCandidate      CitationStatus = "candidate"       // 候选
	CiteApplicable     CitationStatus = "applicable"      // 适用
	CiteTooWide        CitationStatus = "scope_too_wide"  // 范围过宽
	CiteDistinguished  CitationStatus = "distinguished"   // 区分
	CiteConfirmed      CitationStatus = "confirmed"       // 确认
)

// GraphVersionStatus 研究图版本状态：草稿→共享→冻结→替代。
type GraphVersionStatus string

const (
	GVDraft      GraphVersionStatus = "draft"      // 草稿
	GVShared     GraphVersionStatus = "shared"     // 共享
	GVFrozen     GraphVersionStatus = "frozen"     // 冻结
	GVSuperseded GraphVersionStatus = "superseded" // 替代
)

// LimitationType 限制语类型。
type LimitationType string

const (
	LimTerritorial LimitationType = "territorial" // 地域限定
	LimTemporal    LimitationType = "temporal"    // 时间限定
	LimSubject     LimitationType = "subject"     // 主体限定
	LimMatter      LimitationType = "matter"      // 事项限定
)

// ValidBatchTransition 校验研究批次状态流转是否合法。
func ValidBatchTransition(from, to BatchStatus) bool {
	allowed := map[BatchStatus][]BatchStatus{
		BatchOrganizing: {BatchAnalyzing, BatchSealed},
		BatchAnalyzing:  {BatchDeciding, BatchSealed},
		BatchDeciding:   {BatchPublished, BatchSealed},
		BatchPublished:  {BatchSealed},
		BatchSealed:     {},
	}
	for _, ok := range allowed[from] {
		if ok == to {
			return true
		}
	}
	return false
}

// ValidSegmentTransition 校验判决段状态流转是否合法。
func ValidSegmentTransition(from, to SegmentStatus) bool {
	allowed := map[SegmentStatus][]SegmentStatus{
		SegPendingParse: {SegValid, SegLimited, SegExcluded},
		SegValid:        {SegLimited, SegExcluded},
		SegLimited:      {SegValid, SegExcluded},
		SegExcluded:     {SegValid, SegLimited},
	}
	for _, ok := range allowed[from] {
		if ok == to {
			return true
		}
	}
	return false
}

// ValidGraphVersionTransition 校验研究图版本状态流转是否合法。
func ValidGraphVersionTransition(from, to GraphVersionStatus) bool {
	allowed := map[GraphVersionStatus][]GraphVersionStatus{
		GVDraft:      {GVShared, GVFrozen},
		GVShared:     {GVFrozen, GVSuperseded},
		GVFrozen:     {GVSuperseded},
		GVSuperseded: {},
	}
	for _, ok := range allowed[from] {
		if ok == to {
			return true
		}
	}
	return false
}

// LimitationTypes 返回全部限制语类型，供解析与校验使用。
func LimitationTypes() []LimitationType {
	return []LimitationType{LimTerritorial, LimTemporal, LimSubject, LimMatter}
}

// CitationStatuses 返回全部引证/裁决状态。
func CitationStatuses() []CitationStatus {
	return []CitationStatus{CiteCandidate, CiteApplicable, CiteTooWide, CiteDistinguished, CiteConfirmed}
}
