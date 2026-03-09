package mpls

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDocumentRegistry_Register(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		uri     string
		content string
	}{
		{
			name:    "simple registration",
			uri:     "file:///home/user/doc.md",
			content: "# Hello",
		},
		{
			name:    "with special characters",
			uri:     "file:///home/user/my%20doc.md",
			content: "# Test",
		},
		{
			name:    "empty content",
			uri:     "file:///empty.md",
			content: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &DocumentRegistry{
				docs:          make(map[string]*DocumentState),
				workspaceRoot: "/home/user",
			}

			state := &DocumentState{Content: tt.content}
			r.Register(tt.uri, state)

			// Verify URI was set on state
			assert.Equal(t, tt.uri, state.URI)

			// Verify LastModified was set
			assert.False(t, state.LastModified.IsZero())

			// Verify state is retrievable
			got, exists := r.Get(tt.uri)
			require.True(t, exists)
			assert.Equal(t, tt.content, got.Content)
		})
	}
}

func TestDocumentRegistry_Update(t *testing.T) {
	t.Parallel()

	t.Run("updates existing document", func(t *testing.T) {
		t.Parallel()

		r := &DocumentRegistry{
			docs:          make(map[string]*DocumentState),
			workspaceRoot: "/home/user",
		}

		uri := "file:///test.md"
		state := &DocumentState{Content: "original"}
		r.Register(uri, state)

		originalTime := state.LastModified

		// Small delay to ensure time difference
		time.Sleep(time.Millisecond)

		r.Update(uri, "updated content")

		got, exists := r.Get(uri)
		require.True(t, exists)
		assert.Equal(t, "updated content", got.Content)
		assert.True(t, got.LastModified.After(originalTime))
	})

	t.Run("no-op for non-existent document", func(t *testing.T) {
		t.Parallel()

		r := &DocumentRegistry{
			docs:          make(map[string]*DocumentState),
			workspaceRoot: "/home/user",
		}

		// Should not panic or create new entry
		r.Update("file:///nonexistent.md", "content")

		_, exists := r.Get("file:///nonexistent.md")
		assert.False(t, exists)
	})
}

func TestDocumentRegistry_Get(t *testing.T) {
	t.Parallel()

	t.Run("returns existing document", func(t *testing.T) {
		t.Parallel()

		r := &DocumentRegistry{
			docs:          make(map[string]*DocumentState),
			workspaceRoot: "/home/user",
		}

		uri := "file:///test.md"
		state := &DocumentState{Content: "test content"}
		r.Register(uri, state)

		got, exists := r.Get(uri)
		require.True(t, exists)
		assert.Equal(t, "test content", got.Content)
	})

	t.Run("returns false for non-existent", func(t *testing.T) {
		t.Parallel()

		r := &DocumentRegistry{
			docs:          make(map[string]*DocumentState),
			workspaceRoot: "/home/user",
		}

		_, exists := r.Get("file:///nonexistent.md")
		assert.False(t, exists)
	})
}

func TestDocumentRegistry_Remove(t *testing.T) {
	t.Parallel()

	t.Run("removes existing document", func(t *testing.T) {
		t.Parallel()

		r := &DocumentRegistry{
			docs:          make(map[string]*DocumentState),
			workspaceRoot: "/home/user",
		}

		uri := "file:///test.md"
		state := &DocumentState{Content: "test"}
		r.Register(uri, state)

		r.Remove(uri)

		_, exists := r.Get(uri)
		assert.False(t, exists)
	})

	t.Run("no-op for non-existent document", func(t *testing.T) {
		t.Parallel()

		r := &DocumentRegistry{
			docs:          make(map[string]*DocumentState),
			workspaceRoot: "/home/user",
		}

		// Should not panic
		r.Remove("file:///nonexistent.md")
		assert.True(t, r.IsEmpty())
	})
}

func TestDocumentRegistry_IsEmpty(t *testing.T) {
	t.Parallel()

	t.Run("true when empty", func(t *testing.T) {
		t.Parallel()

		r := &DocumentRegistry{
			docs:          make(map[string]*DocumentState),
			workspaceRoot: "/home/user",
		}

		assert.True(t, r.IsEmpty())
	})

	t.Run("false when has documents", func(t *testing.T) {
		t.Parallel()

		r := &DocumentRegistry{
			docs:          make(map[string]*DocumentState),
			workspaceRoot: "/home/user",
		}

		r.Register("file:///test.md", &DocumentState{})
		assert.False(t, r.IsEmpty())
	})

	t.Run("true after removing all documents", func(t *testing.T) {
		t.Parallel()

		r := &DocumentRegistry{
			docs:          make(map[string]*DocumentState),
			workspaceRoot: "/home/user",
		}

		uri := "file:///test.md"
		r.Register(uri, &DocumentState{})
		r.Remove(uri)

		assert.True(t, r.IsEmpty())
	})
}

