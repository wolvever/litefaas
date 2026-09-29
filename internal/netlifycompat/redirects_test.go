package netlifycompat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRedirects(t *testing.T) {
	src := `
# comment
/home              /
/old  /new  301
/blog/*  /posts/:splat  200
/force  /x  302 !
`
	rules, err := ParseRedirects(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 4 {
		t.Fatalf("len=%d", len(rules))
	}
	if rules[0].From != "/home" || rules[0].To != "/" || rules[0].Status != 301 {
		t.Fatalf("home = %+v", rules[0])
	}
	if rules[2].Status != 200 || rules[2].To != "/posts/:splat" {
		t.Fatalf("splat = %+v", rules[2])
	}
	if !rules[3].Force || rules[3].Status != 302 {
		t.Fatalf("force = %+v", rules[3])
	}
}

func TestParseRedirectsBadStatus(t *testing.T) {
	_, err := ParseRedirects("/a /b 404\n")
	if err == nil || !strings.Contains(err.Error(), "unsupported status") {
		t.Fatalf("got %v", err)
	}
}

func TestParseNetlifyTOML(t *testing.T) {
	src := `
[build]
command = "npm run build"

[[redirects]]
from = "/api/*"
to = "/.netlify/functions/:splat"
status = 200

[[redirects]]
from = "/old"
to = "/new"
status = 301
force = true

[[headers]]
for = "/*"
[headers.values]
X-Frame-Options = "DENY"
X-Content-Type-Options = "nosniff"

[[plugins]]
package = "netlify-plugin-foo"
`
	rules, err := ParseNetlifyTOML(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 3 {
		t.Fatalf("len=%d %+v", len(rules), rules)
	}
	if rules[0].Status != 200 || rules[1].Status != 301 {
		t.Fatalf("%+v", rules)
	}
	if rules[2].Status != 0 || rules[2].Headers["X-Frame-Options"] != "DENY" {
		t.Fatalf("headers = %+v", rules[2])
	}
}

func TestMatchPathSplat(t *testing.T) {
	rules := []EdgeRule{{From: "/blog/*", To: "/posts/:splat", Status: 301}}
	m, ok := MatchPath("/blog/2024/hi", rules)
	if !ok || m.Dest != "/posts/2024/hi" || m.Splat != "2024/hi" {
		t.Fatalf("%+v ok=%v", m, ok)
	}
}

func TestMatchHeaders(t *testing.T) {
	rules := []EdgeRule{
		{From: "/*", Status: 0, Headers: map[string]string{"X-A": "1"}},
		{From: "/api", Status: 0, Headers: map[string]string{"X-B": "2"}},
	}
	h := MatchHeaders("/api", rules)
	if h["X-A"] != "1" || h["X-B"] != "2" {
		t.Fatalf("%v", h)
	}
}

func TestParseVercelJSON(t *testing.T) {
	src := `{
  "redirects": [
    {"source": "/old", "destination": "/new", "permanent": true},
    {"source": "/tmp", "destination": "/x", "statusCode": 307}
  ],
  "rewrites": [
    {"source": "/api/:path*", "destination": "/backend/:splat"}
  ],
  "headers": [
    {"source": "/*", "headers": [{"key": "X-Frame-Options", "value": "DENY"}]}
  ],
  "builds": [{"src": "api/**", "use": "@vercel/node"}]
}`
	rules, err := ParseVercelJSON(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) < 3 {
		t.Fatalf("len=%d %+v", len(rules), rules)
	}
	if rules[0].Status != 301 || rules[0].From != "/old" {
		t.Fatalf("perm = %+v", rules[0])
	}
	if rules[1].Status != 307 {
		t.Fatalf("307 = %+v", rules[1])
	}
	// :path* → /*
	var rewrite *EdgeRule
	for i := range rules {
		if rules[i].Status == 200 {
			rewrite = &rules[i]
			break
		}
	}
	if rewrite == nil || rewrite.From != "/api/*" {
		t.Fatalf("rewrite = %+v", rewrite)
	}
}

func TestParseVercelJSONPathStar(t *testing.T) {
	from, err := normalizeVercelSource("/:path*")
	if err != nil || from != "/*" {
		t.Fatalf("%q %v", from, err)
	}
	from, err = normalizeVercelSource("blog/:slug*")
	if err != nil || from != "/blog/*" {
		t.Fatalf("%q %v", from, err)
	}
}

func TestMatchPathParam(t *testing.T) {
	rules := []EdgeRule{{From: "/users/:id", To: "/u/:id", Status: 301}}
	m, ok := MatchPath("/users/42", rules)
	if !ok || m.Dest != "/u/42" || m.Params["id"] != "42" {
		t.Fatalf("%+v ok=%v", m, ok)
	}
	_, ok = MatchPath("/users/42/extra", rules)
	if ok {
		t.Fatal("expected no match for extra segment")
	}
}

func TestLoadProjectVercel(t *testing.T) {
	dir := t.TempDir()
	raw := `{"redirects":[{"source":"/a","destination":"/b","statusCode":302}]}`
	if err := os.WriteFile(filepath.Join(dir, "vercel.json"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	rules, err := LoadProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].Source != "vercel.json" || rules[0].Status != 302 {
		t.Fatalf("%+v", rules)
	}
}
