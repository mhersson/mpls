package plantuml

import (
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestInsertPlantumlDiagram_NoPlantUML(t *testing.T) {
	t.Parallel()

	input := `<p>Hello world</p><pre><code class="language-go">func main() {}</code></pre>`

	result, plantumls, err := InsertPlantumlDiagram(input, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != input {
		t.Errorf("expected unchanged content\ngot:  %s\nwant: %s", result, input)
	}

	if len(plantumls) != 0 {
		t.Errorf("expected no plantumls, got %d", len(plantumls))
	}
}

func TestInsertPlantumlDiagram_NoStartuml(t *testing.T) {
	t.Parallel()

	// PlantUML code block without @startuml should be left as-is
	input := `<pre><code class="language-plantuml">just some text without startuml</code></pre>`

	result, plantumls, err := InsertPlantumlDiagram(input, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != input {
		t.Errorf("expected unchanged content\ngot:  %s\nwant: %s", result, input)
	}

	if len(plantumls) != 0 {
		t.Errorf("expected no plantumls, got %d", len(plantumls))
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `<img src="diagram1">`
	if result != expected {
		t.Errorf("expected diagram replacement\ngot:  %s\nwant: %s", result, expected)
	}

	if len(plantumls) != 1 {
		t.Errorf("expected 1 plantuml, got %d", len(plantumls))
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `<img src="diagram-with-note">`
	if result != expected {
		t.Errorf("expected diagram replacement\ngot:  %s\nwant: %s", result, expected)
	}

	if len(plantumls) != 1 {
		t.Errorf("expected 1 plantuml, got %d", len(plantumls))
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, `<img src="diagram1">`) {
		t.Error("expected diagram1 in result")
	}

	if !strings.Contains(result, `<img src="diagram2">`) {
		t.Error("expected diagram2 in result")
	}

	if !strings.Contains(result, "<p>First diagram:</p>") {
		t.Error("expected first paragraph in result")
	}

	if !strings.Contains(result, "<p>Second diagram:</p>") {
		t.Error("expected second paragraph in result")
	}

	if len(plantumls) != 2 {
		t.Errorf("expected 2 plantumls, got %d", len(plantumls))
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Go code block should be preserved
	if !strings.Contains(result, `<pre><code class="language-go">func main() {}</code></pre>`) {
		t.Error("expected Go code block to be preserved")
	}

	// PlantUML should be replaced
	if !strings.Contains(result, `<img src="diagram1">`) {
		t.Error("expected PlantUML to be replaced with diagram")
	}

	// Python code block should be preserved
	if !strings.Contains(result, `<pre><code class="language-python">print("hello")</code></pre>`) {
		t.Error("expected Python code block to be preserved")
	}

	if len(plantumls) != 1 {
		t.Errorf("expected 1 plantuml, got %d", len(plantumls))
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `<img src="diagram1">`
	if result != expected {
		t.Errorf("expected diagram replacement\ngot:  %s\nwant: %s", result, expected)
	}

	if len(plantumls) != 1 {
		t.Errorf("expected 1 plantuml, got %d", len(plantumls))
	}
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
			if result != tt.expected {
				t.Errorf("hasLanguagePlantuml(%q) = %v, want %v", tt.classes, result, tt.expected)
			}
		})
	}
}

func TestHasLanguagePlantuml_NoClassAttr(t *testing.T) {
	t.Parallel()

	attrs := []html.Attribute{{Key: "id", Val: "some-id"}}
	if hasLanguagePlantuml(attrs) {
		t.Error("expected false when no class attribute")
	}
}

func TestEncode(t *testing.T) {
	t.Parallel()

	// Test that encoding is deterministic
	uml := "@startuml\nA -> B\n@enduml"
	encoded1 := Encode(uml)
	encoded2 := Encode(uml)

	if encoded1 != encoded2 {
		t.Errorf("encoding should be deterministic, got %s and %s", encoded1, encoded2)
	}

	// Should produce non-empty output
	if encoded1 == "" {
		t.Error("encoded string should not be empty")
	}
}

func TestHasLanguagePlantuml_MultipleAttrs(t *testing.T) {
	t.Parallel()

	attrs := []html.Attribute{
		{Key: "id", Val: "code-block"},
		{Key: "class", Val: "language-plantuml"},
		{Key: "data-line", Val: "1"},
	}

	if !hasLanguagePlantuml(attrs) {
		t.Error("expected true when class attribute contains language-plantuml")
	}
}

// Test that slices.Contains works as expected for the hasLanguagePlantuml helper.
func TestSlicesContains(t *testing.T) {
	t.Parallel()

	classes := strings.Fields("highlight language-plantuml line-numbers")
	if !slices.Contains(classes, "language-plantuml") {
		t.Error("expected slices.Contains to find language-plantuml")
	}

	if slices.Contains(classes, "language-plantuml-extra") {
		t.Error("expected slices.Contains to not find partial match")
	}
}
