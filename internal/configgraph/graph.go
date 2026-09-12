// Package configgraph loads Ghostty configuration files without flattening
// away their source documents. It keeps effective assignments and include
// diagnostics tied to the file and line that produced them.
package configgraph

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/thongntit/ghostty-config-tui/internal/configdoc"
)

const (
	defaultMaxFileSize  int64 = 1 << 20
	defaultMaxFiles           = 128
	defaultMaxDepth           = 32
	defaultMaxTotalSize int64 = 16 << 20
)

// Severity identifies the importance of a graph diagnostic.
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Diagnostic describes a problem or noteworthy condition while loading the
// graph. A diagnostic does not expose file contents.
type Diagnostic struct {
	Severity Severity
	Path     string
	Line     int
	Message  string
}

// File is one lossless source document in the graph. The loader returns the
// original document; callers may provide draft documents through
// WithDocuments without flattening the graph.
type File struct {
	Path     string
	Document configdoc.Document
}

// Include describes one config-file edge in source order.
type Include struct {
	SourcePath   string
	Line         int
	Value        string
	Optional     bool
	ResolvedPath string
	Loaded       bool
}

// Assignment is one effective assignment with source provenance. Assignments
// are ordered exactly as Ghostty applies them.
type Assignment struct {
	Key        string
	Value      string
	Empty      bool
	Path       string
	Line       int
	RawLine    string
	SourceFile int
}

// Graph contains all loaded roots, recursively included files, effective
// assignments, include edges, and diagnostics.
type Graph struct {
	Roots        []string
	Files        []File
	Assignments  []Assignment
	Includes     []Include
	Diagnostics  []Diagnostic
	fileIdentity map[string]int
}

// LoadOptions bounds graph traversal. Zero values use conservative defaults.
type LoadOptions struct {
	MaxFileSize  int64
	MaxFiles     int
	MaxDepth     int
	MaxTotalSize int64
}

func (o LoadOptions) withDefaults() LoadOptions {
	if o.MaxFileSize <= 0 {
		o.MaxFileSize = defaultMaxFileSize
	}
	if o.MaxFiles <= 0 {
		o.MaxFiles = defaultMaxFiles
	}
	if o.MaxDepth <= 0 {
		o.MaxDepth = defaultMaxDepth
	}
	if o.MaxTotalSize <= 0 {
		o.MaxTotalSize = defaultMaxTotalSize
	}
	return o
}

// Load reads one or more root config files and recursively follows their
// config-file directives. Root failures are returned as errors; include
// failures are retained as diagnostics so the user can still inspect the
// usable portion of a config.
func Load(roots []string) (Graph, error) {
	return LoadWithOptions(roots, LoadOptions{})
}

// LoadWithOptions is Load with explicit traversal limits.
func LoadWithOptions(roots []string, options LoadOptions) (Graph, error) {
	if len(roots) == 0 {
		return Graph{}, fmt.Errorf("at least one Ghostty config root is required")
	}

	options = options.withDefaults()
	graph := Graph{
		Roots:        append([]string(nil), roots...),
		fileIdentity: make(map[string]int),
	}
	loader := loader{graph: &graph, options: options, active: make(map[string]bool)}
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			return Graph{}, fmt.Errorf("Ghostty config root must not be empty")
		}
		if _, err := loader.visit(root, false, "", 0, 0); err != nil {
			return graph, err
		}
	}
	return graph, nil
}

// AssignmentsFor returns effective assignments for key in application order.
func (g Graph) AssignmentsFor(key string) []Assignment {
	assignments := make([]Assignment, 0)
	for _, assignment := range g.Assignments {
		if assignment.Key == key {
			assignments = append(assignments, assignment)
		}
	}
	return assignments
}

