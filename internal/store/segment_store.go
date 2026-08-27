package store

import (
	"context"
	"database/sql"
	"fmt"

	"task281-citereview/internal/model"
)

// CreateSegment 创建判决段。SummaryHash 用于导入幂等（同批同哈希视为重复）。
func (s *Store) CreateSegment(ctx context.Context, seg *model.JudgmentSegment) (*model.JudgmentSegment, error) {
	now := Now()
	res, err := s.DB.ExecContext(ctx,
		`INSERT INTO judgment_segment (batch_id, source_doc, seq_no, text, status, summary_hash, created_at)
		 VALUES (?, ?, ?, ?, 'pending_parse', ?, ?)`,
		seg.BatchID, seg.SourceDoc, seg.SeqNo, seg.Text, seg.SummaryHash, now)
	if err != nil {
		return nil, fmt.Errorf("insert segment: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("segment last id: %w", err)
	}
	return s.GetSegment(ctx, id)
}

// GetSegment 按 ID 读取判决段。
func (s *Store) GetSegment(ctx context.Context, id int64) (*model.JudgmentSegment, error) {
	row := s.DB.QueryRowContext(ctx,
		`SELECT id, batch_id, source_doc, seq_no, text, status, summary_hash, created_at
		 FROM judgment_segment WHERE id = ?`, id)
	seg := &model.JudgmentSegment{}
	if err := row.Scan(&seg.ID, &seg.BatchID, &seg.SourceDoc, &seg.SeqNo, &seg.Text, &seg.Status, &seg.SummaryHash, &seg.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrSegmentNotFound
		}
		return nil, fmt.Errorf("scan segment: %w", err)
	}
	return seg, nil
}

// GetSegmentByHash 按批次与摘要哈希查找已存在段落，实现导入幂等。
func (s *Store) GetSegmentByHash(ctx context.Context, batchID int64, hash string) (*model.JudgmentSegment, error) {
	row := s.DB.QueryRowContext(ctx,
		`SELECT id, batch_id, source_doc, seq_no, text, status, summary_hash, created_at
		 FROM judgment_segment WHERE batch_id = ? AND summary_hash = ? LIMIT 1`, batchID, hash)
	seg := &model.JudgmentSegment{}
	if err := row.Scan(&seg.ID, &seg.BatchID, &seg.SourceDoc, &seg.SeqNo, &seg.Text, &seg.Status, &seg.SummaryHash, &seg.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrSegmentNotFound
		}
		return nil, fmt.Errorf("scan segment by hash: %w", err)
	}
	return seg, nil
}

// ListSegments 返回某批次下全部判决段（按 seq_no 升序）。
func (s *Store) ListSegments(ctx context.Context, batchID int64) ([]*model.JudgmentSegment, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, batch_id, source_doc, seq_no, text, status, summary_hash, created_at
		 FROM judgment_segment WHERE batch_id = ? ORDER BY seq_no ASC, id ASC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("query segments: %w", err)
	}
	defer rows.Close()
	var out []*model.JudgmentSegment
	for rows.Next() {
		seg := &model.JudgmentSegment{}
		if err := rows.Scan(&seg.ID, &seg.BatchID, &seg.SourceDoc, &seg.SeqNo, &seg.Text, &seg.Status, &seg.SummaryHash, &seg.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan segment row: %w", err)
		}
		out = append(out, seg)
	}
	return out, nil
}

// SetSegmentStatus 按状态机校验后更新判决段状态。
func (s *Store) SetSegmentStatus(ctx context.Context, id int64, to model.SegmentStatus) error {
	seg, err := s.GetSegment(ctx, id)
	if err != nil {
		return err
	}
	if !model.ValidSegmentTransition(seg.Status, to) {
		return fmt.Errorf("%w: %s -> %s", model.ErrInvalidTransition, seg.Status, to)
	}
	if _, err := s.DB.ExecContext(ctx,
		`UPDATE judgment_segment SET status = ? WHERE id = ?`, to, id); err != nil {
		return fmt.Errorf("update segment status: %w", err)
	}
	return nil
}
