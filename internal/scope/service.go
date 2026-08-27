package scope

import (
	"context"
	"fmt"
	"strings"
	"time"

	"task281-citereview/internal/model"
	"task281-citereview/internal/store"
)

// Service 范围模块：解析判决段限制语，并比较引证双方的前提集合以判定适用范围。
type Service struct {
	Store *store.Store
}

// New 构造范围模块服务。
func New(st *store.Store) *Service { return &Service{Store: st} }

// limitationRules 定义四类限制语的识别规则：肯定式限定词与否定式排除词。
var limitationRules = []struct {
	Type      model.LimitationType
	Inclusive []string
	Exclusive []string
}{
	{model.LimTerritorial, []string{"限于", "仅限于", "在", "境内", "本地区", "本省", "本市", "属地"}, []string{"不包括", "除外", "不限于"}},
	{model.LimTemporal, []string{"自", "期间", "暂行", "过渡期", "起至"}, []string{"不包括", "除外"}},
	{model.LimSubject, []string{"仅适用于", "针对", "主体为", "限于主体"}, []string{"不适用于主体"}},
	{model.LimMatter, []string{"就", "关于", "限于行为", "限于事项"}, []string{"不适用于情形", "除外"}},
}

// splitSentences 按中文句末标点切分句子。
func splitSentences(text string) []string {
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return r == '。' || r == '；' || r == '！' || r == '？' || r == '\n' || r == ';'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ParseLimitations 解析判决段的限制语。规则：
//   - 命中某类型肯定式限定词的句子，归入该类型；
//   - 同类型内同时出现肯定式与否定式（排除词）→ 视为限制语冲突，整体拒绝并回滚；
//   - 同类型内文本完全相同的限制语去重。
//
// 并发安全：对同一段的解析以段级互斥锁串行化，且"清空+插入"在单一事务内完成，
// 因此无论多少并发解析同时命中同一段，完成后该段限制语条数始终稳定一致，
// 并发读取也不会撞见"已清空尚未插入"的中间空列表。
func (s *Service) ParseLimitations(ctx context.Context, segmentID int64) ([]*model.LimitationClause, error) {
	seg, err := s.Store.GetSegment(ctx, segmentID)
	if err != nil {
		return nil, err
	}
	sentences := splitSentences(seg.Text)
	type pending struct {
		ltype     model.LimitationType
		text      string
		inclusive bool
		exclusive bool
	}
	groups := map[model.LimitationType][]pending{}
	for _, sent := range sentences {
		for _, rule := range limitationRules {
			inclusive := containsAny(sent, rule.Inclusive)
			exclusive := containsAny(sent, rule.Exclusive)
			if inclusive || exclusive {
				groups[rule.Type] = append(groups[rule.Type], pending{rule.Type, sent, inclusive, exclusive})
				break
			}
		}
	}
	// 冲突检测：同类型内同时含肯定式与否定式 → 限制语冲突
	for _, items := range groups {
		hasInc, hasExc := false, false
		for _, it := range items {
			if it.exclusive {
				hasExc = true
			}
			if it.inclusive {
				hasInc = true
			}
		}
		if hasInc && hasExc {
			return nil, model.ErrLimitationConflict
		}
	}
	// 组装待写入限制语（去重），与段锁、事务解耦
	seen := map[string]bool{}
	var pendingCreate []*model.LimitationClause
	for _, items := range groups {
		for _, it := range items {
			if seen[it.text] {
				continue
			}
			seen[it.text] = true
			pendingCreate = append(pendingCreate, &model.LimitationClause{
				SegmentID: segmentID,
				BatchID:   seg.BatchID,
				LType:     it.ltype,
				Text:      it.text,
			})
		}
	}
	if err := s.Store.ReplaceLimitations(ctx, segmentID, pendingCreate); err != nil {
		return nil, fmt.Errorf("replace limitations: %w", err)
	}
	return s.Store.ListLimitations(ctx, segmentID)
}

// CheckScope 比较一条引证关系双方的适用范围：
//   - 若被引方（原案）存在限制语，而引用方（后案）未以同类型限制语采纳、且正文未包含其范围词 → 判定"范围过宽"；
//   - 否则判定"适用"。
// 结论回写引证关系状态，并返回结构化报告。
func (s *Service) CheckScope(ctx context.Context, edgeID int64) (*model.ScopeReport, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(50 * time.Millisecond):
	}
	edge, err := s.Store.GetEdge(ctx, edgeID)
	if err != nil {
		return nil, err
	}
	cited, err := s.Store.GetSegment(ctx, edge.CitedSegmentID)
	if err != nil {
		return nil, err
	}
	citing, err := s.Store.GetSegment(ctx, edge.CitingSegmentID)
	if err != nil {
		return nil, err
	}
	citedLims, err := s.Store.ListLimitations(ctx, cited.ID)
	if err != nil {
		return nil, err
	}
	citingLims, err := s.Store.ListLimitations(ctx, citing.ID)
	if err != nil {
		return nil, err
	}

	var unack []model.LimitationType
	for _, cl := range citedLims {
		if !acknowledged(cl, citingLims, citing.Text) {
			unack = append(unack, cl.LType)
		}
	}

	var status model.CitationStatus
	var note string
	switch {
	case len(unack) > 0:
		status = model.CiteTooWide
		note = "后案引证未采纳原案的限制语，适用范围过宽"
	case len(citedLims) == 0:
		status = model.CiteApplicable
		note = "原案判决段无限制语，引证适用"
	default:
		status = model.CiteApplicable
		note = "后案已采纳原案全部限制语"
	}
	if err := s.Store.SetEdgeStatus(ctx, edge.ID, status); err != nil {
		return nil, err
	}
	return &model.ScopeReport{
		EdgeID:            edge.ID,
		Status:            status,
		CitingSegmentID:   citing.ID,
		CitedSegmentID:    cited.ID,
		Unacknowledged:    unack,
		Conflicts:         nil,
		Note:              note,
	}, nil
}

// acknowledged 判断引用方是否采纳了被引方的某条限制语：
// 同类型限制语存在，或其正文包含被引限制语的范围词，即视为已采纳。
func acknowledged(cl *model.LimitationClause, citingLims []*model.LimitationClause, citingText string) bool {
	for _, cl2 := range citingLims {
		if cl2.LType == cl.LType {
			return true
		}
	}
	token := extractScopeToken(cl.Text)
	if token != "" && strings.Contains(citingText, token) {
		return true
	}
	return false
}

// extractScopeToken 从限制语文本中提取核心范围词（去掉限定前缀/后缀动词）。
func extractScopeToken(text string) string {
	t := text
	for _, pre := range []string{"仅限于", "限于", "在", "境内", "本地区", "本省", "本市", "仅适用于", "针对", "主体为", "就", "关于"} {
		t = strings.ReplaceAll(t, pre, "")
	}
	for _, suf := range []string{"适用本规则", "适用本规定", "适用", "内适用", "范围内适用", "内", "范围内", "行使", "实施", "有效"} {
		t = strings.ReplaceAll(t, suf, "")
	}
	t = strings.TrimSpace(t)
	if len([]rune(t)) > 8 {
		t = string([]rune(t)[:8])
	}
	return t
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
