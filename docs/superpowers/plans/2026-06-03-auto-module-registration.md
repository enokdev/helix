# Auto-Module Registration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Éliminer l'enregistrement manuel des composants dans `main.go` — `helix generate module` génère un `register.go` avec un `init()`, et `helix generate` produit un `cmd/helix_imports_gen.go` avec des blank imports, permettant à `helix.Run()` de démarrer sans aucune modification de `main.go`.

**Architecture:** `helix.RegisterComponents(components ...any)` accumule les composants dans un slice global (même pattern que `wireSetupFn`). `runAutoBootstrap` fusionne ces composants avant de passer à `runWithApp`. Chaque module scaffoldé reçoit un `register.go` généré ; `helix generate` (appelé par `helix run` avant chaque build) scanne le projet et régénère `cmd/helix_imports_gen.go` avec les blank imports qui déclenchent les `init()`.

**Tech Stack:** Go 1.21+, `go/ast`, `go/parser`, `os/filepath`, `text/template`, `testify` (non requis — `testing` stdlib uniquement)

---

## Carte des fichiers

| Fichier | Action | Responsabilité |
|---------|--------|----------------|
| `helix.go` | Modifier | Ajouter `autoComponents`, `RegisterComponents`, fusion dans `runAutoBootstrap` |
| `helix_test.go` | Modifier | Tests pour `RegisterComponents` et `runAutoBootstrap` |
| `cli/internal/scaffold/templates.go` | Modifier | Ajouter `registerTemplate` |
| `cli/internal/scaffold/scaffold.go` | Modifier | `GenerateModule` et `GenerateContext` génèrent `register.go` |
| `cli/internal/scaffold/scaffold_test.go` | Modifier | Vérifier que `register.go` est généré avec le bon contenu |
| `cli/internal/codegen/imports_gen.go` | Créer | Scanner `register.go` + générateur `helix_imports_gen.go` |
| `cli/internal/codegen/imports_gen_test.go` | Créer | Tests unitaires pour l'imports generator |
| `cli/internal/codegen/generator.go` | Modifier | Appeler `ImportsGenerator` à la fin de `Generate()` |
| `docs/reference/cli.md` | Modifier | Mettre à jour exemples `generate module`, `generate context` |
| `docs/fr/reference/cli.md` | Modifier | Idem en français |
| `docs/guide/cli.md` | Modifier | Supprimer bloc `Components: []any{}` manuel |
| `docs/fr/guide/cli.md` | Modifier | Idem en français |

---

## Task 1 : `helix.RegisterComponents` — API framework

**Files:**
- Modify: `helix.go`
- Modify: `helix_test.go`

- [ ] **Étape 1 : Écrire le test qui échoue**

Dans `helix_test.go`, ajouter après les imports existants :

```go
func TestRegisterComponents_AccumulatesComponents(t *testing.T) {
    // Cannot run in parallel: mutates global autoComponents.
    componentsMu.Lock()
    previous := autoComponents
    componentsMu.Unlock()
    t.Cleanup(func() {
        componentsMu.Lock()
        autoComponents = previous
        componentsMu.Unlock()
    })

    RegisterComponents(&markedService{}, &markedController{})
    RegisterComponents(&markedRepository{})

    componentsMu.Lock()
    got := autoComponents
    componentsMu.Unlock()

    if len(got) != 3 {
        t.Fatalf("RegisterComponents() accumulated %d components, want 3", len(got))
    }
}
```

- [ ] **Étape 2 : Vérifier que le test échoue**

```bash
/Users/yacoubakone/.govm/go/bin/go test -run TestRegisterComponents_AccumulatesComponents ./...
```
Résultat attendu : `FAIL — undefined: RegisterComponents / autoComponents`

- [ ] **Étape 3 : Implémenter dans `helix.go`**

Après les variables globales existantes (`wireSetupFn`, `webSetupFn`), ajouter :

```go
var (
    componentsMu   sync.Mutex
    autoComponents []any
)

// RegisterComponents enregistre des composants pour auto-wiring lors du
// prochain appel à helix.Run() en mode zéro-configuration.
// Conçu pour être appelé depuis des fonctions init() générées par helix generate.
func RegisterComponents(components ...any) {
    componentsMu.Lock()
    defer componentsMu.Unlock()
    autoComponents = append(autoComponents, components...)
}
```

- [ ] **Étape 4 : Vérifier que le test passe**

```bash
/Users/yacoubakone/.govm/go/bin/go test -run TestRegisterComponents_AccumulatesComponents ./...
```
Résultat attendu : `PASS`

- [ ] **Étape 5 : Commit**

```bash
git add helix.go helix_test.go
git commit -m "feat(core): add RegisterComponents for auto-module registration"
```

---

