package generator

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func DetectProjectFacts(root string) (ProjectFacts, error) {
	root = filepath.Clean(root)
	switch {
	case exists(filepath.Join(root, "pyproject.toml")):
		return detectPython(root)
	case exists(filepath.Join(root, "package.json")):
		return detectNode(root)
	case exists(filepath.Join(root, "go.mod")):
		return detectGo(root)
	case exists(filepath.Join(root, "pom.xml")):
		return detectMaven(root)
	case exists(filepath.Join(root, "build.gradle.kts")) || exists(filepath.Join(root, "build.gradle")):
		return detectGradle(root)
	case exists(filepath.Join(root, "Cargo.toml")):
		return detectRust(root)
	default:
		return detectGeneric(root), nil
	}
}

func detectPython(root string) (ProjectFacts, error) {
	manifest := filepath.Join(root, "pyproject.toml")
	name := tomlScalar(tomlGet(manifest, "project", "name"))
	if name == "" {
		name = tomlScalar(tomlGet(manifest, "tool.poetry", "name"))
	}
	if name == "" {
		name = filepath.Base(root)
	}
	runtime := tomlScalar(tomlGet(manifest, "project", "requires-python"))
	if runtime == "" {
		runtime = tomlScalar(tomlGet(manifest, "tool.poetry.dependencies", "python"))
	}
	sourceRoot := strings.TrimSuffix(tomlScalar(tomlGet(manifest, "tool.setuptools.packages.find", "where")), "/")
	if sourceRoot == "" {
		if isDir(filepath.Join(root, "src")) {
			sourceRoot = "src"
		} else {
			sourceRoot = "."
		}
	}
	packageBase := root
	if sourceRoot != "." {
		packageBase = filepath.Join(root, filepath.FromSlash(sourceRoot))
	}
	return ProjectFacts{
		HasManifest:  true,
		ProjectName:  name,
		Language:     "Python",
		ManifestPath: "pyproject.toml",
		SourceRoot:   sourceRoot,
		PackageName:  detectPythonPackage(packageBase),
		Runtime:      runtime,
		BuildTool:    "pyproject",
	}, nil
}

func detectNode(root string) (ProjectFacts, error) {
	type packageJSON struct {
		Name           string            `json:"name"`
		PackageManager string            `json:"packageManager"`
		Engines        map[string]string `json:"engines"`
		Dependencies   map[string]any    `json:"dependencies"`
		DevDeps        map[string]any    `json:"devDependencies"`
	}
	var pkg packageJSON
	if err := readJSON(filepath.Join(root, "package.json"), &pkg); err != nil {
		return ProjectFacts{}, err
	}
	name := pkg.Name
	if name == "" {
		name = filepath.Base(root)
	}
	language := "JavaScript / Node"
	if exists(filepath.Join(root, "tsconfig.json")) {
		language = "TypeScript / Node"
	} else if _, ok := pkg.Dependencies["typescript"]; ok {
		language = "TypeScript / Node"
	} else if _, ok := pkg.DevDeps["typescript"]; ok {
		language = "TypeScript / Node"
	}
	sourceRoot := "."
	if isDir(filepath.Join(root, "src")) {
		sourceRoot = "src"
	}
	buildTool := pkg.PackageManager
	if buildTool == "" {
		buildTool = "npm-compatible"
	}
	return ProjectFacts{
		HasManifest:  true,
		ProjectName:  name,
		Language:     language,
		ManifestPath: "package.json",
		SourceRoot:   sourceRoot,
		PackageName:  name,
		Runtime:      pkg.Engines["node"],
		BuildTool:    buildTool,
	}, nil
}

func detectGo(root string) (ProjectFacts, error) {
	content, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ProjectFacts{}, err
	}
	var modulePath, runtime string
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			modulePath = strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
		if strings.HasPrefix(line, "go ") {
			runtime = strings.TrimSpace(strings.TrimPrefix(line, "go "))
		}
	}
	name := pathBase(modulePath)
	if name == "" {
		name = filepath.Base(root)
	}
	return ProjectFacts{
		HasManifest:  true,
		ProjectName:  name,
		Language:     "Go",
		ManifestPath: "go.mod",
		SourceRoot:   ".",
		Runtime:      runtime,
		BuildTool:    "go",
		ModulePath:   modulePath,
	}, nil
}

func detectMaven(root string) (ProjectFacts, error) {
	model, err := readPOM(filepath.Join(root, "pom.xml"))
	if err != nil {
		return ProjectFacts{}, err
	}
	artifactID := model.ArtifactID
	if artifactID == "" {
		artifactID = filepath.Base(root)
	}
	groupID := model.GroupID
	if groupID == "" {
		groupID = model.Parent.GroupID
	}
	packageName := artifactID
	if groupID != "" {
		packageName = groupID + "." + artifactID
	}
	packageName = strings.ReplaceAll(packageName, "-", ".")
	runtime := model.Properties["java.version"]
	if runtime == "" {
		runtime = model.Properties["maven.compiler.release"]
	}
	sourceRoot := "src/main/java"
	if isDir(filepath.Join(root, "src/main/kotlin")) {
		sourceRoot = "src/main/kotlin"
	}
	return ProjectFacts{
		HasManifest:  true,
		ProjectName:  artifactID,
		Language:     "Java / Kotlin",
		ManifestPath: "pom.xml",
		SourceRoot:   sourceRoot,
		PackageName:  packageName,
		Runtime:      runtime,
		BuildTool:    "maven",
	}, nil
}

