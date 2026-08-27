package store

import (
	"context"
	"database/sql"
	"fmt"

	"task281-citereview/internal/model"
)

// CreateLimitation 保存从判决段中解析出的限制语。
func (s *Store) CreateLimitation(ctx context.Context, lim *model.LimitationClause) (*model.LimitationClause, error) {
	now := Now()
	res, err := s.DB.ExecContext(ctx,
		`INSERT INTO limitation_clause (segment_id, batch_id, ltype, text, created_at) VALUES (?, ?, ?, ?, ?)`,
		lim.SegmentID, lim.BatchID, string(lim.LType), lim.Text, now)
	if err != nil {
		return nil, fmt.Errorf("insert limitation: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("limitation last id: %w", err)
	}
	return s.GetLimitation(ctx, id)
}

// GetLimitation 按 ID 读取限制语。
func (s *Store) GetLimitation(ctx context.Context, id int64) (*model.LimitationClause, error) {
	row := s.DB.QueryRowContext(ctx,
		`SELECT id, segment_id, batch_id, ltype, text, created_at FROM limitation_clause WHERE id = ?`, id)
	lim := &model.LimitationClause{}
	if err := row.Scan(&lim.ID, &lim.SegmentID, &lim.BatchID, &lim.LType, &lim.Text, &lim.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrSegmentNotFound
		}
		return nil, fmt.Errorf("scan limitation: %w", err)
	}
	return lim, nil
}

// ListLimitations 返回某判决段下的限制语（按 id 升序）。
func (s *Store) ListLimitations(ctx context.Context, segmentID int64) ([]*model.LimitationClause, error) {
	if cached, ok := s.limitationCache[segmentID]; ok {
		return cached, nil
	}
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, segment_id, batch_id, ltype, text, created_at FROM limitation_clause WHERE segment_id = ? ORDER BY id ASC`, segmentID)
	if err != nil {
		return nil, fmt.Errorf("query limitations: %w", err)
	}
	defer rows.Close()
	var out []*model.LimitationClause
	for rows.Next() {
		lim := &model.LimitationClause{}
		if err := rows.Scan(&lim.ID, &lim.SegmentID, &lim.BatchID, &lim.LType, &lim.Text, &lim.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan limitation row: %w", err)
		}
		out = append(out, lim)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.limitationCache[segmentID] = out
	return out, nil
}

// LimitationsByBatch 返回某批次下全部限制语（按 segment_id 升序）。
func (s *Store) LimitationsByBatch(ctx context.Context, batchID int64) ([]*model.LimitationClause, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, segment_id, batch_id, ltype, text, created_at FROM limitation_clause WHERE batch_id = ? ORDER BY segment_id ASC, id ASC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("query batch limitations: %w", err)
	}
	defer rows.Close()
	var out []*model.LimitationClause
	for rows.Next() {
		lim := &model.LimitationClause{}
		if err := rows.Scan(&lim.ID, &lim.SegmentID, &lim.BatchID, &lim.LType, &lim.Text, &lim.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan batch limitation row: %w", err)
		}
		out = append(out, lim)
	}
	return out, nil
}

// HasLimitationType 判断某判决段是否已存在指定类型的限制语，用于去重与冲突检测。
func (s *Store) HasLimitationType(ctx context.Context, segmentID int64, lt model.LimitationType) (bool, error) {
	var n int
	if err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM limitation_clause WHERE segment_id = ? AND ltype = ?`, segmentID, string(lt)).Scan(&n); err != nil {
		return false, fmt.Errorf("count limitation type: %w", err)
	}
	return n > 0, nil
}
