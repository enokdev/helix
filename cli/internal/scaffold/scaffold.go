package scaffold

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"
)

var errInvalidName = errors.New("invalid scaffold name")

// Options configures application scaffolding.
type Options struct {
	RootDir          string
	Name             string
	HelixReplacePath string
}

// ModuleOptions configures module scaffolding.
type ModuleOptions struct {
	RootDir string
	Name    string
}

// ContextOptions configures DDD-light context scaffolding.
type ContextOptions struct {
	RootDir string
	Name    string
}

type appTemplateData struct {
	Name             string
	ModulePath       string
	HelixReplacePath string
	ExtraGoMod       string
}

type moduleTemplateData struct {
	PackageName string
	TypeName    string
	FolderName  string
}

type contextTemplateData struct {
	PackageName string
	TypeName    string
	FolderName  string
}

// NewApp creates a minimal Helix application under RootDir/Name.
func NewApp(opts Options) error {
	root := opts.RootDir
	if root == "" {
		root = "."
	}
	appName, err := normalizeAppName(opts.Name)
	if err != nil {
		return fmt.Errorf("helix new app: invalid name %q: %w", opts.Name, err)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("helix new app: resolve root: %w", err)
	}
	appDir, err := safeJoin(root, appName)
	if err != nil {
		return fmt.Errorf("helix new app %s: %w", appName, err)
	}
	if err := ensureEmptyOrMissingDir(appDir); err != nil {
		return fmt.Errorf("helix new app %s: %w", appName, err)
	}

	data := appTemplateData{
		Name:             appName,
		ModulePath:       appName,
		HelixReplacePath: filepath.ToSlash(opts.HelixReplacePath),
	}
	if opts.HelixReplacePath != "" {
		if goMod, err := os.ReadFile(filepath.Join(opts.HelixReplacePath, "go.mod")); err == nil {
			data.ExtraGoMod = extractRequireBlocks(string(goMod))
		}
	}
	files := map[string]string{
		"go.mod":                  renderTemplate(goModTemplate, data),
		"main.go":                 renderGoTemplate(mainTemplate, data),
		"config/application.yaml": renderTemplate(applicationConfigTemplate, data),
	}
	if opts.HelixReplacePath != "" {
		if goSum, err := os.ReadFile(filepath.Join(opts.HelixReplacePath, "go.sum")); err == nil {
			files["go.sum"] = string(goSum)
		}
	}
	for name, content := range files {
		path, err := safeJoin(appDir, name)
		if err != nil {
			return fmt.Errorf("helix new app %s: %w", appName, err)
		}
		if err := writeNewFile(path, []byte(content)); err != nil {
			return fmt.Errorf("helix new app %s: write %s: %w", appName, name, err)
		}
	}
	return nil
}

// GenerateModule creates a conventional Helix module under RootDir.
func GenerateModule(opts ModuleOptions) error {
	root := opts.RootDir
	if root == "" {
		root = "."
	}
	name, err := parseModuleName(opts.Name)
	if err != nil {
		return fmt.Errorf("helix generate module: invalid name %q: %w", opts.Name, err)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("helix generate module %s: resolve root: %w", opts.Name, err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("helix generate module %s: go.mod not found: %w", opts.Name, err)
		}
		return fmt.Errorf("helix generate module %s: stat go.mod: %w", opts.Name, err)
	}

	data := moduleTemplateData{
		PackageName: name.PackageName,
		TypeName:    name.TypeName,
		FolderName:  name.FolderName,
	}
	files := map[string]string{
		filepath.Join(name.FolderName, "repository.go"): renderGoTemplate(repositoryTemplate, data),
		filepath.Join(name.FolderName, "service.go"):    renderGoTemplate(serviceTemplate, data),
		filepath.Join(name.FolderName, "controller.go"): renderGoTemplate(controllerTemplate, data),
	}
	if err := writeAllFiles(root, "helix generate module "+opts.Name, files); err != nil {
		return err
	}
	return nil
}

// GenerateContext creates a DDD-light Helix context scaffold under RootDir.
func GenerateContext(opts ContextOptions) error {
	root := opts.RootDir
	if root == "" {
		root = "."
	}
	name, err := parseContextName(opts.Name)
	if err != nil {
		return fmt.Errorf("helix generate context: invalid name %q: %w", opts.Name, err)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("helix generate context %s: resolve root: %w", opts.Name, err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("helix generate context %s: go.mod not found: %w", opts.Name, err)
		}
		return fmt.Errorf("helix generate context %s: stat go.mod: %w", opts.Name, err)
	}

	data := contextTemplateData{
		PackageName: name.PackageName,
		TypeName:    name.TypeName,
		FolderName:  name.FolderName,
	}
	files := map[string]string{
		filepath.Join(name.FolderName, "api.go"):        renderGoTemplate(contextAPITemplate, data),
		filepath.Join(name.FolderName, "repository.go"): renderGoTemplate(contextRepositoryTemplate, data),
		filepath.Join(name.FolderName, "service.go"):    renderGoTemplate(contextServiceTemplate, data),
		filepath.Join(name.FolderName, "controller.go"): renderGoTemplate(contextControllerTemplate, data),
	}
	if err := writeAllFiles(root, "helix generate context "+opts.Name, files); err != nil {
		return err
	}
	return nil
}

