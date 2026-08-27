package store

import (
	"context"
	"database/sql"
	"fmt"

	"task281-citereview/internal/model"
)

// CreateDecision 保存对一条引证关系的裁决（含区分理由与版本引用）。
func (s *Store) CreateDecision(ctx context.Context, d *model.Decision) (*model.Decision, error) {
	now := Now()
	res, err := s.DB.ExecContext(ctx,
		`INSERT INTO decision (batch_id, edge_id, status, distinction_reason, graph_version_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		d.BatchID, d.EdgeID, string(d.Status), d.DistinctionReason, d.GraphVersionID, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert decision: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("decision last id: %w", err)
	}
	return s.GetDecisionByID(ctx, id)
}

// GetDecisionByID 按 ID 读取裁决。
func (s *Store) GetDecisionByID(ctx context.Context, id int64) (*model.Decision, error) {
	row := s.DB.QueryRowContext(ctx,
		`SELECT id, batch_id, edge_id, status, distinction_reason, graph_version_id, created_at, updated_at
		 FROM decision WHERE id = ?`, id)
	d := &model.Decision{}
	if err := row.Scan(&d.ID, &d.BatchID, &d.EdgeID, &d.Status, &d.DistinctionReason, &d.GraphVersionID, &d.CreatedAt, &d.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrDecisionNotFound
		}
		return nil, fmt.Errorf("scan decision: %w", err)
	}
	return d, nil
}

// LatestDecision 返回某引证关系的最新一条裁决（按 id 倒序）。
func (s *Store) LatestDecision(ctx context.Context, edgeID int64) (*model.Decision, error) {
	row := s.DB.QueryRowContext(ctx,
		`SELECT id, batch_id, edge_id, status, distinction_reason, graph_version_id, created_at, updated_at
		 FROM decision WHERE edge_id = ? ORDER BY id DESC LIMIT 1`, edgeID)
	d := &model.Decision{}
	if err := row.Scan(&d.ID, &d.BatchID, &d.EdgeID, &d.Status, &d.DistinctionReason, &d.GraphVersionID, &d.CreatedAt, &d.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrDecisionNotFound
		}
		return nil, fmt.Errorf("scan latest decision: %w", err)
	}
	return d, nil
}

// ListDecisions 返回某批次下全部裁决（按 id 升序）。
func (s *Store) ListDecisions(ctx context.Context, batchID int64) ([]*model.Decision, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, batch_id, edge_id, status, distinction_reason, graph_version_id, created_at, updated_at
		 FROM decision WHERE batch_id = ? ORDER BY id ASC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("query decisions: %w", err)
	}
	defer rows.Close()
	var out []*model.Decision
	for rows.Next() {
		d := &model.Decision{}
		if err := rows.Scan(&d.ID, &d.BatchID, &d.EdgeID, &d.Status, &d.DistinctionReason, &d.GraphVersionID, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan decision row: %w", err)
		}
		out = append(out, d)
	}
	return out, nil
}