## Task 2 : `runAutoBootstrap` consomme `autoComponents`

**Files:**
- Modify: `helix.go`
- Modify: `helix_test.go`

- [ ] **Étape 1 : Écrire le test qui échoue**

Dans `helix_test.go`, ajouter :

```go
func TestRun_AutoBootstrap_ConsumesRegisteredComponents(t *testing.T) {
    // Cannot run in parallel: mutates global autoComponents.
    componentsMu.Lock()
    previous := autoComponents
    componentsMu.Unlock()
    t.Cleanup(func() {
        componentsMu.Lock()
        autoComponents = previous
        componentsMu.Unlock()
    })

    root := t.TempDir()
    writeTestFile(t, root, "application.yaml", "helix:\n  starters:\n    web:\n      enabled: false\n")
    chdirForTest(t, root)

    started := make(chan struct{}, 1)
    svc := &autoBootstrapService{started: started}
    RegisterComponents(svc)

    err := Run(App{
        awaitShutdown: func() error {
            select {
            case <-started:
            case <-time.After(3 * time.Second):
                t.Fatal("timeout waiting for OnStart")
            }
            return nil
        },
    })
    if err != nil {
        t.Fatalf("Run() error = %v", err)
    }
    if !svc.wasStarted {
        t.Fatal("component registered via RegisterComponents was not started")
    }
}

type autoBootstrapService struct {
    Service
    started    chan struct{}
    wasStarted bool
}

func (s *autoBootstrapService) OnStart() error {
    s.wasStarted = true
    s.started <- struct{}{}
    return nil
}

func (s *autoBootstrapService) OnStop() error { return nil }
```

- [ ] **Étape 2 : Vérifier que le test échoue**

```bash
/Users/yacoubakone/.govm/go/bin/go test -run TestRun_AutoBootstrap_ConsumesRegisteredComponents ./...
```
Résultat attendu : `FAIL — component not started` (autoComponents ignoré)

- [ ] **Étape 3 : Mettre à jour `runAutoBootstrap` dans `helix.go`**

Modifier la fonction `runAutoBootstrap` pour fusionner `autoComponents` avant de déléguer à `runWithApp` :

```go
func runAutoBootstrap(base App) error {
    cfg := autoLoadConfig()

    var settings map[string]any
    _ = cfg.Load(&settings)

    base.Starters = autoDetectStarters(cfg)
    base.valueLookup = cfg.Lookup

    // Collect components registered via RegisterComponents (from init() functions
    // in generated register.go files).
    componentsMu.Lock()
    collected := append([]any(nil), autoComponents...)
    componentsMu.Unlock()
    base.Components = append(base.Components, collected...)

    return runWithApp(base)
}
```

- [ ] **Étape 4 : Vérifier que le test passe**

```bash
/Users/yacoubakone/.govm/go/bin/go test -run TestRun_AutoBootstrap_ConsumesRegisteredComponents ./...
```
Résultat attendu : `PASS`

- [ ] **Étape 5 : Vérifier aucune régression**

```bash
/Users/yacoubakone/.govm/go/bin/go test ./...
```
Résultat attendu : tous verts.

- [ ] **Étape 6 : Commit**

```bash
git add helix.go helix_test.go
git commit -m "feat(core): runAutoBootstrap consumes auto-registered components"
```

---

## Task 3 : Template `register.go` dans le scaffold

**Files:**
- Modify: `cli/internal/scaffold/templates.go`

- [ ] **Étape 1 : Ajouter `registerTemplate` dans `templates.go`**

À la fin du fichier `cli/internal/scaffold/templates.go`, ajouter :

```go
const registerTemplate = `package {{ .PackageName }}

import "github.com/enokdev/helix"

func init() {
	helix.RegisterComponents(
		&{{ .TypeName }}Repository{},
		&{{ .TypeName }}Service{},
		&{{ .TypeName }}Controller{},
	)
}
`
```

- [ ] **Étape 2 : Vérifier la compilation**

```bash
/Users/yacoubakone/.govm/go/bin/go build ./cli/...
```
Résultat attendu : `exit code 0` sans erreur.

- [ ] **Étape 3 : Commit**

```bash
git add cli/internal/scaffold/templates.go
git commit -m "feat(cli): add register.go scaffold template"
```

---

## Task 4 : `GenerateModule` génère `register.go`

**Files:**
- Modify: `cli/internal/scaffold/scaffold.go`
- Modify: `cli/internal/scaffold/scaffold_test.go`

- [ ] **Étape 1 : Mettre à jour le test existant `TestGenerateModuleCreatesUsersPackage`**

Dans `scaffold_test.go`, mettre à jour `TestGenerateModuleCreatesUsersPackage` pour vérifier `register.go` :

