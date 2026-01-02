package runner

import (
	"bufio"
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"text/template"

	"github.com/google/uuid"
)

// TemplateEngine handles parsing and executing templates
type TemplateEngine struct {
	fileCache map[string][]string
	mu        sync.RWMutex
	funcMap   template.FuncMap
}

// TemplateData is passed to the execution context
type TemplateData struct {
	UserID string
	UUID   string
}

// NewTemplateEngine initializes the engine and its functions
func NewTemplateEngine() *TemplateEngine {
	e := &TemplateEngine{
		fileCache: make(map[string][]string),
	}

	e.funcMap = template.FuncMap{
		"randomInt":    e.randomInt,
		"randomUUID":   e.randomUUID,
		"randomChoice": e.randomChoice,
		"randomLine":   e.randomLine,
		"uuid":         e.randomUUID, // Alias
	}

	return e
}

// Preprocess converts simple variables {{userID}} to Go template syntax {{.UserID}}
func (e *TemplateEngine) Preprocess(input string) string {
	s := input
	// Replace "naked" variables with dot-notation for struct access
	// We use a specific replacement to avoid breaking if user actually wrote {{.UserID}}
	s = strings.ReplaceAll(s, "{{userID}}", "{{.UserID}}")
	s = strings.ReplaceAll(s, "{{uuid}}", "{{.UUID}}")
	s = strings.ReplaceAll(s, "{{requestID}}", "{{.UUID}}")
	return s
}

// Parse creates a new template with the engine's functions
func (e *TemplateEngine) Parse(name, text string) (*template.Template, error) {
	// Pre-convert known variables
	readyText := e.Preprocess(text)
	return template.New(name).Funcs(e.funcMap).Parse(readyText)
}

// Execute runs the template with data
func (e *TemplateEngine) Execute(t *template.Template, data TemplateData) (string, error) {
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// --- Functions ---

func (e *TemplateEngine) randomInt(min, max int) int {
	return rand.Intn(max-min) + min
}

func (e *TemplateEngine) randomUUID() string {
	return uuid.New().String()
}

func (e *TemplateEngine) randomChoice(choices ...string) string {
	if len(choices) == 0 {
		return ""
	}
	return choices[rand.Intn(len(choices))]
}

func (e *TemplateEngine) randomLine(filename string) (string, error) {
	e.mu.RLock()
	lines, ok := e.fileCache[filename]
	e.mu.RUnlock()

	if ok {
		if len(lines) == 0 {
			return "", nil
		}
		return lines[rand.Intn(len(lines))], nil
	}

	// Load file (Lazy load)
	e.mu.Lock()
	defer e.mu.Unlock()

	// Double check
	if lines, ok = e.fileCache[filename]; ok {
		if len(lines) == 0 {
			return "", nil
		}
		return lines[rand.Intn(len(lines))], nil
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("failed to read file '%s': %w", filename, err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(content))
	var loaded []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			loaded = append(loaded, line)
		}
	}

	e.fileCache[filename] = loaded
	if len(loaded) == 0 {
		return "", nil
	}

	return loaded[rand.Intn(len(loaded))], nil
}
