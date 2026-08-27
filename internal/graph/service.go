package graph

import (
	"context"
	"encoding/json"
	"fmt"

	"task281-citereview/internal/model"
	"task281-citereview/internal/store"
)

// Service 研究图模块：把批次内的段落、要素、限制语、引证与裁决聚合为版本化快照，
// 支持草稿、共享、冻结、替代等状态流转，并向 Web 复核页提供聚合视图。
type Service struct {
	Store *store.Store
}

// New 构造研究图模块服务。
func New(st *store.Store) *Service { return &Service{Store: st} }

// BuildSnapshot 将批次当前全部材料聚合成 JSON 快照字符串。
func (s *Service) BuildSnapshot(ctx context.Context, batchID int64) (string, error) {
	segs, err := s.Store.ListSegments(ctx, batchID)
	if err != nil {
		return "", err
	}
	elems, err := s.Store.ElementsByBatch(ctx, batchID)
	if err != nil {
		return "", err
	}
	lims, err := s.Store.LimitationsByBatch(ctx, batchID)
	if err != nil {
		return "", err
	}
	edges, err := s.Store.ListEdges(ctx, batchID)
	if err != nil {
		return "", err
	}
	decs, err := s.Store.ListDecisions(ctx, batchID)
	if err != nil {
		return "", err
	}
	snap := map[string]interface{}{
		"batch_id":  batchID,
		"segments":  segs,
		"elements":  elems,
		"limits":    lims,
		"edges":     edges,
		"decisions": decs,
	}
	b, err := json.Marshal(snap)
	if err != nil {
		return "", fmt.Errorf("marshal snapshot: %w", err)
	}
	return string(b), nil
}

// CreateDraft 创建研究图草稿版本（快照当前材料）。
func (s *Service) CreateDraft(ctx context.Context, batchID int64) (*model.ResearchGraphVersion, error) {
	snap, err := s.BuildSnapshot(ctx, batchID)
	if err != nil {
		return nil, err
	}
	return s.Store.CreateVersion(ctx, batchID, snap)
}

// Freeze 冻结研究图版本（快照定型，进入只读）。
func (s *Service) Freeze(ctx context.Context, versionID int64) error {
	return s.Store.SetVersionStatus(ctx, versionID, model.GVFrozen)
}

// Share 将草稿版本转为共享状态。
func (s *Service) Share(ctx context.Context, versionID int64) error {
	return s.Store.SetVersionStatus(ctx, versionID, model.GVShared)
}

// Supersede 将某版本标记为替代，并基于当前材料创建新的草稿版本。
func (s *Service) Supersede(ctx context.Context, versionID int64) (*model.ResearchGraphVersion, error) {
	if err := s.Store.SetVersionStatus(ctx, versionID, model.GVSuperseded); err != nil {
		return nil, err
	}
	ver, err := s.Store.GetVersion(ctx, versionID)
	if err != nil {
		return nil, err
	}
	return s.CreateDraft(ctx, ver.BatchID)
}

// BuildView 组装引证图聚合视图，供 Web 复核页与 /api/graph 使用。
func (s *Service) BuildView(ctx context.Context, batchID int64) (*model.GraphView, error) {
	batch, err := s.Store.GetBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	segs, err := s.Store.ListSegments(ctx, batchID)
	if err != nil {
		return nil, err
	}
	elems, err := s.Store.ElementsByBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	lims, err := s.Store.LimitationsByBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	edges, err := s.Store.ListEdges(ctx, batchID)
	if err != nil {
		return nil, err
	}
	decs, err := s.Store.ListDecisions(ctx, batchID)
	if err != nil {
		return nil, err
	}
	ver, err := s.Store.LatestVersion(ctx, batchID)
	if err != nil {
		// 尚未创建版本时返回空视图，不视为错误
		if err == model.ErrVersionNotFound {
			ver = nil
		} else {
			return nil, err
		}
	}
	return &model.GraphView{
		Batch:       batch,
		Segments:    segs,
		Elements:    elems,
		Limitations: lims,
		Edges:       edges,
		Decisions:   decs,
		Version:     ver,
	}, nil
}