```go
func TestGenerateModuleCreatesUsersPackage(t *testing.T) {
    t.Parallel()

    root := newGoModuleFixture(t)

    if err := GenerateModule(ModuleOptions{RootDir: root, Name: "user"}); err != nil {
        t.Fatalf("GenerateModule() error = %v", err)
    }

    for _, name := range []string{
        filepath.Join("users", "repository.go"),
        filepath.Join("users", "service.go"),
        filepath.Join("users", "controller.go"),
        filepath.Join("users", "register.go"),  // nouveau
    } {
        if _, err := os.Stat(filepath.Join(root, name)); err != nil {
            t.Fatalf("generated file %s stat error = %v", name, err)
        }
    }

    service := readFile(t, filepath.Join(root, "users", "service.go"))
    if !strings.Contains(service, "package users") ||
        !strings.Contains(service, "helix.Service") ||
        !strings.Contains(service, "`inject:\"true\"`") {
        t.Fatalf("service.go content did not include expected package, marker, and inject tag:\n%s", service)
    }
    repository := readFile(t, filepath.Join(root, "users", "repository.go"))
    if !strings.Contains(repository, "helix.Repository") {
        t.Fatalf("repository.go content did not include repository marker:\n%s", repository)
    }
    controller := readFile(t, filepath.Join(root, "users", "controller.go"))
    if !strings.Contains(controller, "helix.Controller") || !strings.Contains(controller, "web.Context") {
        t.Fatalf("controller.go content did not include controller marker and web context:\n%s", controller)
    }

    register := readFile(t, filepath.Join(root, "users", "register.go"))
    if !strings.Contains(register, "// Code generated by helix; DO NOT EDIT.") {
        t.Fatalf("register.go missing generated header:\n%s", register)
    }
    if !strings.Contains(register, "helix.RegisterComponents") {
        t.Fatalf("register.go missing RegisterComponents call:\n%s", register)
    }
    if !strings.Contains(register, "&UserRepository{}") ||
        !strings.Contains(register, "&UserService{}") ||
        !strings.Contains(register, "&UserController{}") {
        t.Fatalf("register.go missing component instantiations:\n%s", register)
    }

    cmd := exec.Command("go", "test", "./...")
    cmd.Dir = root
    output, err := cmd.CombinedOutput()
    if err != nil {
        t.Fatalf("go test ./... error = %v\n%s", err, output)
    }
}
```

- [ ] **Étape 2 : Vérifier que le test échoue**

```bash
/Users/yacoubakone/.govm/go/bin/go test -run TestGenerateModuleCreatesUsersPackage ./cli/...
```
Résultat attendu : `FAIL — register.go not found`

- [ ] **Étape 3 : Mettre à jour `GenerateModule` dans `scaffold.go`**

Dans `GenerateModule`, ajouter `register.go` à la map `files` :

```go
files := map[string]string{
    filepath.Join(name.FolderName, "repository.go"): renderGoTemplate(repositoryTemplate, data),
    filepath.Join(name.FolderName, "service.go"):    renderGoTemplate(serviceTemplate, data),
    filepath.Join(name.FolderName, "controller.go"): renderGoTemplate(controllerTemplate, data),
    filepath.Join(name.FolderName, "register.go"):   renderGoTemplate(registerTemplate, data),
}
```

Le template `registerTemplate` contient le header `// Code generated by helix; DO NOT EDIT.` — il faut l'ajouter **avant** `package`. Mettre à jour `registerTemplate` dans `templates.go` pour inclure le header :

```go
const registerTemplate = `// Code generated by helix; DO NOT EDIT.

package {{ .PackageName }}

import "github.com/enokdev/helix"

func init() {
	helix.RegisterComponents(
		&{{ .TypeName }}Repository{},
		&{{ .TypeName }}Service{},
		&{{ .TypeName }}Controller{},
	)
}
`
```

Note : `renderGoTemplate` appelle `go/format` sur la sortie. Un fichier commençant par un commentaire suivi de `package` est du Go valide et sera formaté correctement.

- [ ] **Étape 4 : Vérifier que le test passe**

```bash
/Users/yacoubakone/.govm/go/bin/go test -run TestGenerateModuleCreatesUsersPackage ./cli/...
```
Résultat attendu : `PASS`

- [ ] **Étape 5 : Vérifier les autres tests scaffold**

```bash
/Users/yacoubakone/.govm/go/bin/go test ./cli/internal/scaffold/...
```
Résultat attendu : tous verts.

- [ ] **Étape 6 : Commit**

```bash
git add cli/internal/scaffold/scaffold.go cli/internal/scaffold/scaffold_test.go cli/internal/scaffold/templates.go
git commit -m "feat(cli): generate register.go in helix generate module"
```

---

## Task 5 : `GenerateContext` génère `register.go`

