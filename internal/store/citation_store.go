package store

import (
	"context"
	"database/sql"
	"fmt"

	"task281-citereview/internal/model"
)

// CreateEdge 创建引证关系。在写入前强制三项不变量：
//  1. 拒绝自引（citing == cited）；
//  2. 拒绝同一对段落重复引证；
//  3. 拒绝会构成引证环的边（cited 经既有边可达 citing）。
//
// 上述不变量均为"先检测再写入"的批次内约束：若不串行化，并发建边会同时通过检测
// 再各自 INSERT，导致重复对与引证环。故整个"重复检测 + 环检测 + 写入"必须在
// 该批次的互斥锁内原子完成。
func (s *Store) CreateEdge(ctx context.Context, edge *model.CitationEdge) (*model.CitationEdge, error) {
	if edge.CitingSegmentID == edge.CitedSegmentID {
		return nil, model.ErrSelfCitation
	}
	mu := s.edgeMutex(edge.BatchID)
	mu.Lock()
	defer mu.Unlock()
	// 重复对检测
	var dup int
	if err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM citation_edge WHERE batch_id = ? AND citing_segment_id = ? AND cited_segment_id = ?`,
		edge.BatchID, edge.CitingSegmentID, edge.CitedSegmentID).Scan(&dup); err != nil {
		return nil, fmt.Errorf("dup edge check: %w", err)
	}
	if dup > 0 {
		return nil, model.ErrDuplicateEdge
	}
	// 引证环检测：新增 citing→cited（newFrom→newTo）会成环，当且仅当
	// 既有图中已存在 newTo(=cited) 到 newFrom(=citing) 的路径，即 cited 已可达 citing。
	reachable, err := s.reachable(ctx, edge.BatchID, edge.CitedSegmentID, edge.CitingSegmentID)
	if err != nil {
		return nil, err
	}
	if reachable {
		return nil, model.ErrCitationCycle
	}
	now := Now()
	res, err := s.DB.ExecContext(ctx,
		`INSERT INTO citation_edge (batch_id, citing_segment_id, cited_segment_id, relation, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 'candidate', ?, ?)`,
		edge.BatchID, edge.CitingSegmentID, edge.CitedSegmentID, edge.Relation, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert edge: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("edge last id: %w", err)
	}
	return s.GetEdge(ctx, id)
}

// GetEdge 按 ID 读取引证关系。
func (s *Store) GetEdge(ctx context.Context, id int64) (*model.CitationEdge, error) {
	row := s.DB.QueryRowContext(ctx,
		`SELECT id, batch_id, citing_segment_id, cited_segment_id, relation, status, created_at, updated_at
		 FROM citation_edge WHERE id = ?`, id)
	e := &model.CitationEdge{}
	if err := row.Scan(&e.ID, &e.BatchID, &e.CitingSegmentID, &e.CitedSegmentID, &e.Relation, &e.Status, &e.CreatedAt, &e.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrEdgeNotFound
		}
		return nil, fmt.Errorf("scan edge: %w", err)
	}
	return e, nil
}

// ListEdges 返回某批次下全部引证关系（按 id 升序）。
func (s *Store) ListEdges(ctx context.Context, batchID int64) ([]*model.CitationEdge, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, batch_id, citing_segment_id, cited_segment_id, relation, status, created_at, updated_at
		 FROM citation_edge WHERE batch_id = ? ORDER BY id ASC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("query edges: %w", err)
	}
	defer rows.Close()
	var out []*model.CitationEdge
	for rows.Next() {
		e := &model.CitationEdge{}
		if err := rows.Scan(&e.ID, &e.BatchID, &e.CitingSegmentID, &e.CitedSegmentID, &e.Relation, &e.Status, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan edge row: %w", err)
		}
		out = append(out, e)
	}
	return out, nil
}

// SetEdgeStatus 更新引证关系状态（范围检查/裁决结果回写）。
func (s *Store) SetEdgeStatus(ctx context.Context, id int64, status model.CitationStatus) error {
	if _, err := s.DB.ExecContext(ctx,
		`UPDATE citation_edge SET status = ?, updated_at = ? WHERE id = ?`, status, Now(), id); err != nil {
		return fmt.Errorf("update edge status: %w", err)
	}
	return nil
}

// reachable 在 batch 的引证有向图（citing→cited 为边方向）上，判断 from 是否可达 to。
// 边方向为 citing 引用 cited，因此"可达"意味着 from 的引用链能一路指向 to。
func (s *Store) reachable(ctx context.Context, batchID, from, to int64) (bool, error) {
	if from == to {
		return true, nil
	}
	edges, err := s.ListEdges(ctx, batchID)
	if err != nil {
		return false, err
	}
	adj := map[int64][]int64{}
	for _, e := range edges {
		adj[e.CitingSegmentID] = append(adj[e.CitingSegmentID], e.CitedSegmentID)
	}
	visited := map[int64]bool{}
	stack := []int64{from}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if cur == to {
			return true, nil
		}
		if visited[cur] {
			continue
		}
		visited[cur] = true
		for _, nxt := range adj[cur] {
			if !visited[nxt] {
				stack = append(stack, nxt)
			}
		}
	}
	return false, nil
}

// Reachable 暴露引证图可达性判定，供测试与诊断使用。
func (s *Store) Reachable(ctx context.Context, batchID, from, to int64) (bool, error) {
	return s.reachable(ctx, batchID, from, to)
}
