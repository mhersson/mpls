package plantuml

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

func TestInsertPlantumlDiagram_NoPlantUML(t *testing.T) {
	t.Parallel()

	input := `<p>Hello world</p><pre><code class="language-go">func main() {}</code></pre>`

	result, plantumls, err := InsertPlantumlDiagram(input, false, nil)
	require.NoError(t, err)

	assert.Equal(t, input, result)
	assert.Empty(t, plantumls)
}

func TestInsertPlantumlDiagram_NoStartuml(t *testing.T) {
	t.Parallel()

	// PlantUML code block without @startuml should be left as-is
	input := `<pre><code class="language-plantuml">just some text without startuml</code></pre>`

	result, plantumls, err := InsertPlantumlDiagram(input, false, nil)
	require.NoError(t, err)

	assert.Equal(t, input, result)
	assert.Empty(t, plantumls)
}

func TestInsertPlantumlDiagram_MultipleClasses(t *testing.T) {
	t.Parallel()

	// Code block with multiple classes including language-plantuml
	input := `<pre><code class="highlight language-plantuml line-numbers">@startuml
A -> B
@enduml</code></pre>`

	result, plantumls, err := InsertPlantumlDiagram(input, false, []Plantuml{
		{EncodedUML: Encode("@startuml\nA -> B\n@enduml"), Diagram: `<img src="diagram1">`},
	})
	require.NoError(t, err)

	expected := `<img src="diagram1">`
	assert.Equal(t, expected, result)
	assert.Len(t, plantumls, 1)
}

func TestInsertPlantumlDiagram_NestedCodeTag(t *testing.T) {
	t.Parallel()

	// PlantUML content containing </code> (the bug case the refactor addresses)
	// Note: In actual PlantUML, this might appear in a note or comment
	input := `<pre><code class="language-plantuml">@startuml
note "Use &lt;/code&gt;&lt;/pre&gt; to end blocks"
A -> B
@enduml</code></pre>`

	encodedUML := "@startuml\nnote \"Use </code></pre> to end blocks\"\nA -> B\n@enduml"

	result, plantumls, err := InsertPlantumlDiagram(input, false, []Plantuml{
		{EncodedUML: Encode(encodedUML), Diagram: `<img src="diagram-with-note">`},
	})
	require.NoError(t, err)

	expected := `<img src="diagram-with-note">`
	assert.Equal(t, expected, result)
	assert.Len(t, plantumls, 1)
}

func TestInsertPlantumlDiagram_MultipleDiagrams(t *testing.T) {
	t.Parallel()

	input := `<p>First diagram:</p>
<pre><code class="language-plantuml">@startuml
A -> B
@enduml</code></pre>
<p>Second diagram:</p>
<pre><code class="language-plantuml">@startuml
C -> D
@enduml</code></pre>`

	result, plantumls, err := InsertPlantumlDiagram(input, false, []Plantuml{
		{EncodedUML: Encode("@startuml\nA -> B\n@enduml"), Diagram: `<img src="diagram1">`},
		{EncodedUML: Encode("@startuml\nC -> D\n@enduml"), Diagram: `<img src="diagram2">`},
	})
	require.NoError(t, err)

	assert.Contains(t, result, `<img src="diagram1">`)
	assert.Contains(t, result, `<img src="diagram2">`)
	assert.Contains(t, result, "<p>First diagram:</p>")
	assert.Contains(t, result, "<p>Second diagram:</p>")
	assert.Len(t, plantumls, 2)
}

func TestInsertPlantumlDiagram_MixedCodeBlocks(t *testing.T) {
	t.Parallel()

	// Mix of PlantUML and regular code blocks
	input := `<pre><code class="language-go">func main() {}</code></pre>
<pre><code class="language-plantuml">@startuml
A -> B
@enduml</code></pre>
<pre><code class="language-python">print("hello")</code></pre>`

	result, plantumls, err := InsertPlantumlDiagram(input, false, []Plantuml{
		{EncodedUML: Encode("@startuml\nA -> B\n@enduml"), Diagram: `<img src="diagram1">`},
	})
	require.NoError(t, err)

	// Go code block should be preserved
	assert.Contains(t, result, `<pre><code class="language-go">func main() {}</code></pre>`)

	// PlantUML should be replaced
	assert.Contains(t, result, `<img src="diagram1">`)

	// Python code block should be preserved
	assert.Contains(t, result, `<pre><code class="language-python">print("hello")</code></pre>`)

	assert.Len(t, plantumls, 1)
}

func TestInsertPlantumlDiagram_HTMLEntities(t *testing.T) {
	t.Parallel()

	// PlantUML with HTML entities that need unescaping
	input := `<pre><code class="language-plantuml">@startuml
A -&gt; B : &quot;message&quot;
@enduml</code></pre>`

	expectedUML := "@startuml\nA -> B : \"message\"\n@enduml"

	result, plantumls, err := InsertPlantumlDiagram(input, false, []Plantuml{
		{EncodedUML: Encode(expectedUML), Diagram: `<img src="diagram1">`},
	})
	require.NoError(t, err)

	expected := `<img src="diagram1">`
	assert.Equal(t, expected, result)
	assert.Len(t, plantumls, 1)
}

func TestHasLanguagePlantuml(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		classes  string
		expected bool
	}{
		{"exact match", "language-plantuml", true},
		{"with prefix", "highlight language-plantuml", true},
		{"with suffix", "language-plantuml line-numbers", true},
		{"in middle", "highlight language-plantuml line-numbers", true},
		{"other language", "language-go", false},
		{"partial match", "language-plantuml-extra", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			attrs := []html.Attribute{{Key: "class", Val: tt.classes}}

			result := hasLanguagePlantuml(attrs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasLanguagePlantuml_NoClassAttr(t *testing.T) {
	t.Parallel()

	attrs := []html.Attribute{{Key: "id", Val: "some-id"}}
	assert.False(t, hasLanguagePlantuml(attrs))
}

func TestEncode(t *testing.T) {
	t.Parallel()

	// Test that encoding is deterministic
	uml := "@startuml\nA -> B\n@enduml"
	encoded1 := Encode(uml)
	encoded2 := Encode(uml)

	assert.Equal(t, encoded1, encoded2, "encoding should be deterministic")
	assert.NotEmpty(t, encoded1, "encoded string should not be empty")
}

func TestHasLanguagePlantuml_MultipleAttrs(t *testing.T) {
	t.Parallel()

	attrs := []html.Attribute{
		{Key: "id", Val: "code-block"},
		{Key: "class", Val: "language-plantuml"},
		{Key: "data-line", Val: "1"},
	}

	assert.True(t, hasLanguagePlantuml(attrs))
}

// Test that slices.Contains works as expected for the hasLanguagePlantuml helper.
func TestSlicesContains(t *testing.T) {
	t.Parallel()

	classes := strings.Fields("highlight language-plantuml line-numbers")
	assert.True(t, slices.Contains(classes, "language-plantuml"))
	assert.False(t, slices.Contains(classes, "language-plantuml-extra"))
}