**Files:**
- Modify: `cli/internal/scaffold/scaffold.go`
- Modify: `cli/internal/scaffold/scaffold_test.go`

- [ ] **Étape 1 : Mettre à jour le test existant `TestGenerateContextCreatesAccountsPackage`**

Dans `scaffold_test.go`, ajouter la vérification de `register.go` dans `TestGenerateContextCreatesAccountsPackage` (trouver et étendre la liste des fichiers vérifiés) :

```go
for _, name := range []string{
    filepath.Join("accounts", "api.go"),
    filepath.Join("accounts", "repository.go"),
    filepath.Join("accounts", "service.go"),
    filepath.Join("accounts", "controller.go"),
    filepath.Join("accounts", "register.go"),  // nouveau
} {
    if _, err := os.Stat(filepath.Join(root, name)); err != nil {
        t.Fatalf("generated file %s stat error = %v", name, err)
    }
}
```

Ajouter également la vérification du contenu de `register.go` après les vérifications existantes :

```go
register := readFile(t, filepath.Join(root, "accounts", "register.go"))
if !strings.Contains(register, "// Code generated by helix; DO NOT EDIT.") {
    t.Fatalf("register.go missing generated header:\n%s", register)
}
if !strings.Contains(register, "helix.RegisterComponents") {
    t.Fatalf("register.go missing RegisterComponents call:\n%s", register)
}
if !strings.Contains(register, "&AccountRepository{}") ||
    !strings.Contains(register, "&AccountService{}") ||
    !strings.Contains(register, "&AccountController{}") {
    t.Fatalf("register.go missing component instantiations:\n%s", register)
}
```

- [ ] **Étape 2 : Vérifier que le test échoue**

```bash
/Users/yacoubakone/.govm/go/bin/go test -run TestGenerateContextCreatesAccountsPackage ./cli/...
```
Résultat attendu : `FAIL — register.go not found`

- [ ] **Étape 3 : Mettre à jour `GenerateContext` dans `scaffold.go`**

Dans `GenerateContext`, ajouter `register.go` à la map `files` :

```go
files := map[string]string{
    filepath.Join(name.FolderName, "api.go"):        renderGoTemplate(contextAPITemplate, data),
    filepath.Join(name.FolderName, "repository.go"): renderGoTemplate(contextRepositoryTemplate, data),
    filepath.Join(name.FolderName, "service.go"):    renderGoTemplate(contextServiceTemplate, data),
    filepath.Join(name.FolderName, "controller.go"): renderGoTemplate(contextControllerTemplate, data),
    filepath.Join(name.FolderName, "register.go"):   renderGoTemplate(registerTemplate, contextTemplateData(data)),
}
```

`contextTemplateData` est déjà un `contextTemplateData` struct qui a les mêmes champs (`PackageName`, `TypeName`, `FolderName`) que `moduleTemplateData` — `registerTemplate` fonctionne donc avec les deux.

Concrètement, remplacer la map files par :

```go
files := map[string]string{
    filepath.Join(name.FolderName, "api.go"):        renderGoTemplate(contextAPITemplate, data),
    filepath.Join(name.FolderName, "repository.go"): renderGoTemplate(contextRepositoryTemplate, data),
    filepath.Join(name.FolderName, "service.go"):    renderGoTemplate(contextServiceTemplate, data),
    filepath.Join(name.FolderName, "controller.go"): renderGoTemplate(contextControllerTemplate, data),
    filepath.Join(name.FolderName, "register.go"):   renderGoTemplate(registerTemplate, data),
}
```

- [ ] **Étape 4 : Vérifier que le test passe**

```bash
/Users/yacoubakone/.govm/go/bin/go test -run TestGenerateContextCreatesAccountsPackage ./cli/...
```
Résultat attendu : `PASS`

- [ ] **Étape 5 : Vérifier toute la suite scaffold**

```bash
/Users/yacoubakone/.govm/go/bin/go test ./cli/internal/scaffold/...
```
Résultat attendu : tous verts.

- [ ] **Étape 6 : Commit**

```bash
git add cli/internal/scaffold/scaffold.go cli/internal/scaffold/scaffold_test.go
git commit -m "feat(cli): generate register.go in helix generate context"
```

---

## Task 6 : `ImportsGenerator` — scanner + générateur de `helix_imports_gen.go`

**Files:**
- Create: `cli/internal/codegen/imports_gen.go`
- Create: `cli/internal/codegen/imports_gen_test.go`

- [ ] **Étape 1 : Écrire les tests qui échouent**

Créer `cli/internal/codegen/imports_gen_test.go` :

