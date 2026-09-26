package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wolvever/litefaas/internal/types"

	_ "modernc.org/sqlite"
)

var (
	ErrNotFound = errors.New("not found")
	ErrExists   = errors.New("already exists")
)

// Store is the sqlite control-plane database.
type Store struct {
	db *sql.DB
}

// Open creates dataDir if needed and opens litefaas.db.
func Open(dataDir string) (*Store, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("data-dir is required")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data-dir: %w", err)
	}
	dbPath := filepath.Join(dataDir, "litefaas.db")
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)", filepath.ToSlash(dbPath))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pragma foreign_keys: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS resources (
	name TEXT PRIMARY KEY,
	kind TEXT NOT NULL,
	runtime TEXT NOT NULL,
	image TEXT NOT NULL DEFAULT '',
	port INTEGER NOT NULL DEFAULT 8080,
	memory INTEGER NOT NULL DEFAULT 128,
	timeout TEXT NOT NULL DEFAULT '',
	health TEXT NOT NULL DEFAULT '/healthz',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS revisions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	image TEXT NOT NULL,
	status TEXT NOT NULL,
	created_at TEXT NOT NULL,
	FOREIGN KEY(name) REFERENCES resources(name) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS revisions_name_idx ON revisions(name);
`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// Close releases the database handle.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// List returns all resources without revision history.
func (s *Store) List() ([]types.Resource, error) {
	rows, err := s.db.Query(`
		SELECT name, kind, runtime, image, port, memory, timeout, health, created_at, updated_at
		FROM resources
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.Resource
	for rows.Next() {
		r, err := scanResource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []types.Resource{}
	}
	return out, nil
}

// Get returns one resource and its revisions.
func (s *Store) Get(name string) (types.Resource, error) {
	row := s.db.QueryRow(`
		SELECT name, kind, runtime, image, port, memory, timeout, health, created_at, updated_at
		FROM resources
		WHERE name = ?
	`, name)
	r, err := scanResource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return types.Resource{}, ErrNotFound
	}
	if err != nil {
		return types.Resource{}, err
	}
	revs, err := s.listRevisions(name)
	if err != nil {
		return types.Resource{}, err
	}
	r.Revisions = revs
	return r, nil
}

// Create inserts a new resource. If Image is set, a registered revision is recorded.
func (s *Store) Create(r types.Resource) (types.Resource, error) {
	if err := types.NormalizeResource(&r); err != nil {
		return types.Resource{}, err
	}
	now := time.Now().UTC().Truncate(time.Second)
	r.CreatedAt = now
	r.UpdatedAt = now

	tx, err := s.db.Begin()
	if err != nil {
		return types.Resource{}, err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(`
		INSERT INTO resources (name, kind, runtime, image, port, memory, timeout, health, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, r.Name, r.Kind, r.Runtime, r.Image, r.Port, r.Memory, r.Timeout, r.Health, r.CreatedAt.Format(time.RFC3339), r.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		if isUnique(err) {
			return types.Resource{}, ErrExists
		}
		return types.Resource{}, err
	}
	if r.Image != "" {
		rev, err := insertRevision(tx, r.Name, r.Image, types.RevisionRegistered, now)
		if err != nil {
			return types.Resource{}, err
		}
		r.Revisions = []types.Revision{rev}
	}
	if err := tx.Commit(); err != nil {
		return types.Resource{}, err
	}
	return r, nil
}

// Delete removes a resource and its revisions.
func (s *Store) Delete(name string) error {
	res, err := s.db.Exec(`DELETE FROM resources WHERE name = ?`, name)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Deploy records a revision and updates the resource image (stub runner).
func (s *Store) Deploy(name, image string) (types.Revision, error) {
	image = strings.TrimSpace(image)
	if image == "" {
		return types.Revision{}, fmt.Errorf("image is required")
	}
	if _, err := s.Get(name); err != nil {
		return types.Revision{}, err
	}
	now := time.Now().UTC().Truncate(time.Second)
	tx, err := s.db.Begin()
	if err != nil {
		return types.Revision{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`UPDATE resources SET image = ?, updated_at = ? WHERE name = ?`, image, now.Format(time.RFC3339), name); err != nil {
		return types.Revision{}, err
	}
	rev, err := insertRevision(tx, name, image, types.RevisionRegistered, now)
	if err != nil {
		return types.Revision{}, err
	}
	if err := tx.Commit(); err != nil {
		return types.Revision{}, err
	}
	return rev, nil
}

func (s *Store) listRevisions(name string) ([]types.Revision, error) {
	rows, err := s.db.Query(`
		SELECT id, name, image, status, created_at
		FROM revisions
		WHERE name = ?
		ORDER BY id
	`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.Revision
	for rows.Next() {
		var rev types.Revision
		var created string
		if err := rows.Scan(&rev.ID, &rev.Name, &rev.Image, &rev.Status, &created); err != nil {
			return nil, err
		}
		rev.CreatedAt, err = time.Parse(time.RFC3339, created)
		if err != nil {
			return nil, err
		}
		out = append(out, rev)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []types.Revision{}
	}
	return out, nil
}

func insertRevision(tx *sql.Tx, name, image, status string, at time.Time) (types.Revision, error) {
	res, err := tx.Exec(`
		INSERT INTO revisions (name, image, status, created_at)
		VALUES (?, ?, ?, ?)
	`, name, image, status, at.Format(time.RFC3339))
	if err != nil {
		return types.Revision{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return types.Revision{}, err
	}
	return types.Revision{
		ID:        id,
		Name:      name,
		Image:     image,
		Status:    status,
		CreatedAt: at,
	}, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanResource(row rowScanner) (types.Resource, error) {
	var r types.Resource
	var created, updated string
	err := row.Scan(&r.Name, &r.Kind, &r.Runtime, &r.Image, &r.Port, &r.Memory, &r.Timeout, &r.Health, &created, &updated)
	if err != nil {
		return types.Resource{}, err
	}
	r.CreatedAt, err = time.Parse(time.RFC3339, created)
	if err != nil {
		return types.Resource{}, err
	}
	r.UpdatedAt, err = time.Parse(time.RFC3339, updated)
	if err != nil {
		return types.Resource{}, err
	}
	return r, nil
}

func isUnique(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
