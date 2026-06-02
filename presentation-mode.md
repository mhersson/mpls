# Presentation Mode

Transform your markdown documents into slideshow presentations directly in the
browser preview.

![presentation](screenshots/presentation-mode.gif)

## Creating Slides

Use HTML comments to mark slide boundaries, or let mpls create slides
automatically from your document structure.

### Automatic Slides

If your markdown has no `<!-- slide -->` markers, presentation mode will
automatically split content into slides based on headers (H1, H2, and H3). This
lets you present any markdown document without modification:

- Each H1, H2, or H3 header starts a new slide
- Content before the first header becomes the title slide
- Headers without content are merged with the following slide (so a section
  header followed immediately by a subsection stays together)

**Note:** Adding even a single `<!-- slide -->` marker disables automatic
splitting entirely, giving you full manual control over slide boundaries.

For more control, use explicit markers:

### Explicit Slide Markers

````markdown
# Welcome

This is the title slide.

<!-- slide -->

# Agenda

- Point 1
- Point 2
- Point 3

<!-- slide -->

# Details

<!-- center -->

This content will be centered on the slide.

<!-- /center -->

Regular content appears normally.

<!-- slide -->

# Code Comparison

<!-- split -->

**Before:**

```python
def old_func():
    pass
```

<!-- | -->

**After:**

```python
def new_func():
    return True
```

<!-- /split -->

<!-- slide -->

# Incremental Reveal

<!-- one-by-one -->
- Item one
- Item two
- Item three
- Item four
<!-- /one-by-one -->
````

### Incremental Reveal

Use `<!-- one-by-one -->` to reveal list items one at a time during the
presentation. Each top-level list item in the region becomes a separate step:

````markdown
<!-- slide -->

## My slide header

<!-- one-by-one -->
- Item one
- Item two
- Item three
- Item four
<!-- /one-by-one -->
````

- Entering the slide (moving **forward**) shows the header and the **first item
  only**. Each press of Next reveals the next item. Only after all items are
  revealed does Next advance to the following slide.
- Pressing **Prev** on a partially-revealed slide hides the last-revealed item.
  Only when back at the first item does Prev move to the previous slide (entering
  that slide with all its items revealed).
- The closing `<!-- /one-by-one -->` marker is **optional** — if omitted the
  region extends to the end of the slide.
- Multiple `<!-- one-by-one -->` regions per slide are supported; their items are
  revealed in order. When using more than one region on a slide, close each with
  `<!-- /one-by-one -->` so the first region does not extend over the next.
- **Printing / PDF export** always shows all items, regardless of reveal state.

## Navigation

| Key                 | Action                   |
| ------------------- | ------------------------ |
| `p`                 | Toggle presentation mode |
| `→` / `l` / `Space` | Next step / slide        |
| `←` / `h`           | Previous step / slide    |
| `↓` / `j`           | Next step / slide        |
| `↑` / `k`           | Previous step / slide    |
| `g` / `Home`        | First slide              |
| `G` / `End`         | Last slide               |
| `Escape`            | Exit presentation mode   |

You can also use the on-screen `<` `>` buttons or the "Presentation" checkbox in
the header.

## PDF Export

1. Enter presentation mode
2. Press `Ctrl+P` (or `Cmd+P` on Mac)
3. Select "Save as PDF"

Each slide becomes a separate page with proper page breaks.