```go
package codegen

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestImportsGenerator_WritesImportsFile(t *testing.T) {
    t.Parallel()

    root := t.TempDir()
    // go.mod
    writeFixture(t, root, "go.mod", "module example.test/myapp\n\ngo 1.21.0\n")
    // Simulated generated register.go in orders/
    ordersDir := filepath.Join(root, "orders")
    if err := os.MkdirAll(ordersDir, 0o755); err != nil {
        t.Fatalf("mkdir orders: %v", err)
    }
    writeFixture(t, ordersDir, "register.go",
        "// Code generated by helix; DO NOT EDIT.\npackage orders\n\nimport \"github.com/enokdev/helix\"\n\nfunc init() {\n\thelix.RegisterComponents()\n}\n")
    // cmd/ with main.go
    cmdDir := filepath.Join(root, "cmd")
    if err := os.MkdirAll(cmdDir, 0o755); err != nil {
        t.Fatalf("mkdir cmd: %v", err)
    }
    writeFixture(t, cmdDir, "main.go", "package main\n\nfunc main() {}\n")

    gen := NewImportsGenerator(root)
    result, err := gen.Generate()
    if err != nil {
        t.Fatalf("ImportsGenerator.Generate() error = %v", err)
    }

    if len(result.Files) == 0 {
        t.Fatal("ImportsGenerator.Generate() produced no files")
    }

    outPath := filepath.Join(root, "cmd", "helix_imports_gen.go")
    if _, err := os.Stat(outPath); err != nil {
        t.Fatalf("helix_imports_gen.go not created: %v", err)
    }

    content := readFile(t, outPath)
    if !strings.Contains(content, "// Code generated by helix; DO NOT EDIT.") {
        t.Fatalf("missing codegen header:\n%s", content)
    }
    if !strings.Contains(content, `_ "example.test/myapp/orders"`) {
        t.Fatalf("missing blank import for orders:\n%s", content)
    }
    if !strings.Contains(content, "package main") {
        t.Fatalf("wrong package name:\n%s", content)
    }
}

func TestImportsGenerator_NoModules_NoFileWritten(t *testing.T) {
    t.Parallel()

    root := t.TempDir()
    writeFixture(t, root, "go.mod", "module example.test/empty\n\ngo 1.21.0\n")

    gen := NewImportsGenerator(root)
    result, err := gen.Generate()
    if err != nil {
        t.Fatalf("ImportsGenerator.Generate() error = %v", err)
    }

    if len(result.Files) != 0 {
        t.Fatalf("ImportsGenerator.Generate() wrote files unexpectedly: %v", result.Files)
    }
    if _, err := os.Stat(filepath.Join(root, "cmd", "helix_imports_gen.go")); !os.IsNotExist(err) {
        t.Fatal("helix_imports_gen.go should not be created when no modules exist")
    }
}

func TestImportsGenerator_MultipleModules_SortedImports(t *testing.T) {
    t.Parallel()

    root := t.TempDir()
    writeFixture(t, root, "go.mod", "module example.test/multi\n\ngo 1.21.0\n")

    for _, pkg := range []string{"zebras", "apples", "mangoes"} {
        dir := filepath.Join(root, pkg)
        if err := os.MkdirAll(dir, 0o755); err != nil {
            t.Fatalf("mkdir %s: %v", pkg, err)
        }
        writeFixture(t, dir, "register.go",
            "// Code generated by helix; DO NOT EDIT.\npackage "+pkg+"\n\nimport \"github.com/enokdev/helix\"\n\nfunc init() {}\n")
    }

    gen := NewImportsGenerator(root)
    _, err := gen.Generate()
    if err != nil {
        t.Fatalf("ImportsGenerator.Generate() error = %v", err)
    }

    outPath := filepath.Join(root, "helix_imports_gen.go")
    content := readFile(t, outPath)

    applePos := strings.Index(content, "apples")
    mangoPos := strings.Index(content, "mangoes")
    zebraPos := strings.Index(content, "zebras")
    if !(applePos < mangoPos && mangoPos < zebraPos) {
        t.Fatalf("imports not sorted alphabetically:\n%s", content)
    }
}

func TestImportsGenerator_FallsBackToRoot_WhenNoCmdDir(t *testing.T) {
    t.Parallel()

    root := t.TempDir()
    writeFixture(t, root, "go.mod", "module example.test/nomd\n\ngo 1.21.0\n")
    // No cmd/ directory
    pkgDir := filepath.Join(root, "orders")
    if err := os.MkdirAll(pkgDir, 0o755); err != nil {
        t.Fatalf("mkdir orders: %v", err)
    }
    writeFixture(t, pkgDir, "register.go",
        "// Code generated by helix; DO NOT EDIT.\npackage orders\n\nimport \"github.com/enokdev/helix\"\n\nfunc init() {}\n")
    // main.go at root (no cmd/)
    writeFixture(t, root, "main.go", "package main\n\nfunc main() {}\n")

    gen := NewImportsGenerator(root)
    _, err := gen.Generate()
    if err != nil {
        t.Fatalf("ImportsGenerator.Generate() error = %v", err)
    }

    outPath := filepath.Join(root, "helix_imports_gen.go")
    if _, err := os.Stat(outPath); err != nil {
        t.Fatalf("helix_imports_gen.go not created at root: %v", err)
    }
}

func TestImportsGenerator_IdempotentTwoRuns(t *testing.T) {
    t.Parallel()

    root := t.TempDir()
    writeFixture(t, root, "go.mod", "module example.test/idem\n\ngo 1.21.0\n")
    ordersDir := filepath.Join(root, "orders")
    if err := os.MkdirAll(ordersDir, 0o755); err != nil {
        t.Fatalf("mkdir orders: %v", err)
    }
    writeFixture(t, ordersDir, "register.go",
        "// Code generated by helix; DO NOT EDIT.\npackage orders\n\nimport \"github.com/enokdev/helix\"\n\nfunc init() {}\n")

    gen := NewImportsGenerator(root)
    if _, err := gen.Generate(); err != nil {
        t.Fatalf("first Generate() error = %v", err)
    }
    if _, err := gen.Generate(); err != nil {
        t.Fatalf("second Generate() error = %v", err)
    }

    outPath := filepath.Join(root, "helix_imports_gen.go")
    content := readFile(t, outPath)
    count := strings.Count(content, `"example.test/idem/orders"`)
    if count != 1 {
        t.Fatalf("import appears %d times, want 1:\n%s", count, content)
    }
}
```

