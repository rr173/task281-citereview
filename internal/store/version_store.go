package store

import (
	"context"
	"database/sql"
	"fmt"

	"task281-citereview/internal/model"
)

// CreateVersion 为某批次创建研究图版本，version_no 按批次内自增。
func (s *Store) CreateVersion(ctx context.Context, batchID int64, snapshot string) (*model.ResearchGraphVersion, error) {
	var maxNo int
	if err := s.DB.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version_no), 0) FROM research_graph_version WHERE batch_id = ?`, batchID).Scan(&maxNo); err != nil {
		return nil, fmt.Errorf("max version no: %w", err)
	}
	now := Now()
	res, err := s.DB.ExecContext(ctx,
		`INSERT INTO research_graph_version (batch_id, version_no, status, material_snapshot, created_at)
		 VALUES (?, ?, 'draft', ?, ?)`, batchID, maxNo+1, snapshot, now)
	if err != nil {
		return nil, fmt.Errorf("insert version: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("version last id: %w", err)
	}
	return s.GetVersion(ctx, id)
}

// GetVersion 按 ID 读取研究图版本。
func (s *Store) GetVersion(ctx context.Context, id int64) (*model.ResearchGraphVersion, error) {
	row := s.DB.QueryRowContext(ctx,
		`SELECT id, batch_id, version_no, status, material_snapshot, created_at FROM research_graph_version WHERE id = ?`, id)
	v := &model.ResearchGraphVersion{}
	if err := row.Scan(&v.ID, &v.BatchID, &v.VersionNo, &v.Status, &v.MaterialSnapshot, &v.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrVersionNotFound
		}
		return nil, fmt.Errorf("scan version: %w", err)
	}
	return v, nil
}

// LatestVersion 返回某批次最新创建的研究图版本（按 version_no 倒序）。
func (s *Store) LatestVersion(ctx context.Context, batchID int64) (*model.ResearchGraphVersion, error) {
	row := s.DB.QueryRowContext(ctx,
		`SELECT id, batch_id, version_no, status, material_snapshot, created_at
		 FROM research_graph_version WHERE batch_id = ? ORDER BY version_no DESC LIMIT 1`, batchID)
	v := &model.ResearchGraphVersion{}
	if err := row.Scan(&v.ID, &v.BatchID, &v.VersionNo, &v.Status, &v.MaterialSnapshot, &v.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrVersionNotFound
		}
		return nil, fmt.Errorf("scan latest version: %w", err)
	}
	return v, nil
}

// ListVersions 返回某批次下全部研究图版本（按 version_no 升序）。
func (s *Store) ListVersions(ctx context.Context, batchID int64) ([]*model.ResearchGraphVersion, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, batch_id, version_no, status, material_snapshot, created_at
		 FROM research_graph_version WHERE batch_id = ? ORDER BY version_no ASC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("query versions: %w", err)
	}
	defer rows.Close()
	var out []*model.ResearchGraphVersion
	for rows.Next() {
		v := &model.ResearchGraphVersion{}
		if err := rows.Scan(&v.ID, &v.BatchID, &v.VersionNo, &v.Status, &v.MaterialSnapshot, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan version row: %w", err)
		}
		out = append(out, v)
	}
	return out, nil
}

// SetVersionStatus 按状态机校验后更新研究图版本状态。
func (s *Store) SetVersionStatus(ctx context.Context, id int64, to model.GraphVersionStatus) error {
	v, err := s.GetVersion(ctx, id)
	if err != nil {
		return err
	}
	if !model.ValidGraphVersionTransition(v.Status, to) {
		return fmt.Errorf("%w: %s -> %s", model.ErrInvalidTransition, v.Status, to)
	}
	if _, err := s.DB.ExecContext(ctx,
		`UPDATE research_graph_version SET status = ? WHERE id = ?`, to, id); err != nil {
		return fmt.Errorf("update version status: %w", err)
	}
	return nil
}
