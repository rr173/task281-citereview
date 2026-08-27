package store

import (
	"context"
	"database/sql"
	"fmt"

	"task281-citereview/internal/model"
)

// CreateBatch 创建研究批次，初始状态为整理中（organizing）。
func (s *Store) CreateBatch(ctx context.Context, code, title string) (*model.ResearchBatch, error) {
	now := Now()
	res, err := s.DB.ExecContext(ctx,
		`INSERT INTO research_batch (code, title, status, created_at, updated_at) VALUES (?, ?, 'organizing', ?, ?)`,
		code, title, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert batch: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("batch last id: %w", err)
	}
	return s.GetBatch(ctx, id)
}

// GetBatch 按 ID 读取研究批次。
func (s *Store) GetBatch(ctx context.Context, id int64) (*model.ResearchBatch, error) {
	row := s.DB.QueryRowContext(ctx,
		`SELECT id, code, title, status, created_at, updated_at FROM research_batch WHERE id = ?`, id)
	b := &model.ResearchBatch{}
	if err := row.Scan(&b.ID, &b.Code, &b.Title, &b.Status, &b.CreatedAt, &b.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrBatchNotFound
		}
		return nil, fmt.Errorf("scan batch: %w", err)
	}
	return b, nil
}

// ListBatches 返回全部研究批次（按创建时间倒序）。
func (s *Store) ListBatches(ctx context.Context) ([]*model.ResearchBatch, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, code, title, status, created_at, updated_at FROM research_batch ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query batches: %w", err)
	}
	defer rows.Close()
	var out []*model.ResearchBatch
	for rows.Next() {
		b := &model.ResearchBatch{}
		if err := rows.Scan(&b.ID, &b.Code, &b.Title, &b.Status, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan batch row: %w", err)
		}
		out = append(out, b)
	}
	return out, nil
}

// SetBatchStatus 按状态机校验后更新研究批次状态。
func (s *Store) SetBatchStatus(ctx context.Context, id int64, to model.BatchStatus) error {
	b, err := s.GetBatch(ctx, id)
	if err != nil {
		return err
	}
	if !model.ValidBatchTransition(b.Status, to) {
		return fmt.Errorf("invalid transition: %v", model.ErrInvalidTransition)
	}
	if _, err := s.DB.ExecContext(ctx,
		`UPDATE research_batch SET status = ?, updated_at = ? WHERE id = ?`, to, Now(), id); err != nil {
		return fmt.Errorf("update batch status: %w", err)
	}
	return nil
}

// CountSegments 统计某批次下判决段数量，供统计 API 使用。
func (s *Store) CountSegments(ctx context.Context, batchID int64) (int, error) {
	var n int
	if err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM judgment_segment WHERE batch_id = ?`, batchID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count segments: %w", err)
	}
	return n, nil
}