- [ ] **Étape 2 : Vérifier que les tests échouent**

```bash
/Users/yacoubakone/.govm/go/bin/go test -run TestImportsGenerator ./cli/internal/codegen/...
```
Résultat attendu : `FAIL — undefined: NewImportsGenerator`

- [ ] **Étape 3 : Implémenter `cli/internal/codegen/imports_gen.go`**

Créer le fichier :

```go
package codegen

import (
    "bytes"
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"

    "github.com/enokdev/helix/cli/internal/gofmtx"
)

const importsGenHeader = "// Code generated by helix; DO NOT EDIT."
const importsGenFilename = "helix_imports_gen.go"

// ImportsGeneratorResult describes the output of one ImportsGenerator run.
type ImportsGeneratorResult struct {
    Files []string
}

// ImportsGenerator scans a project for generated register.go files and writes
// a helix_imports_gen.go file with blank imports that trigger their init().
type ImportsGenerator struct {
    dir string
}

// NewImportsGenerator creates a generator rooted at dir.
func NewImportsGenerator(dir string) *ImportsGenerator {
    if dir == "" {
        dir = "."
    }
    return &ImportsGenerator{dir: dir}
}

// Generate scans for register.go files and writes helix_imports_gen.go.
// Returns an empty result (no files written) when no generated register.go
// files are found.
func (g *ImportsGenerator) Generate() (ImportsGeneratorResult, error) {
    root, err := filepath.Abs(g.dir)
    if err != nil {
        return ImportsGeneratorResult{}, fmt.Errorf("cli/codegen: imports gen: resolve root: %w", err)
    }

    _, modulePath, err := findGoModule(root)
    if err != nil {
        return ImportsGeneratorResult{}, fmt.Errorf("cli/codegen: imports gen: %w", err)
    }

    importPaths, err := findRegisteredModuleImports(root, modulePath)
    if err != nil {
        return ImportsGeneratorResult{}, err
    }
    if len(importPaths) == 0 {
        return ImportsGeneratorResult{}, nil
    }

    outDir := resolveOutputDir(root)
    outPath := filepath.Join(outDir, importsGenFilename)

    content, err := renderImportsFile(importPaths)
    if err != nil {
        return ImportsGeneratorResult{}, err
    }

    if err := os.MkdirAll(outDir, 0o755); err != nil {
        return ImportsGeneratorResult{}, fmt.Errorf("cli/codegen: imports gen: mkdir %s: %w", outDir, err)
    }
    if err := os.WriteFile(outPath, content, 0o644); err != nil {
        return ImportsGeneratorResult{}, fmt.Errorf("cli/codegen: imports gen: write %s: %w", outPath, err)
    }

    return ImportsGeneratorResult{Files: []string{outPath}}, nil
}

// findRegisteredModuleImports walks the project tree and returns sorted import
// paths for every package that contains a generated register.go file.
func findRegisteredModuleImports(root, modulePath string) ([]string, error) {
    var imports []string

    err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
        if walkErr != nil {
            return walkErr
        }
        if d.IsDir() {
            if path != root && shouldSkipDir(d.Name()) {
                return filepath.SkipDir
            }
            return nil
        }
        if d.Name() != "register.go" {
            return nil
        }
        content, err := os.ReadFile(path)
        if err != nil {
            return fmt.Errorf("cli/codegen: imports gen: read %s: %w", path, err)
        }
        if !strings.HasPrefix(strings.TrimSpace(string(content)), importsGenHeader) {
            return nil
        }
        dir := filepath.Dir(path)
        importPath, err := importPathForDir(root, modulePath, dir)
        if err != nil {
            return fmt.Errorf("cli/codegen: imports gen: import path for %s: %w", dir, err)
        }
        imports = append(imports, importPath)
        return nil
    })
    if err != nil {
        return nil, fmt.Errorf("cli/codegen: imports gen: walk %s: %w", root, err)
    }

    sort.Strings(imports)
    return imports, nil
}

// resolveOutputDir returns the directory where helix_imports_gen.go should be
// written. Prefers cmd/ if it exists at the root, falls back to root itself.
func resolveOutputDir(root string) string {
    cmdDir := filepath.Join(root, "cmd")
    if info, err := os.Stat(cmdDir); err == nil && info.IsDir() {
        return cmdDir
    }
    return root
}

// renderImportsFile produces the formatted Go source for helix_imports_gen.go.
func renderImportsFile(importPaths []string) ([]byte, error) {
    var buf bytes.Buffer
    buf.WriteString(importsGenHeader + "\n\npackage main\n\nimport (\n")
    for _, imp := range importPaths {
        fmt.Fprintf(&buf, "\t_ %q\n", imp)
    }
    buf.WriteString(")\n")

    formatted, err := gofmtx.Source(buf.Bytes())
    if err != nil {
        return nil, fmt.Errorf("cli/codegen: imports gen: format: %w", err)
    }
    return formatted, nil
}
```

