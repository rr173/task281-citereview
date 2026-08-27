package store

import (
	"context"
	"database/sql"
	"fmt"

	"task281-citereview/internal/model"
)

// CreateElement 保存从判决段中抽取的事实要素。
func (s *Store) CreateElement(ctx context.Context, el *model.FactualElement) (*model.FactualElement, error) {
	now := Now()
	res, err := s.DB.ExecContext(ctx,
		`INSERT INTO factual_element (segment_id, batch_id, key, value, element_type, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		el.SegmentID, el.BatchID, el.Key, el.Value, el.ElementType, now)
	if err != nil {
		return nil, fmt.Errorf("insert element: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("element last id: %w", err)
	}
	return s.GetElement(ctx, id)
}

// GetElement 按 ID 读取事实要素。
func (s *Store) GetElement(ctx context.Context, id int64) (*model.FactualElement, error) {
	row := s.DB.QueryRowContext(ctx,
		`SELECT id, segment_id, batch_id, key, value, element_type, created_at FROM factual_element WHERE id = ?`, id)
	el := &model.FactualElement{}
	if err := row.Scan(&el.ID, &el.SegmentID, &el.BatchID, &el.Key, &el.Value, &el.ElementType, &el.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrSegmentNotFound
		}
		return nil, fmt.Errorf("scan element: %w", err)
	}
	return el, nil
}

// ListElements 返回某判决段下的事实要素（按 id 升序）。
func (s *Store) ListElements(ctx context.Context, segmentID int64) ([]*model.FactualElement, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, segment_id, batch_id, key, value, element_type, created_at
		 FROM factual_element WHERE segment_id = ? ORDER BY id ASC`, segmentID)
	if err != nil {
		return nil, fmt.Errorf("query elements: %w", err)
	}
	defer rows.Close()
	var out []*model.FactualElement
	for rows.Next() {
		el := &model.FactualElement{}
		if err := rows.Scan(&el.ID, &el.SegmentID, &el.BatchID, &el.Key, &el.Value, &el.ElementType, &el.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan element row: %w", err)
		}
		out = append(out, el)
	}
	return out, nil
}

// ElementsByBatch 返回某批次下全部事实要素（按 segment_id 升序）。
func (s *Store) ElementsByBatch(ctx context.Context, batchID int64) ([]*model.FactualElement, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, segment_id, batch_id, key, value, element_type, created_at
		 FROM factual_element WHERE batch_id = ? ORDER BY segment_id ASC, id ASC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("query batch elements: %w", err)
	}
	defer rows.Close()
	var out []*model.FactualElement
	for rows.Next() {
		el := &model.FactualElement{}
		if err := rows.Scan(&el.ID, &el.SegmentID, &el.BatchID, &el.Key, &el.Value, &el.ElementType, &el.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan batch element row: %w", err)
		}
		out = append(out, el)
	}
	return out, nil
}

// CountElements 统计某判决段事实要素数量。
func (s *Store) CountElements(ctx context.Context, segmentID int64) (int, error) {
	var n int
	if err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM factual_element WHERE segment_id = ?`, segmentID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count elements: %w", err)
	}
	return n, nil
}