// Effective returns the last assignment for key, matching Ghostty's scalar
// precedence behavior. Repeatable callers should use AssignmentsFor.
func (g Graph) Effective(key string) (Assignment, bool) {
	for index := len(g.Assignments) - 1; index >= 0; index-- {
		if g.Assignments[index].Key == key {
			return g.Assignments[index], true
		}
	}
	return Assignment{}, false
}

// DocumentFor finds the first source document whose displayed path matches
// path. It intentionally does not resolve symlinks for presentation.
func (g Graph) DocumentFor(path string) (configdoc.Document, bool) {
	for _, file := range g.Files {
		if file.Path == path {
			return file.Document, true
		}
	}
	if index, ok := g.fileIdentity[canonicalPath(path)]; ok && index >= 0 && index < len(g.Files) {
		return g.Files[index].Document, true
	}
	return configdoc.Document{}, false
}

// WithDocuments returns a graph-shaped view using replacement documents for
// the loaded files. It replays the current root/include traversal so
// effective assignments, include edges, and diagnostics reflect an in-memory draft.
// Unknown or newly referenced include files are not loaded here; the next
// persisted reload is responsible for discovering them and reporting any
// diagnostics.
func (g Graph) WithDocuments(documents map[string]configdoc.Document) Graph {
	clone := g
	clone.Files = make([]File, len(g.Files))
	for index, file := range g.Files {
		clone.Files[index] = File{Path: file.Path, Document: file.Document}
		if document, ok := documents[file.Path]; ok {
			clone.Files[index].Document = document
			continue
		}
		canonical := canonicalPath(file.Path)
		for path, document := range documents {
			if canonicalPath(path) == canonical {
				clone.Files[index].Document = document
				break
			}
		}
	}
	clone.Assignments = nil
	clone.rebuildAssignments()
	return clone
}

func (g *Graph) rebuildAssignments() {
	g.Assignments = nil
	g.Includes = nil
	g.Diagnostics = nil
	byPath := make(map[string]int, len(g.Files)*2)
	for index, file := range g.Files {
		byPath[file.Path] = index
		byPath[canonicalPath(file.Path)] = index
	}
	active := make(map[int]bool, len(g.Files))
	stack := make([]string, 0, len(g.Files))
	var visit func(string) bool
	visit = func(path string) bool {
		index, ok := byPath[path]
		if !ok {
			index, ok = byPath[canonicalPath(path)]
		}
		if !ok || active[index] {
			return false
		}
		active[index] = true
		stack = append(stack, canonicalPath(path))
		file := g.Files[index]
		for lineIndex, node := range file.Document.Nodes {
			if node.Kind != configdoc.AssignmentNode {
				continue
			}
			line := lineIndex + 1
			if node.Key == "config-file" {
				request, err := parseInclude(node.Value)
				if err != nil {
					g.Diagnostics = append(g.Diagnostics, Diagnostic{
						Severity: SeverityError,
						Path:     file.Path,
						Line:     line,
						Message:  "invalid config-file value: " + err.Error(),
					})
					continue
				}
				resolved := resolveInclude(file.Path, request.value)
				includeIndex := len(g.Includes)
				g.Includes = append(g.Includes, Include{
					SourcePath:   file.Path,
					Line:         line,
					Value:        request.value,
					Optional:     request.optional,
					ResolvedPath: resolved,
				})
				targetIndex, exists := byPath[resolved]
				if !exists {
					targetIndex, exists = byPath[canonicalPath(resolved)]
				}
				if exists {
					if active[targetIndex] {
						g.Diagnostics = append(g.Diagnostics, Diagnostic{
							Severity: SeverityWarning,
							Path:     resolved,
							Line:     line,
							Message:  fmt.Sprintf("config-file cycle ignored: %s", strings.Join(append(stack, canonicalPath(resolved)), " -> ")),
						})
					} else {
						g.Includes[includeIndex].Loaded = visit(resolved)
					}
				} else {
					severity := SeverityError
					if request.optional {
						severity = SeverityInfo
					}
					g.Diagnostics = append(g.Diagnostics, Diagnostic{
						Severity: severity,
						Path:     file.Path,
						Line:     line,
						Message:  fmt.Sprintf("config-file %q: file is not part of the loaded graph", resolved),
					})
				}
				continue
			}
			g.Assignments = append(g.Assignments, Assignment{
				Key:        node.Key,
				Value:      node.Value,
				Empty:      node.Value == "",
				Path:       file.Path,
				Line:       line,
				RawLine:    strings.TrimSuffix(strings.TrimSuffix(string(node.Raw), "\n"), "\r"),
				SourceFile: index,
			})
		}
		active[index] = false
		stack = stack[:len(stack)-1]
		return true
	}
	for _, root := range g.Roots {
		visit(root)
	}
}

