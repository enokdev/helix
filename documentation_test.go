package helix_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestPublicDocumentationOnboarding(t *testing.T) {
	readme := readTextFile(t, "README.md")

	required := []string{
		"# Helix",
		"## Installation",
		"## Quick Start",
		"## Fonctionnalites",
		"## Exemples",
		"## Guides",
		"## Developpement du framework",
		"go mod init example.com/helix-users",
		"go get github.com/enokdev/helix",
		"go run ./examples/crud-api",
		"curl http://localhost:8080/users",
		"helix.Service",
		"helix.Controller",
		"helix.Repository",
	}
	for _, want := range required {
		if !strings.Contains(readme, want) {
			t.Fatalf("README.md should contain %q", want)
		}
	}
}

func TestReadmeBadgesUseRealSignals(t *testing.T) {
	readme := readTextFile(t, "README.md")

	badges := []string{
		"https://github.com/enokdev/helix/actions/workflows/ci.yml/badge.svg",
		"https://github.com/enokdev/helix/actions/workflows/coverage.yml/badge.svg",
		"https://goreportcard.com/badge/github.com/enokdev/helix",
	}
	for _, badge := range badges {
		if !strings.Contains(readme, badge) {
			t.Fatalf("README.md should contain badge %q", badge)
		}
	}
	if strings.Contains(readme, "img.shields.io/badge/coverage-") {
		t.Fatal("README.md must not use a static coverage badge")
	}

	coverage := readTextFile(t, filepath.Join(".github", "workflows", "coverage.yml"))
	coverageChecks := []string{"name: Coverage", "-coverprofile=coverage.out ./...", "go-version: \"1.21\""}
	for _, want := range coverageChecks {
		if !strings.Contains(coverage, want) {
			t.Fatalf("coverage workflow should contain %q", want)
		}
	}
}

func TestReadmeGuideLinksExist(t *testing.T) {
	readme := readTextFile(t, "README.md")
	for _, link := range markdownLinks(readme) {
		if isExternalLink(link) {
			continue
		}
		if _, err := os.Stat(link); err != nil {
			t.Fatalf("README.md link %q should point to an existing repository file: %v", link, err)
		}
	}

	for _, path := range []string{
		filepath.Join("docs", "di-and-config.md"),
		filepath.Join("docs", "http-layer.md"),
		filepath.Join("docs", "data-layer.md"),
		filepath.Join("docs", "security-observability-scheduling.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected guide placeholder %q: %v", path, err)
		}
	}
}

func TestDIAndConfigGuideDocumentsCoreConcepts(t *testing.T) {
	const guidePath = "docs/di-and-config.md"
	if _, err := os.Stat(guidePath); err != nil {
		t.Skipf("test requires %s — guide not found: %v", guidePath, err)
	}
	guide := readTextFile(t, guidePath)

	required := []string{
		"# DI et configuration",
		"## Sommaire",
		"## Modele mental",
		"## Marqueurs de composants",
		"`helix.Service`",
		"`helix.Controller`",
		"`helix.Repository`",
		"`helix.Component`",
		"## Injection de dependances",
		"`inject:\"true\"`",
		"`value:\"key\"`",
		"champs exportes",
		"core.NewContainer(core.WithResolver(core.NewReflectResolver()))",
		"core.ErrNotFound",
		"core.ErrUnresolvable",
		"CyclicDepError",
		"## Scopes",
		"core.ScopeSingleton",
		"core.ScopePrototype",
		"core.ComponentRegistration",
		"Lazy",
		"## Configuration",
		"`mapstructure:\"key\"`",
		"ENV > profil YAML > application.yaml > DEFAULT",
		"config.NewLoader",
		"config.WithDefaults",
		"config.WithProfiles",
		"config.WithEnvPrefix",
		"core.WithValueLookup(loader.Lookup)",
		"SERVER_PORT",
		"HELIX_SERVER_PORT",
		"## Profils",
		"HELIX_PROFILES_ACTIVE",
		"application-dev.yaml",
		"## Rechargement dynamique",
		"config.NewReloader",
		"OnConfigReload",
		"SIGHUP",
		"RLock",
		"RUnlock",
		"## Tests",
		"helix.NewTestApp",
		"helix.TestConfigDefaults",
		"## Erreurs frequentes",
	}
	for _, want := range required {
		if !strings.Contains(guide, want) {
			t.Fatalf("docs/di-and-config.md should contain %q", want)
		}
	}
}

