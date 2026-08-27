package element

import (
	"context"
	"fmt"
	"strings"

	"task281-citereview/internal/model"
	"task281-citereview/internal/store"
)

// Service 要素模块：从判决段文本中抽取事实要素（主体、标的、地域、金额、时间等）。
type Service struct {
	Store *store.Store
}

// New 构造要素模块服务。
func New(st *store.Store) *Service { return &Service{Store: st} }

// factualMarkers 定义可抽取的事实要素标记词及其要素类型。
var factualMarkers = []struct {
	Marker string
	Type   string
}{
	{"原告", "party"},
	{"被告", "party"},
	{"申请人", "party"},
	{"被申请人", "party"},
	{"案由", "cause"},
	{"标的", "object"},
	{"金额", "amount"},
	{"赔偿", "amount"},
	{"地点", "place"},
	{"住所地", "place"},
	{"境内", "place"},
	{"时间", "time"},
	{"期间", "time"},
	{"主体", "subject"},
}

// splitSentences 按中文句末标点切分句子，保留每个句子的原始文本。
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

// ExtractElements 抽取判决段的事实要素。抽取前先清空该段既有要素，
// 保证重复抽取产生稳定、幂等的结果。
func (s *Service) ExtractElements(ctx context.Context, segmentID int64) ([]*model.FactualElement, error) {
	seg, err := s.Store.GetSegment(ctx, segmentID)
	if err != nil {
		return nil, err
	}
	if err := s.clearElements(ctx, segmentID); err != nil {
		return nil, err
	}
	sentences := splitSentences(seg.Text)
	seen := map[string]bool{}
	var out []*model.FactualElement
	for _, sent := range sentences {
		for _, m := range factualMarkers {
			if !strings.Contains(sent, m.Marker) {
				continue
			}
			key := fmt.Sprintf("%s#%s", m.Type, m.Marker)
			dedup := key + "|" + sent
			if seen[dedup] {
				continue
			}
			seen[dedup] = true
			el := &model.FactualElement{
				SegmentID:   segmentID,
				BatchID:     seg.BatchID,
				Key:         m.Marker,
				Value:       sent,
				ElementType: m.Type,
			}
			created, err := s.Store.CreateElement(ctx, el)
			if err != nil {
				return nil, fmt.Errorf("create element: %w", err)
			}
			out = append(out, created)
		}
	}
	return out, nil
}

// clearElements 删除某判决段下全部事实要素（抽取前重置）。
func (s *Service) clearElements(ctx context.Context, segmentID int64) error {
	if _, err := s.Store.DB.ExecContext(ctx,
		`DELETE FROM factual_element WHERE segment_id = ?`, segmentID); err != nil {
		return fmt.Errorf("clear elements: %w", err)
	}
	return nil
}
