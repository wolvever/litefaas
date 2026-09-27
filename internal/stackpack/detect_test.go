package stackpack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedPacks(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"go-chi-sqlx", "go-echo-gorm", "go-gin-gorm", "java-spring-jpa", "java-spring-mybatis", "node-express-prisma", "node-fastify-prisma", "node-nestjs-prisma", "node-nextjs", "python-django", "python-fastapi", "python-flask-sqlalchemy"}
	got := cat.IDs()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ids = %v want %v", got, want)
	}
	for _, id := range want {
		p, err := cat.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Dockerfile) == 0 || !strings.Contains(string(p.Dockerfile), "$PORT") && !strings.Contains(string(p.Dockerfile), "${PORT}") && !strings.Contains(string(p.Dockerfile), "PORT") {
			t.Fatalf("%s Dockerfile missing PORT contract", id)
		}
		if len(p.Match) == 0 {
			t.Fatalf("%s has no match rules", id)
		}
	}
}

func TestDetectExamples(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		dir string
		id  string
	}{
		{"../../examples/stacks/shop", "java-spring-mybatis"},
		{"../../examples/stacks/catalog", "python-fastapi"},
		{"../../examples/stacks/inventory", "go-gin-gorm"},
		{"../../examples/stacks/notes", "python-flask-sqlalchemy"},
		{"../../examples/stacks/tickets", "node-express-prisma"},
		{"../../examples/stacks/library", "java-spring-jpa"},
		{"../../examples/stacks/portal", "node-nextjs"},
		{"../../examples/stacks/blog", "python-django"},
		{"../../examples/stacks/tasks", "node-nestjs-prisma"},
		{"../../examples/stacks/echo-api", "go-echo-gorm"},
		{"../../examples/stacks/chi-api", "go-chi-sqlx"},
		{"../../examples/stacks/fastify-tasks", "node-fastify-prisma"},
	}
	for _, tc := range cases {
		p, err := cat.Detect(tc.dir)
		if err != nil {
			t.Fatalf("%s: %v", tc.dir, err)
		}
		if p.ID != tc.id {
			t.Fatalf("%s: got %s want %s", tc.dir, p.ID, tc.id)
		}
	}
}

