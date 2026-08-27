package store

import (
	"context"
	"database/sql"
	"fmt"

	"task281-citereview/internal/model"
)

// CreateLimitation 保存从判决段中解析出的限制语。
// 缓存由 Store 独占维护：写入后使该判决段的限制语缓存失效，
// 保证后续读取必从数据库重新拉取，避免重新解析后列表仍返回旧限制语。
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
	s.invalidateLimitationCache(lim.SegmentID)
	return s.GetLimitation(ctx, id)
}

// DeleteLimitationsBySegment 删除某判决段下全部限制语，并同步使其缓存失效。
// 这是重新解析限制语前的重置入口，确保数据库与缓存一致：清空后任何读取
// 都会回源数据库，杜绝重新解析后仍命中陈旧缓存而返回旧限制语。
func (s *Store) DeleteLimitationsBySegment(ctx context.Context, segmentID int64) error {
	if _, err := s.DB.ExecContext(ctx,
		`DELETE FROM limitation_clause WHERE segment_id = ?`, segmentID); err != nil {
		return fmt.Errorf("delete limitations: %w", err)
	}
	s.invalidateLimitationCache(segmentID)
	return nil
}

// invalidateLimitationCache 清除指定判决段的限制语缓存条目。
// 在任何会改变该段限制语（删除/插入）的写路径之后调用，保证缓存不与数据库背离。
func (s *Store) invalidateLimitationCache(segmentID int64) {
	delete(s.limitationCache, segmentID)
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