func ensureEmptyOrMissingDir(path string) error {
	entries, err := os.ReadDir(path)
	if err == nil && len(entries) > 0 {
		return fmt.Errorf("directory %s is not empty", path)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func writeNewFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("refusing to overwrite existing file %s", filepath.Base(path))
		}
		return err
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

// writeAllFiles writes each entry into root, rolling back on any error.
func writeAllFiles(root, errPrefix string, files map[string]string) error {
	var written []string
	for name, content := range files {
		path, err := safeJoin(root, name)
		if err != nil {
			removeFiles(written)
			return fmt.Errorf("%s: %w", errPrefix, err)
		}
		if err := writeNewFile(path, []byte(content)); err != nil {
			removeFiles(written)
			return fmt.Errorf("%s: write %s: %w", errPrefix, name, err)
		}
		written = append(written, path)
	}
	return nil
}

func removeFiles(paths []string) {
	for _, p := range paths {
		_ = os.Remove(p)
	}
}

func safeJoin(root, name string) (string, error) {
	if name == "" || filepath.IsAbs(name) {
		return "", fmt.Errorf("invalid path %q", name)
	}
	cleanRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	joined := filepath.Join(cleanRoot, filepath.Clean(name))
	rel, err := filepath.Rel(cleanRoot, joined)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", fmt.Errorf("path %q escapes root %q", name, root)
	}
	return joined, nil
}

func renderTemplate(text string, data any) string {
	var buf bytes.Buffer
	if err := template.Must(template.New("scaffold").Parse(text)).Execute(&buf, data); err != nil {
		panic(fmt.Sprintf("scaffold template execution failed: %v", err))
	}
	return buf.String()
}

func renderGoTemplate(text string, data any) string {
	source := renderTemplate(text, data)
	formatted, err := format.Source([]byte(source))
	if err != nil {
		panic(fmt.Sprintf("scaffold: renderGoTemplate: format.Source failed: %v\nsource:\n%s", err, source))
	}
	return string(formatted)
}

func normalizeAppName(name string) (string, error) {
	if name == "" || name == "." || name == ".." || filepath.IsAbs(name) || strings.ContainsAny(name, `/\`) {
		return "", errInvalidName
	}
	runes := []rune(name)
	if !('a' <= runes[0] && runes[0] <= 'z') {
		return "", errInvalidName
	}
	for _, r := range runes[1:] {
		if r == '-' || unicode.IsDigit(r) || ('a' <= r && r <= 'z') {
			continue
		}
		return "", errInvalidName
	}
	if runes[len(runes)-1] == '-' {
		return "", errInvalidName
	}
	return name, nil
}

type parsedModuleName struct {
	FolderName  string
	PackageName string
	TypeName    string
}

func parseModuleName(name string) (parsedModuleName, error) {
	if name == "" || name == "." || name == ".." || filepath.IsAbs(name) || strings.ContainsAny(name, `/\_`) {
		return parsedModuleName{}, errInvalidName
	}
	var words []string
	var current strings.Builder
	for _, r := range name {
		switch {
		case r == '-':
			if current.Len() == 0 {
				return parsedModuleName{}, errInvalidName
			}
			words = append(words, current.String())
			current.Reset()
		case 'a' <= r && r <= 'z':
			current.WriteRune(r)
		case unicode.IsDigit(r):
			current.WriteRune(r)
		default:
			return parsedModuleName{}, errInvalidName
		}
	}
	if current.Len() == 0 {
		return parsedModuleName{}, errInvalidName
	}
	words = append(words, current.String())
	if len(words) == 0 || unicode.IsDigit([]rune(words[0])[0]) {
		return parsedModuleName{}, errInvalidName
	}

	base := strings.Join(words, "")
	folder := base
	if !strings.HasSuffix(folder, "s") {
		folder += "s"
	}
	typeName := exportedName(words)
	return parsedModuleName{
		FolderName:  folder,
		PackageName: folder,
		TypeName:    typeName,
	}, nil
}

func parseContextName(name string) (parsedModuleName, error) {
	parsed, err := parseModuleName(name)
	if err != nil {
		return parsedModuleName{}, err
	}
	parsed.TypeName = exportedSingularName(parsed.PackageName)
	return parsed, nil
}

func exportedSingularName(name string) string {
	singular := name
	switch {
	case strings.HasSuffix(singular, "ies") && len(singular) > len("ies"):
		singular = strings.TrimSuffix(singular, "ies") + "y"
	case strings.HasSuffix(singular, "s") && !strings.HasSuffix(singular, "ss") && len(singular) > 1:
		singular = strings.TrimSuffix(singular, "s")
	}
	return exportedName([]string{singular})
}

func exportedName(words []string) string {
	var b strings.Builder
	for _, word := range words {
		runes := []rune(word)
		if len(runes) == 0 {
			continue
		}
		b.WriteRune(unicode.ToUpper(runes[0]))
		b.WriteString(string(runes[1:]))
	}
	return b.String()
}

func extractRequireBlocks(goMod string) string {
	lines := strings.Split(goMod, "\n")
	var blocks []string
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if !strings.HasPrefix(line, "require") {
			continue
		}
		if line == "require (" {
			var block []string
			found := false
			for ; i < len(lines); i++ {
				block = append(block, lines[i])
				if strings.TrimSpace(lines[i]) == ")" {
					found = true
					break
				}
			}
			if !found {
				continue
			}
			blocks = append(blocks, strings.Join(block, "\n"))
			continue
		}
		blocks = append(blocks, lines[i])
	}
	if len(blocks) == 0 {
		return ""
	}
	return "\n" + strings.Join(blocks, "\n\n") + "\n"
}