// ErrorCount returns the number of error diagnostics in the graph.
func (g Graph) ErrorCount() int {
	count := 0
	for _, diagnostic := range g.Diagnostics {
		if diagnostic.Severity == SeverityError {
			count++
		}
	}
	return count
}

type loader struct {
	graph      *Graph
	options    LoadOptions
	active     map[string]bool
	stack      []string
	totalBytes int64
}

func (l *loader) visit(path string, optional bool, sourcePath string, sourceLine, depth int) (bool, error) {
	if depth > l.options.MaxDepth {
		return l.includeFailure(path, optional, sourcePath, sourceLine, fmt.Sprintf("include depth exceeds %d", l.options.MaxDepth))
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if sourcePath == "" && !optional {
				return false, fmt.Errorf("stat Ghostty config root %q: %w", path, err)
			}
			return l.includeFailure(path, optional, sourcePath, sourceLine, "file does not exist")
		}
		if sourcePath == "" {
			return false, fmt.Errorf("stat Ghostty config root %q: %w", path, err)
		}
		return l.includeFailure(path, optional, sourcePath, sourceLine, "file could not be inspected")
	}
	if !info.Mode().IsRegular() {
		message := "path is not a regular file"
		if sourcePath == "" {
			return false, fmt.Errorf("Ghostty config root is not a regular file: %q", path)
		}
		return l.includeFailure(path, optional, sourcePath, sourceLine, message)
	}

	identity := canonicalPath(path)
	if l.active[identity] {
		l.graph.Diagnostics = append(l.graph.Diagnostics, Diagnostic{
			Severity: SeverityWarning,
			Path:     path,
			Line:     sourceLine,
			Message:  fmt.Sprintf("config-file cycle ignored: %s", strings.Join(append(l.stack, identity), " -> ")),
		})
		return false, nil
	}

	fileIndex, exists := l.graph.fileIdentity[identity]
	if !exists {
		document, err := l.readDocument(path, info.Size())
		if err != nil {
			if sourcePath == "" {
				return false, err
			}
			return l.includeFailure(path, optional, sourcePath, sourceLine, err.Error())
		}
		if len(l.graph.Files) >= l.options.MaxFiles {
			if sourcePath == "" {
				return false, fmt.Errorf("config graph exceeds %d files", l.options.MaxFiles)
			}
			return l.includeFailure(path, optional, sourcePath, sourceLine, fmt.Sprintf("file count exceeds %d", l.options.MaxFiles))
		}
		fileIndex = len(l.graph.Files)
		l.graph.fileIdentity[identity] = fileIndex
		l.graph.Files = append(l.graph.Files, File{Path: path, Document: document})
	}

	l.active[identity] = true
	l.stack = append(l.stack, identity)
	file := l.graph.Files[fileIndex]
	includeRequests := make([]includeRequest, 0)
	for index, node := range file.Document.Nodes {
		if node.Kind != configdoc.AssignmentNode {
			continue
		}
		line := index + 1
		if node.Key == "config-file" {
			request, err := parseInclude(node.Value)
			if err != nil {
				l.graph.Diagnostics = append(l.graph.Diagnostics, Diagnostic{
					Severity: SeverityError,
					Path:     path,
					Line:     line,
					Message:  "invalid config-file value: " + err.Error(),
				})
				continue
			}
			includeRequests = append(includeRequests, includeRequest{
				value:    request.value,
				optional: request.optional,
				line:     line,
			})
			continue
		}
		l.graph.Assignments = append(l.graph.Assignments, Assignment{
			Key:        node.Key,
			Value:      node.Value,
			Empty:      node.Value == "",
			Path:       path,
			Line:       line,
			RawLine:    strings.TrimSuffix(strings.TrimSuffix(string(node.Raw), "\n"), "\r"),
			SourceFile: fileIndex,
		})
	}

	for _, request := range includeRequests {
		resolved := resolveInclude(path, request.value)
		includeIndex := len(l.graph.Includes)
		l.graph.Includes = append(l.graph.Includes, Include{
			SourcePath:   path,
			Line:         request.line,
			Value:        request.value,
			Optional:     request.optional,
			ResolvedPath: resolved,
		})
		loaded, err := l.visit(resolved, request.optional, path, request.line, depth+1)
		if err != nil {
			l.active[identity] = false
			l.stack = l.stack[:len(l.stack)-1]
			return false, err
		}
		l.graph.Includes[includeIndex].Loaded = loaded
	}

	l.active[identity] = false
	l.stack = l.stack[:len(l.stack)-1]
	return true, nil
}

