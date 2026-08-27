package store

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Store 持有数据库连接，提供全部持久化能力。所有 store 子文件的方法均挂在 *Store 上。
type Store struct {
	DB     *sql.DB
	edgeMu sync.Mutex
	// segMu 为每个判决段维护一把独立互斥锁，使同一段的解析（限制语/要素重建）
	// 串行执行，避免并发清空与插入交错导致条数错乱。不同段之间互不阻塞。
	segMu sync.Map // segmentID int64 -> *sync.Mutex
}

// LockSegment 返回某判决段专属的互斥锁。同一 segmentID 始终返回同一把锁，
// 因此同一段的解析串行化；不同段返回各自独立的锁，可并发进行。
func (s *Store) LockSegment(segmentID int64) *sync.Mutex {
	v, _ := s.segMu.LoadOrStore(segmentID, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// Open 打开（必要时创建）SQLite 数据库并建立全部表结构。
// 使用 WAL 日志与忙等待超时，保证并发写入与重启恢复的正确性。
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(32)
	db.SetMaxIdleConns(32)
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{DB: db}, nil
}

// Close 关闭数据库连接。
func (s *Store) Close() error {
	return s.DB.Close()
}

// Now 返回当前 Unix 毫秒时间戳，作为全表统一时间口径。
func Now() int64 {
	return time.Now().UnixMilli()
}

// migrate 创建全部表。全部使用 IF NOT EXISTS，可重复安全执行。
func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS research_batch (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			code TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'organizing',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS judgment_segment (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL,
			source_doc TEXT NOT NULL DEFAULT '',
			seq_no INTEGER NOT NULL DEFAULT 0,
			text TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending_parse',
			summary_hash TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS factual_element (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			segment_id INTEGER NOT NULL,
			batch_id INTEGER NOT NULL,
			key TEXT NOT NULL DEFAULT '',
			value TEXT NOT NULL DEFAULT '',
			element_type TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS limitation_clause (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			segment_id INTEGER NOT NULL,
			batch_id INTEGER NOT NULL,
			ltype TEXT NOT NULL DEFAULT '',
			text TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS citation_edge (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL,
			citing_segment_id INTEGER NOT NULL,
			cited_segment_id INTEGER NOT NULL,
			relation TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'candidate',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS decision (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL,
			edge_id INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'candidate',
			distinction_reason TEXT NOT NULL DEFAULT '',
			graph_version_id INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS research_graph_version (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL,
			version_no INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'draft',
			material_snapshot TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_segment_batch ON judgment_segment(batch_id);`,
		`CREATE INDEX IF NOT EXISTS idx_element_segment ON factual_element(segment_id);`,
		`CREATE INDEX IF NOT EXISTS idx_limitation_segment ON limitation_clause(segment_id);`,
		`CREATE INDEX IF NOT EXISTS idx_edge_batch ON citation_edge(batch_id);`,
		`CREATE INDEX IF NOT EXISTS idx_decision_edge ON decision(edge_id);`,
		`CREATE INDEX IF NOT EXISTS idx_version_batch ON research_graph_version(batch_id);`,
	}
	for _, st := range stmts {
		if _, err := db.Exec(st); err != nil {
			return fmt.Errorf("exec migration: %w", err)
		}
	}
	return nil
}