func TestDocumentRegistry_GetMostRecentDocument(t *testing.T) {
	t.Parallel()

	t.Run("returns nil when empty", func(t *testing.T) {
		t.Parallel()

		r := &DocumentRegistry{
			docs:          make(map[string]*DocumentState),
			workspaceRoot: "/home/user",
		}

		assert.Nil(t, r.GetMostRecentDocument())
	})

	t.Run("returns single document", func(t *testing.T) {
		t.Parallel()

		r := &DocumentRegistry{
			docs:          make(map[string]*DocumentState),
			workspaceRoot: "/home/user",
		}

		state := &DocumentState{Content: "only doc"}
		r.Register("file:///only.md", state)

		got := r.GetMostRecentDocument()
		require.NotNil(t, got)
		assert.Equal(t, "only doc", got.Content)
	})

	t.Run("returns most recently modified", func(t *testing.T) {
		t.Parallel()

		r := &DocumentRegistry{
			docs:          make(map[string]*DocumentState),
			workspaceRoot: "/home/user",
		}

		// Register first document
		r.Register("file:///first.md", &DocumentState{Content: "first"})

		// Small delay to ensure different timestamps
		time.Sleep(time.Millisecond)

		// Register second document
		r.Register("file:///second.md", &DocumentState{Content: "second"})

		got := r.GetMostRecentDocument()
		require.NotNil(t, got)
		assert.Equal(t, "second", got.Content)
	})

	t.Run("returns updated document as most recent", func(t *testing.T) {
		t.Parallel()

		r := &DocumentRegistry{
			docs:          make(map[string]*DocumentState),
			workspaceRoot: "/home/user",
		}

		// Register two documents
		r.Register("file:///first.md", &DocumentState{Content: "first"})

		time.Sleep(time.Millisecond)

		r.Register("file:///second.md", &DocumentState{Content: "second"})

		time.Sleep(time.Millisecond)

		// Update first document
		r.Update("file:///first.md", "first updated")

		got := r.GetMostRecentDocument()
		require.NotNil(t, got)
		assert.Equal(t, "first updated", got.Content)
	})
}

func TestDocumentRegistry_GetRelativePath(t *testing.T) {
	t.Parallel()

	// Skip path-dependent tests on Windows
	if runtime.GOOS == "windows" {
		t.Skip("skipping path tests on Windows")
	}

	tests := []struct {
		name          string
		workspaceRoot string
		uri           string
		expected      string
	}{
		{
			name:          "file in workspace root",
			workspaceRoot: "/home/user/project",
			uri:           "file:///home/user/project/README.md",
			expected:      "/README.md",
		},
		{
			name:          "file in subdirectory",
			workspaceRoot: "/home/user/project",
			uri:           "file:///home/user/project/docs/guide.md",
			expected:      "/docs/guide.md",
		},
		{
			name:          "file outside workspace",
			workspaceRoot: "/home/user/project",
			uri:           "file:///home/other/doc.md",
			expected:      "",
		},
		{
			name:          "deeply nested file",
			workspaceRoot: "/workspace",
			uri:           "file:///workspace/a/b/c/d/file.md",
			expected:      "/a/b/c/d/file.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &DocumentRegistry{
				docs:          make(map[string]*DocumentState),
				workspaceRoot: tt.workspaceRoot,
			}

			result := r.GetRelativePath(tt.uri)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDocumentRegistry_GetFileURI(t *testing.T) {
	t.Parallel()

	// Skip path-dependent tests on Windows
	if runtime.GOOS == "windows" {
		t.Skip("skipping path tests on Windows")
	}

	t.Run("returns registered URI when document exists", func(t *testing.T) {
		t.Parallel()

		r := &DocumentRegistry{
			docs:          make(map[string]*DocumentState),
			workspaceRoot: "/home/user/project",
		}

		uri := "file:///home/user/project/docs/readme.md"
		r.Register(uri, &DocumentState{Content: "test"})

		result := r.GetFileURI("/docs/readme.md")
		assert.Equal(t, uri, result)
	})

	t.Run("constructs file URI when not in registry", func(t *testing.T) {
		t.Parallel()

		r := &DocumentRegistry{
			docs:          make(map[string]*DocumentState),
			workspaceRoot: "/home/user/project",
		}

		result := r.GetFileURI("/docs/other.md")
		assert.Equal(t, "file:///home/user/project/docs/other.md", result)
	})

	t.Run("handles relative path without leading slash", func(t *testing.T) {
		t.Parallel()

		r := &DocumentRegistry{
			docs:          make(map[string]*DocumentState),
			workspaceRoot: "/home/user/project",
		}

		result := r.GetFileURI("docs/file.md")
		assert.Equal(t, "file:///home/user/project/docs/file.md", result)
	})
}

func TestDocumentRegistry_GetWorkspaceRoot(t *testing.T) {
	t.Parallel()

	r := &DocumentRegistry{
		docs:          make(map[string]*DocumentState),
		workspaceRoot: "/some/path",
	}

	assert.Equal(t, "/some/path", r.GetWorkspaceRoot())
}
