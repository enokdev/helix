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
		if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") || strings.HasPrefix(link, "#") {
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
