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

// ReplaceLimitations 原子地重建某判决段的限制语：先清空该段既有限制语，再批量插入新集合。
// 该操作在段级互斥锁保护下、于单一数据库事务内完成：
//   - 段级锁串行化对同一段的并发解析，避免"清空/插入"交错导致条数错乱或重复；
//   - 事务保证清空与插入要么整体提交、要么整体回滚，并发读取方永远不会撞见
//     "已清空尚未插入"的中间空列表。
// 传入空切片等价于清空该段全部限制语。
func (s *Store) ReplaceLimitations(ctx context.Context, segmentID int64, lims []*model.LimitationClause) error {
	mu := s.LockSegment(segmentID)
	mu.Lock()
	defer mu.Unlock()

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM limitation_clause WHERE segment_id = ?`, segmentID); err != nil {
		return fmt.Errorf("clear limitations: %w", err)
	}
	if len(lims) > 0 {
		stmt, err := tx.PrepareContext(ctx,
			`INSERT INTO limitation_clause (segment_id, batch_id, ltype, text, created_at) VALUES (?, ?, ?, ?, ?)`)
		if err != nil {
			return fmt.Errorf("prepare insert limitation: %w", err)
		}
		defer stmt.Close()
		now := Now()
		for _, lim := range lims {
			if _, err := stmt.ExecContext(ctx, segmentID, lim.BatchID, string(lim.LType), lim.Text, now); err != nil {
				return fmt.Errorf("insert limitation: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit replace limitations: %w", err)
	}
	return nil
}
