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

// DefaultRevisionKeep is how many revision rows to retain per resource.
// Pinned rows and the newest row are exempt.
const DefaultRevisionKeep = 5

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
	if err != nil {
		return err
	}
	for _, stmt := range []string{
		`ALTER TABLE revisions ADD COLUMN image_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE revisions ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE revisions ADD COLUMN snapshot_json TEXT NOT NULL DEFAULT '{}'`,
		`ALTER TABLE revisions ADD COLUMN target TEXT NOT NULL DEFAULT 'prod'`,
	} {
		if err := ignoreDupColumn(s.db.Exec(stmt)); err != nil {
			return err
		}
	}
	return nil
}

func ignoreDupColumn(res sql.Result, err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "duplicate column") {
		return nil
	}
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
	return s.AddRevisionFull(types.Revision{Name: name, Image: image, Status: status})
}

func (s *Store) AddRevisionFull(rev types.Revision) (types.Revision, error) {
	if _, err := s.Get(rev.Name); err != nil {
		return types.Revision{}, err
	}
	rev.Snapshot = types.SanitizeSnapshot(rev.Snapshot)
	raw, err := json.Marshal(rev.Snapshot)
	if err != nil {
		return types.Revision{}, err
	}
	if rev.Status == "" {
		rev.Status = "deployed"
	}
	if rev.Target == "" {
		rev.Target = "prod"
	}
	now := time.Now().UTC().Truncate(time.Second)
	rev.CreatedAt = now
	pin := 0
	if rev.Pinned {
		pin = 1
	}
	res, err := s.db.Exec(
		`INSERT INTO revisions (name, image, status, created_at, image_id, pinned, snapshot_json, target) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		rev.Name, rev.Image, rev.Status, now.Format(time.RFC3339), rev.ImageID, pin, string(raw), rev.Target,
	)
	if err != nil {
		return types.Revision{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return types.Revision{}, err
	}
	rev.ID = id
	return rev, nil
}

func (s *Store) ListRevisions(name string) ([]types.Revision, error) {
	rows, err := s.db.Query(
		`SELECT id, name, image, status, created_at, image_id, pinned, snapshot_json, target FROM revisions WHERE name = ? ORDER BY id`,
		name,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []types.Revision
	for rows.Next() {
		rev, err := scanRevision(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rev)
	}
	if out == nil {
		out = []types.Revision{}
	}
	return out, rows.Err()
}

func scanRevision(rows *sql.Rows) (types.Revision, error) {
	var rev types.Revision
	var ts, snap string
	var pin int
	if err := rows.Scan(&rev.ID, &rev.Name, &rev.Image, &rev.Status, &ts, &rev.ImageID, &pin, &snap, &rev.Target); err != nil {
		return types.Revision{}, err
	}
	if rev.Target == "" {
		rev.Target = "prod"
	}
	rev.Pinned = pin != 0
	rev.CreatedAt, _ = time.Parse(time.RFC3339, ts)
	if snap != "" && snap != "{}" {
		_ = json.Unmarshal([]byte(snap), &rev.Snapshot)
	}
	rev.Snapshot = types.SanitizeSnapshot(rev.Snapshot)
	return rev, nil
}

func (s *Store) GetRevision(name string, id int64) (types.Revision, error) {
	rows, err := s.db.Query(
		`SELECT id, name, image, status, created_at, image_id, pinned, snapshot_json, target FROM revisions WHERE name = ? AND id = ?`,
		name, id,
	)
	if err != nil {
		return types.Revision{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return types.Revision{}, err
		}
		return types.Revision{}, ErrNotFound
	}
	return scanRevision(rows)
}

func (s *Store) SetRevisionPinned(name string, id int64, pinned bool) (types.Revision, error) {
	pin := 0
	if pinned {
		pin = 1
	}
	res, err := s.db.Exec(`UPDATE revisions SET pinned = ? WHERE name = ? AND id = ?`, pin, name, id)
	if err != nil {
		return types.Revision{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return types.Revision{}, err
	}
	if n == 0 {
		return types.Revision{}, ErrNotFound
	}
	return s.GetRevision(name, id)
}

// PruneRevisions deletes oldest unpinned revisions until len<=keep.
// The newest revision is never deleted. Pinned revisions are exempt, so the
// retained count can exceed keep.
func (s *Store) PruneRevisions(name string, keep int) (int, error) {
	if keep < 1 {
		keep = DefaultRevisionKeep
	}
	revs, err := s.ListRevisions(name)
	if err != nil {
		return 0, err
	}
	if len(revs) <= keep {
		return 0, nil
	}
	groups := map[string][]types.Revision{}
	for _, r := range revs {
		t := r.Target
		if t == "" {
			t = "prod"
		}
		groups[t] = append(groups[t], r)
	}
	n := 0
	for _, group := range groups {
		if len(group) <= keep {
			continue
		}
		newest := group[len(group)-1].ID
		excess := len(group) - keep
		for _, r := range group {
			if excess <= 0 {
				break
			}
			if r.Pinned || r.ID == newest {
				continue
			}
			if _, err := s.db.Exec(`DELETE FROM revisions WHERE name = ? AND id = ?`, name, r.ID); err != nil {
				return n, err
			}
			n++
			excess--
		}
	}
	return n, nil
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
// Project is stamped from the owning bucket on read (empty = legacy unscoped).
type EdgeRuleSpec struct {
	From    string            `json:"from"`
	To      string            `json:"to,omitempty"`
	Status  int               `json:"status,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Force   bool              `json:"force,omitempty"`
	Source  string            `json:"source,omitempty"`
	Project string            `json:"project,omitempty"`
}

// edgeProject is one project's rules. ID "" is the legacy unscoped bucket
// written by a bare-array PUT.
type edgeProject struct {
	ID    string         `json:"id"`
	Rules []EdgeRuleSpec `json:"rules"`
}

// edgeDoc is the on-disk edge_rules document. Older rows are a raw JSON array.
type edgeDoc struct {
	Projects []edgeProject `json:"projects"`
}

func (s *Store) GetEdgeRules() ([]EdgeRuleSpec, bool, error) {
	doc, ok, err := s.loadEdgeDoc()
	if err != nil || !ok {
		return nil, ok, err
	}
	return flattenEdgeDoc(doc), true, nil
}

// SetEdgeRules replaces the whole table with one unscoped bucket (legacy PUT).
func (s *Store) SetEdgeRules(rules []EdgeRuleSpec) error {
	if rules == nil {
		rules = []EdgeRuleSpec{}
	}
	for i := range rules {
		rules[i].Project = ""
	}
	return s.saveEdgeDoc(edgeDoc{Projects: []edgeProject{{ID: "", Rules: rules}}})
}

// SetProjectEdgeRules replaces rules for project and leaves every other project.
// A sole legacy "" bucket is replaced (it was the only project on the gateway).
// Empty rules drops the project. project must be non-empty.
func (s *Store) SetProjectEdgeRules(project string, rules []EdgeRuleSpec) error {
	if strings.TrimSpace(project) == "" {
		return fmt.Errorf("project id required")
	}
	doc, _, err := s.loadEdgeDoc()
	if err != nil {
		return err
	}
	if rules == nil {
		rules = []EdgeRuleSpec{}
	}
	for i := range rules {
		rules[i].Project = project
	}
	if len(doc.Projects) == 1 && doc.Projects[0].ID == "" {
		doc.Projects = nil
	}
	if len(rules) == 0 {
		var dst []edgeProject
		for _, p := range doc.Projects {
			if p.ID != project {
				dst = append(dst, p)
			}
		}
		doc.Projects = dst
		return s.saveEdgeDoc(doc)
	}
	for i := range doc.Projects {
		if doc.Projects[i].ID == project {
			doc.Projects[i].Rules = rules
			return s.saveEdgeDoc(doc)
		}
	}
	doc.Projects = append(doc.Projects, edgeProject{ID: project, Rules: rules})
	return s.saveEdgeDoc(doc)
}

func (s *Store) ClearEdgeRules() error {
	_, err := s.db.Exec(`DELETE FROM edge_rules WHERE id = 1`)
	return err
}

func (s *Store) loadEdgeDoc() (edgeDoc, bool, error) {
	var raw string
	err := s.db.QueryRow(`SELECT spec_json FROM edge_rules WHERE id = 1`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return edgeDoc{}, false, nil
	}
	if err != nil {
		return edgeDoc{}, false, err
	}
	doc, err := decodeEdgeDoc(raw)
	if err != nil {
		return edgeDoc{}, false, err
	}
	return doc, true, nil
}

func decodeEdgeDoc(raw string) (edgeDoc, error) {
	trim := strings.TrimSpace(raw)
	if trim == "" {
		return edgeDoc{}, nil
	}
	if trim[0] == '[' {
		var rules []EdgeRuleSpec
		if err := json.Unmarshal([]byte(trim), &rules); err != nil {
			return edgeDoc{}, err
		}
		if rules == nil {
			rules = []EdgeRuleSpec{}
		}
		return edgeDoc{Projects: []edgeProject{{ID: "", Rules: rules}}}, nil
	}
	var doc edgeDoc
	if err := json.Unmarshal([]byte(trim), &doc); err != nil {
		return edgeDoc{}, err
	}
	if doc.Projects == nil {
		doc.Projects = []edgeProject{}
	}
	return doc, nil
}

func flattenEdgeDoc(doc edgeDoc) []EdgeRuleSpec {
	var out []EdgeRuleSpec
	for _, p := range doc.Projects {
		for _, r := range p.Rules {
			r.Project = p.ID
			out = append(out, r)
		}
	}
	if out == nil {
		out = []EdgeRuleSpec{}
	}
	return out
}

func (s *Store) saveEdgeDoc(doc edgeDoc) error {
	if doc.Projects == nil {
		doc.Projects = []edgeProject{}
	}
	raw, err := json.Marshal(doc)
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