func TestDetectFastAPIPyproject(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\ndependencies = [\"fastapi>=0.115\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "python-fastapi" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestDetectRejectsNearMisses(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project><artifactId>spring-boot-starter-web</artifactId></project>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := cat.Detect(dir); err == nil {
		t.Fatal("spring without mybatis should not match")
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\nrequire github.com/gin-gonic/gin v1.10.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := cat.Detect(dir); err == nil {
		t.Fatal("gin without gorm should not match")
	}
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask==3.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := cat.Detect(dir); err == nil {
		t.Fatal("flask without sqlalchemy should not match")
	}
}

func TestDetectSpringJPA(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pom := "<project><parent><artifactId>spring-boot-starter-parent</artifactId></parent>\n" +
		"<dependencies>\n" +
		"  <dependency><artifactId>spring-boot-starter-data-jpa</artifactId></dependency>\n" +
		"</dependencies></project>\n"
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(pom), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "java-spring-jpa" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestDetectSpringJPABeatsMyBatisWhenBoth(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pom := "<project>\n" +
		"<dependency><artifactId>spring-boot-starter-web</artifactId></dependency>\n" +
		"<dependency><artifactId>spring-boot-starter-data-jpa</artifactId></dependency>\n" +
		"<dependency><artifactId>mybatis-spring-boot-starter</artifactId></dependency>\n" +
		"</project>\n"
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(pom), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "java-spring-jpa" {
		t.Fatalf("got %+v err=%v (jpa priority should win)", p, err)
	}
}

func TestDetectMyBatisNotStolenByJPA(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pom := "<project>\n" +
		"<dependency><artifactId>spring-boot-starter-web</artifactId></dependency>\n" +
		"<dependency><artifactId>mybatis-spring-boot-starter</artifactId></dependency>\n" +
		"</project>\n"
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(pom), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "java-spring-mybatis" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestDetectNextJS(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pkg := "{\"name\":\"x\",\"dependencies\":{\"next\":\"14.2.13\",\"react\":\"18.3.1\"}}\n"
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "node-nextjs" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestDetectNextJSWithConfig(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pkg := "{\"dependencies\":{\"next\":\"14.2.13\"}}\n"
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "next.config.mjs"), []byte("export default { output: 'standalone' };\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "node-nextjs" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestDetectNextJSBeatsExpressPrisma(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pkg := "{\"dependencies\":{\"next\":\"14.2.13\",\"express\":\"4.21.0\",\"@prisma/client\":\"5.20.0\",\"prisma\":\"5.20.0\"}}\n"
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "node-nextjs" {
		t.Fatalf("got %+v err=%v (next should win)", p, err)
	}
}

func TestDetectFlaskSQLAlchemy(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask==3.0.3\nflask-sqlalchemy==3.1.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "python-flask-sqlalchemy" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestDetectFlaskSQLAlchemyPyproject(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\ndependencies = [\"flask>=3\", \"sqlalchemy>=2\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "python-flask-sqlalchemy" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestDetectFastAPIBeatsFlaskWhenBoth(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("fastapi==0.115.0\nflask==3.0.3\nsqlalchemy==2.0.35\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "python-fastapi" {
		t.Fatalf("got %+v err=%v (fastapi priority should win)", p, err)
	}
}

func TestDetectExpressPrisma(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pkg := "{\"name\":\"x\",\"dependencies\":{\"express\":\"^4.21.0\",\"@prisma/client\":\"^5.20.0\"}}\n"
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "node-express-prisma" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestDetectExpressPrismaWithSchema(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pkg := "{\"dependencies\":{\"express\":\"4.21.0\",\"prisma\":\"5.20.0\"}}\n"
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "prisma"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prisma", "schema.prisma"), []byte("generator client { provider = \"prisma-client-js\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "node-express-prisma" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestDetectRejectsExpressWithoutPrisma(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{\"dependencies\":{\"express\":\"4.21.0\"}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := cat.Detect(dir); err == nil {
		t.Fatal("express without prisma should not match")
	}
}

func TestDetectErrorSuggestsStack(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	_, err = cat.Detect(dir)
	if err == nil || !strings.Contains(err.Error(), "--stack") || !strings.Contains(err.Error(), "lf stacks") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "Next:") {
		t.Fatalf("expected Next: guidance in err: %v", err)
	}
}

func TestDetectDjango(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("django==5.1.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "python-django" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestDetectFastAPIBeatsDjango(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("fastapi==0.115.0\ndjango==5.1.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "python-fastapi" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestDetectDjangoBeatsFlask(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("django==5.1.1\nflask==3.0.3\nsqlalchemy==2.0.35\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "python-django" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestDetectNestJSPrisma(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pkg := "{\"dependencies\":{\"@nestjs/core\":\"10.4.4\",\"@prisma/client\":\"5.20.0\"}}\n"
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "node-nestjs-prisma" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestDetectNestBeatsExpress(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pkg := "{\"dependencies\":{\"@nestjs/core\":\"10.4.4\",\"express\":\"4.21.0\",\"@prisma/client\":\"5.20.0\",\"prisma\":\"5.20.0\"}}\n"
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "node-nestjs-prisma" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestDetectNextBeatsNest(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pkg := "{\"dependencies\":{\"next\":\"14.2.13\",\"@nestjs/core\":\"10.4.4\",\"@prisma/client\":\"5.20.0\"}}\n"
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "node-nextjs" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}


func TestDetectEchoGorm(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\nrequire github.com/labstack/echo/v4 v4.12.0\nrequire gorm.io/gorm v1.25.12\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "go-echo-gorm" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestDetectGinBeatsEchoWhenBoth(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	mod := "module x\nrequire github.com/gin-gonic/gin v1.10.0\nrequire github.com/labstack/echo/v4 v4.12.0\nrequire gorm.io/gorm v1.25.12\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "go-gin-gorm" {
		t.Fatalf("got %+v err=%v (gin priority should win)", p, err)
	}
}

func TestDetectChiSqlx(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\nrequire github.com/go-chi/chi/v5 v5.1.0\nrequire github.com/jmoiron/sqlx v1.4.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "go-chi-sqlx" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestDetectChiPgx(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\nrequire github.com/go-chi/chi/v5 v5.1.0\nrequire github.com/jackc/pgx/v5 v5.7.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "go-chi-sqlx" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestDetectRejectsEchoWithoutGorm(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\nrequire github.com/labstack/echo/v4 v4.12.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := cat.Detect(dir); err == nil {
		t.Fatal("echo without gorm should not match")
	}
}

func TestDetectFastifyPrisma(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pkg := "{\"dependencies\":{\"fastify\":\"^4.28.1\",\"@prisma/client\":\"^5.20.0\"}}\n"
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "node-fastify-prisma" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestDetectExpressBeatsFastifyWhenBoth(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pkg := "{\"dependencies\":{\"express\":\"4.21.0\",\"fastify\":\"4.28.1\",\"@prisma/client\":\"5.20.0\"}}\n"
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "node-express-prisma" {
		t.Fatalf("got %+v err=%v (express priority should win)", p, err)
	}
}

func TestDetectRejectsFastifyWithoutPrisma(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{\"dependencies\":{\"fastify\":\"4.28.1\"}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := cat.Detect(dir); err == nil {
		t.Fatal("fastify without prisma should not match")
	}
}

func TestDetectAmbiguous(t *testing.T) {
	cat := NewCatalog()
	a := &Pack{ID: "one", Runtime: "go", Priority: 10, Match: []RuleSet{{Files: []string{"go.mod"}}}}
	b := &Pack{ID: "two", Runtime: "go", Priority: 10, Match: []RuleSet{{Files: []string{"go.mod"}}}}
	if err := cat.Add(a); err != nil {
		t.Fatal(err)
	}
	if err := cat.Add(b); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := cat.Detect(dir)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "priority") {
		t.Fatalf("expected priority in ambiguous err: %v", err)
	}
}

func TestDetectPriorityBreaksTie(t *testing.T) {
	cat := NewCatalog()
	_ = cat.Add(&Pack{ID: "low", Runtime: "go", Priority: 1, Match: []RuleSet{{Files: []string{"go.mod"}}}})
	_ = cat.Add(&Pack{ID: "high", Runtime: "go", Priority: 50, Match: []RuleSet{{Files: []string{"go.mod"}}}})
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := cat.Detect(dir)
	if err != nil || p.ID != "high" {
		t.Fatalf("got %+v err=%v", p, err)
	}
}

func TestNameFromDir(t *testing.T) {
	if g := NameFromDir("/tmp/catalog"); g != "catalog" {
		t.Fatalf("got %q", g)
	}
	if g := NameFromDir("/tmp/weird name!!"); g != "weird-name" {
		t.Fatalf("got %q", g)
	}
}

func TestWriteDockerfile(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	p, err := cat.Get("python-fastapi")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := p.WriteDockerfile(dir); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "uvicorn") {
		t.Fatalf("Dockerfile = %s", raw)
	}
	if _, err := os.Stat(filepath.Join(dir, ".dockerignore")); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownStack(t *testing.T) {
	cat, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cat.Get("quarkus"); err == nil {
		t.Fatal("expected unknown stack")
	}
}

func TestMergeOverlay(t *testing.T) {
	base, err := OpenEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	extra := NewCatalog()
	_ = extra.Add(&Pack{
		ID:      "python-fastapi",
		Runtime: "python",
		Title:   "overlay",
		Match:   []RuleSet{{Files: []string{"requirements.txt"}}},
	})
	base.Merge(extra)
	p, err := base.Get("python-fastapi")
	if err != nil || p.Title != "overlay" {
		t.Fatalf("overlay = %+v err=%v", p, err)
	}
}