func (l *loader) readDocument(path string, size int64) (configdoc.Document, error) {
	if size > l.options.MaxFileSize {
		return configdoc.Document{}, fmt.Errorf("config file is larger than %d bytes: %q", l.options.MaxFileSize, path)
	}
	if l.totalBytes+size > l.options.MaxTotalSize {
		return configdoc.Document{}, fmt.Errorf("config graph exceeds %d total bytes", l.options.MaxTotalSize)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return configdoc.Document{}, fmt.Errorf("read Ghostty config %q: %w", path, err)
	}
	if !utf8.Valid(data) {
		return configdoc.Document{}, fmt.Errorf("config file is not valid UTF-8: %q", path)
	}
	l.totalBytes += int64(len(data))
	return configdoc.Parse(data), nil
}

func (l *loader) includeFailure(path string, optional bool, sourcePath string, line int, message string) (bool, error) {
	severity := SeverityError
	if optional {
		severity = SeverityInfo
	}
	l.graph.Diagnostics = append(l.graph.Diagnostics, Diagnostic{
		Severity: severity,
		Path:     sourcePath,
		Line:     line,
		Message:  fmt.Sprintf("config-file %q: %s", path, message),
	})
	return false, nil
}

type includeRequest struct {
	value    string
	optional bool
	line     int
}

type parsedInclude struct {
	value    string
	optional bool
}

func parseInclude(raw string) (parsedInclude, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return parsedInclude{}, fmt.Errorf("path is empty")
	}
	quoted := strings.HasPrefix(raw, "\"") || strings.HasSuffix(raw, "\"")
	value := raw
	if quoted {
		if len(raw) < 2 || !strings.HasPrefix(raw, "\"") || !strings.HasSuffix(raw, "\"") {
			return parsedInclude{}, fmt.Errorf("quoted path is not closed")
		}
		unquoted, err := strconv.Unquote(raw)
		if err != nil {
			return parsedInclude{}, fmt.Errorf("decode quoted path: %w", err)
		}
		value = unquoted
	}
	optional := !quoted && strings.HasPrefix(value, "?")
	if optional {
		value = strings.TrimPrefix(value, "?")
	}
	if value == "" {
		return parsedInclude{}, fmt.Errorf("path is empty")
	}
	return parsedInclude{value: value, optional: optional}, nil
}

func resolveInclude(sourcePath, includePath string) string {
	if filepath.IsAbs(includePath) {
		return filepath.Clean(includePath)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(sourcePath), includePath))
}

func canonicalPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = filepath.Clean(path)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}
