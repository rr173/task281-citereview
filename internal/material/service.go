package material

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"task281-citereview/internal/model"
	"task281-citereview/internal/store"
)

// Service 材料模块：负责原始判例材料的导入与判决段切分、幂等摘要。
type Service struct {
	Store *store.Store
}

// New 构造材料模块服务。
func New(st *store.Store) *Service { return &Service{Store: st} }

// normalizeText 对判决段文本做归一化：去首尾空白、合并连续空白。
// 归一化结果用于确定性摘要，保证同素材重复导入得到同一摘要哈希（幂等）。
func normalizeText(text string) string {
	t := strings.TrimSpace(text)
	t = strings.ReplaceAll(t, "\r", " ")
	t = strings.ReplaceAll(t, "\n", " ")
	for strings.Contains(t, "  ") {
		t = strings.ReplaceAll(t, "  ", " ")
	}
	return t
}

// SummaryHash 计算判决段的确定性摘要哈希。
func SummaryHash(text string) string {
	h := sha256.Sum256([]byte(normalizeText(text)))
	return fmt.Sprintf("%x", h)
}

// ImportSegment 导入单条判决段（按 source_doc + seq_no + 文本）。
// 若同批次内已存在相同归一化文本的段落，则直接返回既有段落（幂等导入）。
func (s *Service) ImportSegment(ctx context.Context, batchID int64, sourceDoc string, seqNo int, text string) (*model.JudgmentSegment, error) {
	hash := SummaryHash(text)
	existing, err := s.Store.GetSegmentByHash(ctx, batchID, hash)
	if err == nil && existing != nil {
		// 幂等：已存在相同摘要的段落，不重复创建
		return existing, nil
	}
	seg := &model.JudgmentSegment{
		BatchID:     batchID,
		SourceDoc:   sourceDoc,
		SeqNo:       seqNo,
		Text:        normalizeText(text),
		SummaryHash: hash,
	}
	created, err := s.Store.CreateSegment(ctx, seg)
	if err != nil {
		return nil, err
	}
	// 导入即进入有效状态（待后续要素/限制语抽取）
	if err := s.Store.SetSegmentStatus(ctx, created.ID, model.SegValid); err != nil {
		return nil, err
	}
	return s.Store.GetSegment(ctx, created.ID)
}

// ImportMaterial 批量导入一份判例材料：按段落切片顺序生成判决段。
func (s *Service) ImportMaterial(ctx context.Context, batchID int64, sourceDoc string, texts []string) ([]*model.JudgmentSegment, error) {
	var out []*model.JudgmentSegment
	for i, t := range texts {
		seg, err := s.ImportSegment(ctx, batchID, sourceDoc, i+1, t)
		if err != nil {
			return nil, fmt.Errorf("import segment %d: %w", i, err)
		}
		out = append(out, seg)
	}
	return out, nil
}