func TestHTTPLayerGuideDocumentsCoreConcepts(t *testing.T) {
	const guidePath = "docs/http-layer.md"
	if _, err := os.Stat(guidePath); err != nil {
		t.Skipf("test requires %s — guide not found: %v", guidePath, err)
	}
	guide := readTextFile(t, guidePath)

	required := []string{
		"# Couche HTTP",
		"## Sommaire",
		"## Modèle mental",
		"## Routing par convention",
		"`web.NewServer`",
		"`web.RegisterController`",
		"`helix.Controller`",
		"`Index`",
		"`Show`",
		"`Create`",
		"`Update`",
		"`Delete`",
		"func (c *UserController) Update(ctx web.Context, input UpdateUserInput) (*User, error)",
		"func (c *UserController) Delete(ctx web.Context) error",
		"`GET /users`",
		"`GET /users/:id`",
		"`POST /users`",
		"`PUT /users/:id`",
		"`DELETE /users/:id`",
		"## Routes custom",
		"`//helix:route GET /users/search`",
		"## Extracteurs types",
		"`web.Context`",
		"`query:\"page\"`",
		"`json:\"email\"`",
		"`validate:\"required,email\"`",
		"## Guards",
		"`//helix:guard authenticated`",
		"**Comportement en cas d'échec**",
		"## Interceptors",
		"`//helix:interceptor cache:5m`",
		"## Mapping des réponses",
		"**Cas (nil, nil)**",
		"## Error handlers",
		"`helix.ErrorHandler`",
		"`//helix:handles ValidationError`",
		"## Tests",
		"`ServeHTTP`",
		"## Erreurs fréquentes",
	}
	for _, want := range required {
		if !strings.Contains(guide, want) {
			t.Fatalf("docs/http-layer.md should contain %q", want)
		}
	}
}

func TestDocumentationLinksExist(t *testing.T) {
	for _, path := range []string{"README.md", filepath.Join("docs", "di-and-config.md"), filepath.Join("docs", "http-layer.md")} {
		content := readTextFile(t, path)
		for _, link := range markdownLinks(content) {
			if isExternalLink(link) {
				continue
			}
			if _, err := os.Stat(link); err != nil {
				t.Fatalf("%s link %q should point to an existing repository file: %v", path, link, err)
			}
		}
	}
}

func TestPublicDocsDoNotLeakInternalPlanningTerms(t *testing.T) {
	paths := []string{"README.md", "CONTRIBUTING.md"}
	entries, err := os.ReadDir("docs")
	if err != nil {
		t.Fatalf("os.ReadDir docs: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			paths = append(paths, filepath.Join("docs", entry.Name()))
		}
	}

	for _, path := range paths {
		content := readTextFile(t, path)
		for _, forbidden := range []string{"_bmad", "BMad", "sprint", "Sprint", "Story "} {
			if strings.Contains(content, forbidden) {
				t.Fatalf("%s should not contain internal planning term %q", path, forbidden)
			}
		}
	}
}

func TestCrudExampleCompiles(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "./examples/crud-api")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test ./examples/crud-api failed: %v\n%s", err, output)
	}
}

func readTextFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func markdownLinks(markdown string) []string {
	markdown = stripFencedCode(markdown)
	markdown = stripInlineCode(markdown)
	pattern := regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)
	matches := pattern.FindAllStringSubmatch(markdown, -1)
	links := make([]string, 0, len(matches))
	for _, match := range matches {
		link := strings.TrimSpace(match[1])
		if i := strings.IndexByte(link, '#'); i >= 0 {
			link = link[:i]
		}
		if link != "" {
			links = append(links, link)
		}
	}
	return links
}

func stripFencedCode(markdown string) string {
	pattern := regexp.MustCompile("(?s)```.*?```")
	return pattern.ReplaceAllString(markdown, "")
}

func stripInlineCode(markdown string) string {
	pattern := regexp.MustCompile("`[^`]*`")
	return pattern.ReplaceAllString(markdown, "")
}

func isExternalLink(link string) bool {
	return strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") || strings.HasPrefix(link, "#")
}