- [ ] **Étape 4 : Vérifier que les tests passent**

```bash
/Users/yacoubakone/.govm/go/bin/go test -run TestImportsGenerator ./cli/internal/codegen/...
```
Résultat attendu : tous `PASS`

- [ ] **Étape 5 : Vérifier toute la suite codegen**

```bash
/Users/yacoubakone/.govm/go/bin/go test ./cli/internal/codegen/...
```
Résultat attendu : tous verts.

- [ ] **Étape 6 : Commit**

```bash
git add cli/internal/codegen/imports_gen.go cli/internal/codegen/imports_gen_test.go
git commit -m "feat(cli): add ImportsGenerator for helix_imports_gen.go"
```

---

## Task 7 : Câbler `Generator.Generate()` vers `ImportsGenerator`

**Files:**
- Modify: `cli/internal/codegen/generator.go`
- Modify: `cli/internal/codegen/generator_test.go` (ou `generator_integration_test.go`)

- [ ] **Étape 1 : Écrire le test**

Ajouter dans `cli/internal/codegen/generator_test.go` (ou un nouveau fichier de test) :

```go
func TestGenerator_Generate_WritesImportsFile_WhenRegisterGoExists(t *testing.T) {
    t.Parallel()

    dir := t.TempDir()
    writeFixture(t, dir, "go.mod", "module example.test/wired\n\ngo 1.21.0\n")

    // Simulated generated register.go in orders/
    ordersDir := filepath.Join(dir, "orders")
    if err := os.MkdirAll(ordersDir, 0o755); err != nil {
        t.Fatalf("mkdir orders: %v", err)
    }
    writeFixture(t, ordersDir, "register.go",
        "// Code generated by helix; DO NOT EDIT.\npackage orders\n\nimport \"github.com/enokdev/helix\"\n\nfunc init() {}\n")

    result, err := NewGenerator(dir).Generate(context.Background())
    if err != nil {
        t.Fatalf("Generate() error = %v", err)
    }

    outPath := filepath.Join(dir, "helix_imports_gen.go")
    found := false
    for _, f := range result.Files {
        if f == outPath {
            found = true
            break
        }
    }
    if !found {
        t.Fatalf("Generate() result.Files = %v, want to contain %s", result.Files, outPath)
    }
    if _, err := os.Stat(outPath); err != nil {
        t.Fatalf("helix_imports_gen.go not created: %v", err)
    }
}
```

- [ ] **Étape 2 : Vérifier que le test échoue**

```bash
/Users/yacoubakone/.govm/go/bin/go test -run TestGenerator_Generate_WritesImportsFile ./cli/internal/codegen/...
```
Résultat attendu : `FAIL — helix_imports_gen.go not created`

- [ ] **Étape 3 : Mettre à jour `Generator.Generate()` dans `generator.go`**

À la fin de la fonction `Generate`, avant le `return`, appeler `ImportsGenerator` :

