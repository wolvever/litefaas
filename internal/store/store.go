// Package store is the sqlite control-plane database (RFC-0001 §13).
package store

import (
	"database/sql"
	"encoding/json"
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

const filename = "litefaas.db"

// Store persists resources and revisions under a data directory.
type Store struct {
	db  *sql.DB
	dir string
}

func Open(dataDir string) (*Store, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("data-dir is required")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir data-dir: %w", err)
	}
	dsn := filepath.Join(dataDir, filename) + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, dir: dataDir}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Dir() string { return s.dir }

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS resources (
	name TEXT PRIMARY KEY,
	kind TEXT NOT NULL,
	runtime TEXT NOT NULL,
	spec_json TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS revisions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	image TEXT NOT NULL,
	status TEXT NOT NULL,
	created_at TEXT NOT NULL,
	FOREIGN KEY (name) REFERENCES resources(name) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS instances (
	name TEXT PRIMARY KEY,
	container_id TEXT,
	endpoint TEXT,
	image TEXT,
	status TEXT,
	FOREIGN KEY (name) REFERENCES resources(name) ON DELETE CASCADE
);
`)
	return err
}

func (s *Store) Update(r types.Resource) (types.Resource, error) {
	existing, err := s.Get(r.Name)
	if err != nil {
		return types.Resource{}, err
	}
	r.CreatedAt = existing.CreatedAt
	r.UpdatedAt = time.Now().UTC().Truncate(time.Second)
	raw, err := json.Marshal(r)
	if err != nil {
		return types.Resource{}, err
	}
	res, err := s.db.Exec(
		`UPDATE resources SET kind = ?, runtime = ?, spec_json = ?, updated_at = ? WHERE name = ?`,
		string(r.Kind), string(r.Runtime), string(raw), r.UpdatedAt.Format(time.RFC3339), r.Name,
	)
	if err != nil {
		return types.Resource{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return types.Resource{}, ErrNotFound
	}
	return r, nil
}

func (s *Store) Upsert(r types.Resource) (types.Resource, error) {
	_, err := s.Get(r.Name)
	if errors.Is(err, ErrNotFound) {
		return s.Create(r)
	}
	if err != nil {
		return types.Resource{}, err
	}
	return s.Update(r)
}

func (s *Store) PutInstance(inst types.Instance) error {
	_, err := s.db.Exec(
		`INSERT INTO instances (name, container_id, endpoint, image, status) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET container_id=excluded.container_id, endpoint=excluded.endpoint, image=excluded.image, status=excluded.status`,
		inst.Name, inst.ContainerID, inst.Endpoint, inst.Image, inst.Status,
	)
	return err
}

func (s *Store) GetInstance(name string) (types.Instance, error) {
	var inst types.Instance
	err := s.db.QueryRow(
		`SELECT name, container_id, endpoint, image, status FROM instances WHERE name = ?`, name,
	).Scan(&inst.Name, &inst.ContainerID, &inst.Endpoint, &inst.Image, &inst.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return types.Instance{}, ErrNotFound
	}
	if err != nil {
		return types.Instance{}, err
	}
	return inst, nil
}

func (s *Store) DeleteInstance(name string) error {
	_, err := s.db.Exec(`DELETE FROM instances WHERE name = ?`, name)
	return err
}

func (s *Store) Create(r types.Resource) (types.Resource, error) {
	now := time.Now().UTC().Truncate(time.Second)
	r.CreatedAt = now
	r.UpdatedAt = now
	raw, err := json.Marshal(r)
	if err != nil {
		return types.Resource{}, err
	}
	_, err = s.db.Exec(
		`INSERT INTO resources (name, kind, runtime, spec_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		r.Name, string(r.Kind), string(r.Runtime), string(raw), now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		if isUnique(err) {
			return types.Resource{}, ErrExists
		}
		return types.Resource{}, err
	}
	return r, nil
}

func (s *Store) Get(name string) (types.Resource, error) {
	var raw string
	err := s.db.QueryRow(`SELECT spec_json FROM resources WHERE name = ?`, name).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return types.Resource{}, ErrNotFound
	}
	if err != nil {
		return types.Resource{}, err
	}
	var r types.Resource
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return types.Resource{}, err
	}
	return r, nil
}

func (s *Store) List() ([]types.Resource, error) {
	rows, err := s.db.Query(`SELECT spec_json FROM resources ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []types.Resource
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var r types.Resource
		if err := json.Unmarshal([]byte(raw), &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if out == nil {
		out = []types.Resource{}
	}
	return out, rows.Err()
}

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

func (s *Store) AddRevision(name, image, status string) (types.Revision, error) {
	if _, err := s.Get(name); err != nil {
		return types.Revision{}, err
	}
	now := time.Now().UTC().Truncate(time.Second)
	res, err := s.db.Exec(
		`INSERT INTO revisions (name, image, status, created_at) VALUES (?, ?, ?, ?)`,
		name, image, status, now.Format(time.RFC3339),
	)
	if err != nil {
		return types.Revision{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return types.Revision{}, err
	}
	return types.Revision{ID: id, Name: name, Image: image, Status: status, CreatedAt: now}, nil
}

func (s *Store) ListRevisions(name string) ([]types.Revision, error) {
	rows, err := s.db.Query(
		`SELECT id, name, image, status, created_at FROM revisions WHERE name = ? ORDER BY id`,
		name,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []types.Revision
	for rows.Next() {
		var rev types.Revision
		var ts string
		if err := rows.Scan(&rev.ID, &rev.Name, &rev.Image, &rev.Status, &ts); err != nil {
			return nil, err
		}
		rev.CreatedAt, _ = time.Parse(time.RFC3339, ts)
		out = append(out, rev)
	}
	if out == nil {
		out = []types.Revision{}
	}
	return out, rows.Err()
}

func isUnique(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "constraint")
}
