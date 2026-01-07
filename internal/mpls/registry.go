package mpls

import (
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mhersson/mpls/internal/previewserver"
	"github.com/mhersson/mpls/pkg/parser"
	"github.com/mhersson/mpls/pkg/plantuml"
)

type DocumentState struct {
	URI          string
	Content      string
	HTML         string
	Meta         map[string]any
	PlantUMLs    []plantuml.Plantuml
	LastModified time.Time
}

type DocumentRegistry struct {
	docs              map[string]*DocumentState
	workspaceRoot     string
	firstPreviewShown bool
	mutex             sync.RWMutex
}

var documentRegistry *DocumentRegistry

func InitializeDocumentRegistry(wsRoot string) {
	documentRegistry = &DocumentRegistry{
		docs:              make(map[string]*DocumentState),
		workspaceRoot:     wsRoot,
		firstPreviewShown: false,
	}
}

func (r *DocumentRegistry) Register(uri string, state *DocumentState) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	state.URI = uri
	state.LastModified = time.Now()
	r.docs[uri] = state
}

func (r *DocumentRegistry) Update(uri string, content string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if doc, exists := r.docs[uri]; exists {
		doc.Content = content
		doc.LastModified = time.Now()
	}
}

func (r *DocumentRegistry) Get(uri string) (*DocumentState, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	doc, exists := r.docs[uri]

	return doc, exists
}

func (r *DocumentRegistry) GetRelativePath(uri string) string {
	normalizedURI := parser.NormalizePath(uri)
	normalizedRoot := r.workspaceRoot

	if !strings.HasPrefix(normalizedURI, normalizedRoot) {
		return ""
	}

	relativePath, err := filepath.Rel(normalizedRoot, normalizedURI)
	if err != nil {
		return ""
	}

	// Ensure it starts with /
	if !strings.HasPrefix(relativePath, "/") {
		relativePath = "/" + relativePath
	}

	return relativePath
}

func (r *DocumentRegistry) GetFileURI(relativePath string) string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// Remove leading slash if present
	relativePath = strings.TrimPrefix(relativePath, "/")

	// Construct absolute path
	absolutePath := filepath.Join(r.workspaceRoot, relativePath)

	// Look for matching document
	for uri := range r.docs {
		normalizedURI := parser.NormalizePath(uri)
		if normalizedURI == absolutePath {
			return uri
		}
	}

	// If not in registry, construct file:// URI
	if !strings.HasPrefix(absolutePath, "file://") {
		return "file://" + absolutePath
	}

	return absolutePath
}

func (r *DocumentRegistry) MarkFirstPreviewShown() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.firstPreviewShown = true
}

func (r *DocumentRegistry) ShouldAutoOpen() bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// Always auto-open if OpenBrowserOnStartup is true (i.e., --no-auto is false)
	if previewserver.OpenBrowserOnStartup {
		return true
	}

	// If --no-auto is set, only auto-open after first preview shown
	return r.firstPreviewShown
}

func (r *DocumentRegistry) GetWorkspaceRoot() string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.workspaceRoot
}

func (r *DocumentRegistry) GetMostRecentDocument() *DocumentState {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var mostRecent *DocumentState

	var mostRecentTime time.Time

	for _, doc := range r.docs {
		if doc.LastModified.After(mostRecentTime) {
			mostRecent = doc
			mostRecentTime = doc.LastModified
		}
	}

	return mostRecent
}

func (r *DocumentRegistry) Remove(uri string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	delete(r.docs, uri)
}

func (r *DocumentRegistry) IsEmpty() bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return len(r.docs) == 0
}