```go
func (g *Generator) Generate(ctx context.Context) (Result, error) {
    if ctx == nil {
        return Result{}, fmt.Errorf("cli/codegen: generate: nil context: %w", errInvalidPackage)
    }
    root := g.dir
    if root == "" {
        root = "."
    }

    var files []string
    if err := scanPackages(ctx, root, func(pkg *packageModel) error {
        changed, err := generatePackage(pkg)
        if err != nil {
            return err
        }
        files = append(files, changed...)
        return nil
    }); err != nil {
        return Result{}, fmt.Errorf("cli/codegen: generate %s: %w", root, err)
    }

    // Generate helix_imports_gen.go for auto-module registration.
    importsResult, err := NewImportsGenerator(root).Generate()
    if err != nil {
        return Result{}, fmt.Errorf("cli/codegen: generate imports: %w", err)
    }
    files = append(files, importsResult.Files...)

    sort.Strings(files)
    return Result{GeneratedFiles: len(files), Files: files}, nil
}
```

- [ ] **Étape 4 : Vérifier que le test passe**

```bash
/Users/yacoubakone/.govm/go/bin/go test -run TestGenerator_Generate_WritesImportsFile ./cli/internal/codegen/...
```
Résultat attendu : `PASS`

- [ ] **Étape 5 : Vérifier toute la suite codegen et framework**

```bash
/Users/yacoubakone/.govm/go/bin/go test ./...
```
Résultat attendu : tous verts.

- [ ] **Étape 6 : Commit**

```bash
git add cli/internal/codegen/generator.go cli/internal/codegen/generator_test.go
git commit -m "feat(cli): Generator.Generate triggers ImportsGenerator"
```

---

## Task 8 : Mise à jour de la documentation

**Files:**
- Modify: `docs/reference/cli.md`
- Modify: `docs/fr/reference/cli.md`
- Modify: `docs/guide/cli.md`
- Modify: `docs/fr/guide/cli.md`

- [ ] **Étape 1 : Mettre à jour `docs/reference/cli.md` — section `helix generate module`**

Trouver le bloc (autour de la ligne 165) :

```
**Generated structure** (`helix generate module order`):

```
orders/
├── controller.go
├── service.go
└── repository.go
```
```

Remplacer par :

```
**Generated structure** (`helix generate module order`):

```
orders/
├── controller.go
├── service.go
├── repository.go
└── register.go   # auto-registration via init()
```
```

Trouver et supprimer (autour de la ligne 221) :

```
After generating, register the components in `main.go`:

```go
helix.Run(helix.App{
    Components: []any{
        &orders.OrderRepository{},
        &orders.OrderService{},
        &orders.OrderController{},
    },
})
```
```

Remplacer par :

```
Components are registered automatically. No changes to `main.go` are required — run `helix run` directly.
```

- [ ] **Étape 2 : Mettre à jour `docs/reference/cli.md` — section `helix generate context`**

Chercher la section `helix generate context` et appliquer les mêmes changements :
- Ajouter `└── register.go` à la structure de fichiers
- Supprimer tout bloc `Components: []any{...}` manuel

- [ ] **Étape 3 : Mettre à jour `docs/reference/cli.md` — section `helix run`**

Dans la section `helix run`, ajouter une note :

```
> `helix run` automatically calls `helix generate` before each build, regenerating `cmd/helix_imports_gen.go` to include any new modules added since the last run.
```

- [ ] **Étape 4 : Appliquer les mêmes changements à `docs/fr/reference/cli.md`**

Même éditions en français :
- Ajouter `└── register.go   # auto-enregistrement via init()` à la structure
- Supprimer le bloc `Components: []any{...}` manuel
- Ajouter la note sur `helix run` en français

- [ ] **Étape 5 : Mettre à jour `docs/guide/cli.md`**

Chercher et supprimer tout bloc `Components: []any{...}` dans les exemples post-`generate module`. Remplacer par :

```
Les composants sont enregistrés automatiquement. Lancez `helix run` directement.
```

- [ ] **Étape 6 : Mettre à jour `docs/fr/guide/cli.md`**

Appliquer les mêmes changements à la version française.

- [ ] **Étape 7 : Commit**

```bash
git add docs/reference/cli.md docs/fr/reference/cli.md docs/guide/cli.md docs/fr/guide/cli.md
git commit -m "docs: update CLI docs for auto-module registration"
```

---

## Vérification finale

- [ ] **Lint complet**

```bash
golangci-lint run
```
Résultat attendu : aucune erreur.

- [ ] **Suite de tests complète**

```bash
/Users/yacoubakone/.govm/go/bin/go test ./...
```
Résultat attendu : tous verts.

- [ ] **Build complet**

```bash
/Users/yacoubakone/.govm/go/bin/go build ./...
```
Résultat attendu : exit code 0.