func detectGradle(root string) (ProjectFacts, error) {
	manifest := "build.gradle"
	if exists(filepath.Join(root, "build.gradle.kts")) {
		manifest = "build.gradle.kts"
	}
	buildFile := filepath.Join(root, manifest)
	name := ""
	for _, settings := range []string{"settings.gradle.kts", "settings.gradle"} {
		content, err := os.ReadFile(filepath.Join(root, settings))
		if err == nil {
			name = firstRegexGroup(string(content), `rootProject[.]name[[:space:]]*=[[:space:]]*['"]([^'"]+)['"]`)
			break
		}
	}
	if name == "" {
		name = filepath.Base(root)
	}
	content, err := os.ReadFile(buildFile)
	if err != nil {
		return ProjectFacts{}, err
	}
	build := string(content)
	group := firstRegexGroup(build, `group[[:space:]]*=[[:space:]]*['"]([^'"]+)['"]`)
	language := "Java"
	if strings.Contains(build, "org.jetbrains.kotlin") || strings.Contains(build, "kotlin(") {
		language = "Kotlin"
	}
	runtime := firstRegexGroup(build, `JavaVersion[.]VERSION_([0-9]+)`)
	if runtime == "" {
		runtime = firstRegexGroup(build, `jvmToolchain[(]([0-9]+)[)]`)
	}
	packageName := name
	if group != "" {
		packageName = group + "." + name
	}
	return ProjectFacts{
		HasManifest:  true,
		ProjectName:  name,
		Language:     language + " / Gradle",
		ManifestPath: manifest,
		SourceRoot:   mapBool(language == "Kotlin", "src/main/kotlin", "src/main/java"),
		PackageName:  strings.ReplaceAll(packageName, "-", "."),
		Runtime:      runtime,
		BuildTool:    "gradle",
	}, nil
}

func detectRust(root string) (ProjectFacts, error) {
	manifest := filepath.Join(root, "Cargo.toml")
	name := tomlScalar(tomlGet(manifest, "package", "name"))
	if name == "" {
		name = filepath.Base(root)
	}
	edition := tomlScalar(tomlGet(manifest, "package", "edition"))
	runtime := ""
	if edition != "" {
		runtime = "edition " + edition
	}
	return ProjectFacts{
		HasManifest:  true,
		ProjectName:  name,
		Language:     "Rust",
		ManifestPath: "Cargo.toml",
		SourceRoot:   "src",
		PackageName:  strings.ReplaceAll(name, "-", "_"),
		Runtime:      runtime,
		BuildTool:    "cargo",
	}, nil
}

func detectGeneric(root string) ProjectFacts {
	sourceRoot := "."
	if isDir(filepath.Join(root, "src")) {
		sourceRoot = "src"
	}
	return ProjectFacts{ProjectName: filepath.Base(root), SourceRoot: sourceRoot}
}

func detectPythonPackage(root string) string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	var candidates []string
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if exists(filepath.Join(root, entry.Name(), "__init__.py")) {
			candidates = append(candidates, entry.Name())
		}
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	return ""
}

func tomlGet(file, section, key string) string {
	content, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	current := ""
	for _, raw := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(stripInlineComment(raw))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.Contains(line, "]") {
			current = strings.TrimSpace(strings.Trim(line, "[]"))
			continue
		}
		if current != section {
			continue
		}
		left, right, ok := strings.Cut(line, "=")
		if ok && strings.TrimSpace(left) == key {
			return strings.TrimSpace(right)
		}
	}
	return ""
}

func tomlScalar(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		inner := strings.TrimSuffix(strings.TrimPrefix(value, "["), "]")
		value = strings.TrimSpace(strings.Split(inner, ",")[0])
	}
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '\'' || first == '"') && first == last {
			value = value[1 : len(value)-1]
		}
	}
	return value
}

func stripInlineComment(value string) string {
	inSingle, inDouble := false, false
	for i, r := range value {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				return value[:i]
			}
		}
	}
	return value
}

type pomModel struct {
	XMLName    xml.Name `xml:"project"`
	GroupID    string   `xml:"groupId"`
	ArtifactID string   `xml:"artifactId"`
	Parent     struct {
		GroupID string `xml:"groupId"`
	} `xml:"parent"`
	Properties map[string]string
}

func readPOM(file string) (pomModel, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return pomModel{}, err
	}
	type rawProperties struct {
		Items []struct {
			XMLName xml.Name
			Value   string `xml:",chardata"`
		} `xml:",any"`
	}
	type rawModel struct {
		GroupID    string `xml:"groupId"`
		ArtifactID string `xml:"artifactId"`
		Parent     struct {
			GroupID string `xml:"groupId"`
		} `xml:"parent"`
		Properties rawProperties `xml:"properties"`
	}
	var raw rawModel
	if err := xml.Unmarshal(content, &raw); err != nil {
		return pomModel{}, err
	}
	props := map[string]string{}
	for _, item := range raw.Properties.Items {
		props[item.XMLName.Local] = strings.TrimSpace(item.Value)
	}
	return pomModel{GroupID: strings.TrimSpace(raw.GroupID), ArtifactID: strings.TrimSpace(raw.ArtifactID), Parent: raw.Parent, Properties: props}, nil
}

func readJSON(path string, target any) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(content, target); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func firstRegexGroup(value, pattern string) string {
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(value)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func pathBase(value string) string {
	value = strings.TrimSuffix(value, "/")
	if value == "" {
		return ""
	}
	if i := strings.LastIndex(value, "/"); i >= 0 {
		return value[i+1:]
	}
	return value
}

func mapBool(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}
