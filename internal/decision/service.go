package decision

import (
	"context"
	"fmt"

	"task281-citereview/internal/model"
	"task281-citereview/internal/store"
)

// Service 裁决模块：记录研究者对引证关系的适用范围结论与区分理由，并回写引证状态。
type Service struct {
	Store *store.Store
}

// New 构造裁决模块服务。
func New(st *store.Store) *Service { return &Service{Store: st} }

// Decide 对一条引证关系作出裁决。约束：
//   - 引用的研究图版本必须存在且未被冻结（冻结版本不可修改）；
//   - 若裁决为"适用/确认"，要求被引方与引用方的判决段均已抽取事实要素（事实要素缺失则拒绝）；
//   - 裁决状态回写引证关系，使图保持最新结论。
func (s *Service) Decide(ctx context.Context, edgeID int64, status model.CitationStatus, reason string, graphVersionID int64) (*model.Decision, error) {
	edge, err := s.Store.GetEdge(ctx, edgeID)
	if err != nil {
		return nil, err
	}
	ver, err := s.Store.GetVersion(ctx, graphVersionID)
	if err != nil {
		return nil, fmt.Errorf("decision: get version: %v", err)
	}
	if ver.Status == model.GVFrozen || ver.Status == model.GVSuperseded {
		return nil, fmt.Errorf("%w: version %d is %s", model.ErrFrozenImmutable, graphVersionID, ver.Status)
	}
	if status == model.CiteApplicable || status == model.CiteConfirmed {
		citedN, err := s.Store.CountElements(ctx, edge.CitedSegmentID)
		if err != nil {
			return nil, err
		}
		citingN, err := s.Store.CountElements(ctx, edge.CitingSegmentID)
		if err != nil {
			return nil, err
		}
		if citedN == 0 || citingN == 0 {
			return nil, model.ErrElementMissing
		}
	}
	dec := &model.Decision{
		BatchID:           edge.BatchID,
		EdgeID:            edgeID,
		Status:            status,
		DistinctionReason: reason,
		GraphVersionID:    graphVersionID,
	}
	created, err := s.Store.CreateDecision(ctx, dec)
	if err != nil {
		return nil, err
	}
	// 裁决结论回写引证关系状态
	if err := s.Store.SetEdgeStatus(ctx, edgeID, status); err != nil {
		return nil, err
	}
	return created, nil
}

// Latest 返回某引证关系的最新裁决。
func (s *Service) Latest(ctx context.Context, edgeID int64) (*model.Decision, error) {
	return s.Store.LatestDecision(ctx, edgeID)
}
