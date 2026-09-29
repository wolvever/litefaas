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
CREATE TABLE IF NOT EXISTS routes (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	spec_json TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS edge_rules (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	spec_json TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
`)
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

func (s *Store) Update(r types.Resource) (types.Resource, error) {
	cur, err := s.Get(r.Name)
	if err != nil {
		return types.Resource{}, err
	}
	now := time.Now().UTC().Truncate(time.Second)
	r.CreatedAt = cur.CreatedAt
	r.UpdatedAt = now
	raw, err := json.Marshal(r)
	if err != nil {
		return types.Resource{}, err
	}
	res, err := s.db.Exec(
		`UPDATE resources SET kind = ?, runtime = ?, spec_json = ?, updated_at = ? WHERE name = ?`,
		string(r.Kind), string(r.Runtime), string(raw), now.Format(time.RFC3339), r.Name,
	)
	if err != nil {
		return types.Resource{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return types.Resource{}, err
	}
	if n == 0 {
		return types.Resource{}, ErrNotFound
	}
	return r, nil
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

// RouteSpec is a persisted edge binding (endpoint is filled at serve time).
type RouteSpec struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	StripPrefix bool   `json:"strip_prefix,omitempty"`
	SPA         bool   `json:"spa,omitempty"`
}

// GetRouteOverride returns the PUT /v1/routes table, or false if derived-from-manifests.
func (s *Store) GetRouteOverride() ([]RouteSpec, bool, error) {
	var raw string
	err := s.db.QueryRow(`SELECT spec_json FROM routes WHERE id = 1`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var out []RouteSpec
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, false, err
	}
	if out == nil {
		out = []RouteSpec{}
	}
	return out, true, nil
}

func (s *Store) SetRouteOverride(routes []RouteSpec) error {
	if routes == nil {
		routes = []RouteSpec{}
	}
	raw, err := json.Marshal(routes)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	_, err = s.db.Exec(
		`INSERT INTO routes (id, spec_json, updated_at) VALUES (1, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET spec_json = excluded.spec_json, updated_at = excluded.updated_at`,
		string(raw), now,
	)
	return err
}

func (s *Store) ClearRouteOverride() error {
	_, err := s.db.Exec(`DELETE FROM routes WHERE id = 1`)
	return err
}

func isUnique(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "constraint")
}

// EdgeRuleSpec is a persisted gateway redirect/rewrite/header rule.
type EdgeRuleSpec struct {
	From    string            `json:"from"`
	To      string            `json:"to,omitempty"`
	Status  int               `json:"status,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Force   bool              `json:"force,omitempty"`
	Source  string            `json:"source,omitempty"`
}

func (s *Store) GetEdgeRules() ([]EdgeRuleSpec, bool, error) {
	var raw string
	err := s.db.QueryRow(`SELECT spec_json FROM edge_rules WHERE id = 1`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var out []EdgeRuleSpec
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, false, err
	}
	if out == nil {
		out = []EdgeRuleSpec{}
	}
	return out, true, nil
}

func (s *Store) SetEdgeRules(rules []EdgeRuleSpec) error {
	if rules == nil {
		rules = []EdgeRuleSpec{}
	}
	raw, err := json.Marshal(rules)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	_, err = s.db.Exec(
		`INSERT INTO edge_rules (id, spec_json, updated_at) VALUES (1, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET spec_json = excluded.spec_json, updated_at = excluded.updated_at`,
		string(raw), now,
	)
	return err
}

func (s *Store) ClearEdgeRules() error {
	_, err := s.db.Exec(`DELETE FROM edge_rules WHERE id = 1`)
	return err
}
