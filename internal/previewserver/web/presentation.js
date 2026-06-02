// Presentation mode for mpls
(function () {
    "use strict";

    const CONTROLS_HIDE_DELAY = 2000;

    // State
    let state = {
        active: false,
        slides: [],
        currentIndex: 0,
        currentFragmentStep: 0,
        rawHtml: "",
        hideControlsTimer: null,
    };

    // DOM elements (initialized on DOMContentLoaded)
    let container = null;
    let slideContent = null;
    let printContainer = null;
    let prevBtn = null;
    let nextBtn = null;
    let counter = null;
    let checkbox = null;

    /**
     * Parse HTML content into slides
     * Splits on <!-- slide --> comments (case-insensitive)
     * Falls back to splitting on H1/H2/H3 headers if no slide markers found
     * Processes <!-- center --> / <!-- /center --> markers
     */
    function parseSlides(html) {
        if (!html || typeof html !== "string") {
            return [];
        }

        // Split on <!-- slide --> (case-insensitive)
        const slideMarker = /<!--\s*slide\s*-->/gi;
        let parts = html.split(slideMarker);

        // If no slide markers found (only 1 part), fall back to header splitting
        if (parts.length === 1) {
            parts = splitOnHeaders(html);
        }

        const slides = [];
        for (let i = 0; i < parts.length; i++) {
            let content = parts[i].trim();
            if (content.length === 0) {
                continue;
            }

            // Process center markers
            content = processCenterMarkers(content);

            // Process split/column markers
            content = processSplitMarkers(content);

            // Process one-by-one reveal markers (after center/split so structure is final)
            const oneByOne = processOneByOneMarkers(content);
            content = oneByOne.html;

            slides.push({
                html: content,
                index: slides.length,
                fragmentCount: oneByOne.fragmentCount,
            });
        }

        return slides;
    }

    /**
     * Split HTML on H1, H2, and H3 headers for auto-slide mode
     * Used as fallback when no <!-- slide --> markers are present
     * Merges header-only slides with next slide (except first slide)
     */
    function splitOnHeaders(html) {
        // Split before <h1>, <h2>, or <h3> tags using lookahead
        const headerPattern = /(?=<h[123][^>]*>)/gi;
        const parts = html.split(headerPattern).filter(part => part.trim().length > 0);

        // Merge header-only parts with next part (except first slide which can be title-only)
        const merged = [];
        for (let i = 0; i < parts.length; i++) {
            const part = parts[i];
            // Check if this part is header-only (no content after the header)
            const isHeaderOnly = /^<h[123][^>]*>.*?<\/h[123]>\s*$/is.test(part.trim());

            if (isHeaderOnly && i > 0 && i < parts.length - 1) {
                // Merge with next part
                parts[i + 1] = part + parts[i + 1];
            } else {
                merged.push(part);
            }
        }

        return merged;
    }

    /**
     * Process <!-- center --> / <!-- /center --> markers
     * Wraps content between markers in <div class="slide-centered">
     */
    function processCenterMarkers(html) {
        const centerStart = /<!--\s*center\s*-->/gi;
        const centerEnd = /<!--\s*\/center\s*-->/gi;

        // Replace start markers with opening div
        html = html.replace(centerStart, '<div class="slide-centered">');

        // Replace end markers with closing div
        html = html.replace(centerEnd, "</div>");

        return html;
    }

    /**
     * Process <!-- split --> / <!-- | --> / <!-- /split --> markers
     * Creates a two-column layout
     */
    function processSplitMarkers(html) {
        const splitPattern = /<!--\s*split\s*-->([\s\S]*?)<!--\s*\|\s*-->([\s\S]*?)<!--\s*\/split\s*-->/gi;

        return html.replace(splitPattern, (_match, left, right) => {
            return `<div class="slide-columns"><div class="slide-column-left">${left.trim()}</div><div class="slide-column-right">${right.trim()}</div></div>`;
        });
    }

    /**
     * Process <!-- one-by-one --> / <!-- /one-by-one --> markers
     * Adds class="slide-fragment" to each top-level <li> inside each region.
     * Returns { html, fragmentCount } so parseSlides can store the count per slide.
     * The closing marker is optional; if absent the region extends to end of slide.
     */
    function processOneByOneMarkers(html) {
        let fragmentCount = 0;
        // Lazy match from the open marker to the optional close marker; if the close
        // marker is absent the `$` alternative (end of string, no `m` flag) extends the
        // region to the end of the slide.
        const regionPattern = /<!--\s*one-by-one\s*-->([\s\S]*?)(?:<!--\s*\/one-by-one\s*-->|$)/gi;

        const result = html.replace(regionPattern, function (_match, inner) {
            // Tag each top-level <li> (not nested ones) with class="slide-fragment".
            // We walk the string and track <ul>/<ol> depth, tagging only depth-1 <li> tags.
            // depth is reset here so each region is counted independently. Assumes
            // well-formed goldmark HTML (balanced tags, quoted attributes).
            let depth = 0;
            const tagged = inner.replace(/<(\/?)(ul|ol|li)(\s[^>]*)?>/gi, function (tag, slash, tagName, attrs) {
                const lower = tagName.toLowerCase();
                if (slash === '/') {
                    // Closing tag
                    if (lower === 'ul' || lower === 'ol') {
                        depth--;
                    }
                    return tag;
                }
                // Opening tag
                if (lower === 'ul' || lower === 'ol') {
                    depth++;
                    return tag;
                }
                // <li> at depth 1 (top-level list in this region)
                if (lower === 'li' && depth === 1) {
                    fragmentCount++;
                    // Merge class if existing attrs already have one, else add fresh
                    if (attrs && /\bclass\s*=/i.test(attrs)) {
                        return '<li' + attrs.replace(/(\bclass\s*=\s*["'])/, '$1slide-fragment ') + '>';
                    }
                    return '<li class="slide-fragment"' + (attrs || '') + '>';
                }
                return tag;
            });
            // Emit tagged inner content without the comment markers
            return tagged;
        });

        return { html: result, fragmentCount: fragmentCount };
    }

    /**
     * Apply reveal state to fragment elements currently in the DOM.
     * step is the index of the last revealed fragment (0-based).
     * Fragments at index <= step are visible; index > step are hidden.
     */
    function applyFragmentState(step) {
        if (!slideContent) return;
        const fragments = slideContent.querySelectorAll('.slide-fragment');
        for (let i = 0; i < fragments.length; i++) {
            if (i <= step) {
                fragments[i].classList.remove('slide-fragment--hidden');
            } else {
                fragments[i].classList.add('slide-fragment--hidden');
            }
        }
    }

    /**
     * Render current slide.
     * revealAll=true: reveal all fragments (entering backward or live-update).
     * revealAll=false (default): reveal only the first fragment.
     */
    function renderSlide(revealAll) {
        if (!slideContent || state.slides.length === 0) {
            return;
        }

        const slide = state.slides[state.currentIndex];
        if (slide) {
            slideContent.innerHTML = slide.html;

            // Set initial fragment reveal state
            const fc = slide.fragmentCount || 0;
            if (fc > 0) {
                state.currentFragmentStep = revealAll ? fc - 1 : 0;
                applyFragmentState(state.currentFragmentStep);
            } else {
                state.currentFragmentStep = 0;
            }

            // Re-render mermaid diagrams if present and not already rendered
            if (window.mermaid) {
                const mermaidElements =
                    slideContent.querySelectorAll(".language-mermaid");
                if (mermaidElements.length > 0) {
                    // Only render elements that don't already have SVGs
                    const unrenderedElements = Array.from(mermaidElements).filter(
                        (el) => !el.querySelector("svg")
                    );

                    if (unrenderedElements.length > 0) {
                        // Reset processed state so mermaid will render
                        unrenderedElements.forEach((el) => {
                            el.removeAttribute("data-processed");
                        });
                        window.mermaid.run({
                            nodes: unrenderedElements,
                        });
                    }
                }
            }
        }

        updateCounter();
        updateNavButtons();
    }

    /**
     * Update slide counter display
     */
    function updateCounter() {
        if (counter && state.slides.length > 0) {
            counter.textContent = `${state.currentIndex + 1} / ${state.slides.length}`;
        }
    }

    /**
     * Update navigation button states
     */
    function updateNavButtons() {
        const slide = state.slides[state.currentIndex];
        const fc = (slide && slide.fragmentCount) || 0;

        if (prevBtn) {
            // Prev is disabled only at the very beginning: first slide, first (or no) fragment revealed
            prevBtn.disabled = state.currentIndex === 0 && state.currentFragmentStep <= 0;
        }
        if (nextBtn) {
            // Next is disabled only at the very end: last slide with all fragments revealed
            nextBtn.disabled = state.currentIndex >= state.slides.length - 1 &&
                (fc === 0 || state.currentFragmentStep >= fc - 1);
        }
    }

    /**
     * Navigate to next slide
     */
    function nextSlide() {
        const slide = state.slides[state.currentIndex];
        const fc = (slide && slide.fragmentCount) || 0;

        if (fc > 0 && state.currentFragmentStep < fc - 1) {
            // Reveal the next fragment without changing slide
            state.currentFragmentStep++;
            applyFragmentState(state.currentFragmentStep);
            updateNavButtons();
        } else if (state.currentIndex < state.slides.length - 1) {
            state.currentIndex++;
            renderSlide(false);
        }
    }

    /**
     * Navigate to previous slide
     */
    function prevSlide() {
        const slide = state.slides[state.currentIndex];
        const fc = (slide && slide.fragmentCount) || 0;

        if (fc > 0 && state.currentFragmentStep > 0) {
            // Hide the last-revealed fragment without changing slide
            state.currentFragmentStep--;
            applyFragmentState(state.currentFragmentStep);
            updateNavButtons();
        } else if (state.currentIndex > 0) {
            state.currentIndex--;
            // Entering a slide backward: reveal all its fragments so
            // pressing back again starts hiding from the last one.
            renderSlide(true);
        }
    }

    /**
     * Navigate to first slide
     */
    function firstSlide() {
        state.currentIndex = 0;
        // Show only the first fragment: consistent with entering a slide forward.
        renderSlide(false);
    }

    /**
     * Navigate to last slide
     */
    function lastSlide() {
        if (state.slides.length > 0) {
            state.currentIndex = state.slides.length - 1;
            // Reveal all fragments: pressing back will start hiding from the end,
            // which is the expected behaviour for End/G.
            renderSlide(true);
        }
    }

    /**
     * Show controls temporarily
     */
    function showControls() {
        if (!container) return;

        container.classList.add("controls-visible");

        // Clear existing timer
        if (state.hideControlsTimer) {
            clearTimeout(state.hideControlsTimer);
        }

        // Set new timer to hide controls
        state.hideControlsTimer = setTimeout(() => {
            container.classList.remove("controls-visible");
        }, CONTROLS_HIDE_DELAY);
    }

    /**
     * Enter presentation mode
     */
    function enter() {
        if (state.active) return;

        // Parse slides from current content
        const contentEl = document.getElementById("content");
        if (contentEl) {
            state.rawHtml = contentEl.innerHTML;
        }

        state.slides = parseSlides(state.rawHtml);

        if (state.slides.length === 0) {
            console.warn("No slides found. Add <!-- slide --> markers to your markdown.");
            return;
        }

        state.active = true;
        state.currentIndex = 0;
        state.currentFragmentStep = 0;

        if (container) {
            container.classList.add("active");
        }
        if (checkbox) {
            checkbox.checked = true;
        }

        renderSlide(false);
        showControls();

        // Hide scrollbar on body
        document.body.style.overflow = "hidden";
    }

    /**
     * Exit presentation mode
     */
    function exit() {
        if (!state.active) return;

        state.active = false;

        if (container) {
            container.classList.remove("active");
            container.classList.remove("controls-visible");
        }
        if (checkbox) {
            checkbox.checked = false;
        }

        // Clear hide timer
        if (state.hideControlsTimer) {
            clearTimeout(state.hideControlsTimer);
            state.hideControlsTimer = null;
        }

        // Restore scrollbar
        document.body.style.overflow = "";
    }

    /**
     * Toggle presentation mode
     */
    function toggle() {
        if (state.active) {
            exit();
        } else {
            enter();
        }
    }

    /**
     * Find which slide contains the scroll anchor (edit position)
     */
    function findSlideWithAnchor(slides) {
        for (let i = 0; i < slides.length; i++) {
            if (slides[i].html.includes('data-mpls-scroll-anchor')) {
                return i;
            }
        }
        return -1;
    }

    /**
     * Handle content updates from ws.js
     */
    function onContentUpdate(html) {
        state.rawHtml = html;

        if (state.active) {
            // Re-parse slides
            state.slides = parseSlides(html);

            // Clamp index to valid range
            if (state.slides.length === 0) {
                exit();
                return;
            }

            // Find slide containing the edit anchor and navigate to it
            const anchorSlide = findSlideWithAnchor(state.slides);
            if (anchorSlide >= 0) {
                state.currentIndex = anchorSlide;
            } else {
                // Fallback: clamp to valid range
                state.currentIndex = Math.min(state.currentIndex, state.slides.length - 1);
            }

            // Reveal all fragments so the author sees full content while editing
            renderSlide(true);
        }
    }

    /**
     * Handle keyboard events
     */
    function handleKeydown(event) {
        // Ignore if in input field
        if (
            event.target.tagName === "INPUT" ||
            event.target.tagName === "TEXTAREA" ||
            event.target.isContentEditable
        ) {
            return;
        }

        // Toggle presentation mode with 'p'
        if (event.key === "p" && !event.ctrlKey && !event.metaKey && !event.altKey) {
            event.preventDefault();
            toggle();
            return;
        }

        // Only handle navigation keys when in presentation mode
        if (!state.active) return;

        switch (event.key) {
            case "ArrowRight":
            case "l":
            case " ":
            case "PageDown":
                event.preventDefault();
                nextSlide();
                showControls();
                break;
            case "ArrowLeft":
            case "h":
            case "PageUp":
                event.preventDefault();
                prevSlide();
                showControls();
                break;
            case "ArrowDown":
            case "j":
                event.preventDefault();
                nextSlide();
                showControls();
                break;
            case "ArrowUp":
            case "k":
                event.preventDefault();
                prevSlide();
                showControls();
                break;
            case "Home":
            case "g":
                event.preventDefault();
                firstSlide();
                showControls();
                break;
            case "End":
            case "G":
                event.preventDefault();
                lastSlide();
                showControls();
                break;
            case "Escape":
                event.preventDefault();
                exit();
                break;
        }
    }

    /**
     * Create presentation DOM elements
     */
    function createElements() {
        // Create container
        container = document.createElement("div");
        container.id = "presentation-container";

        // Create slide content area
        slideContent = document.createElement("div");
        slideContent.id = "slide-content";
        container.appendChild(slideContent);

        // Create prev button
        prevBtn = document.createElement("button");
        prevBtn.id = "prev-slide";
        prevBtn.className = "slide-nav";
        prevBtn.innerHTML = "&#8249;"; // ‹
        prevBtn.title = "Previous slide (←)";
        prevBtn.setAttribute("aria-label", "Previous slide");
        prevBtn.addEventListener("click", () => {
            prevSlide();
            showControls();
        });
        container.appendChild(prevBtn);

        // Create next button
        nextBtn = document.createElement("button");
        nextBtn.id = "next-slide";
        nextBtn.className = "slide-nav";
        nextBtn.innerHTML = "&#8250;"; // ›
        nextBtn.title = "Next slide (→)";
        nextBtn.setAttribute("aria-label", "Next slide");
        nextBtn.addEventListener("click", () => {
            nextSlide();
            showControls();
        });
        container.appendChild(nextBtn);

        // Create counter
        counter = document.createElement("div");
        counter.id = "slide-counter";
        container.appendChild(counter);

        // Add to body
        document.body.appendChild(container);

        // Mouse movement shows controls
        container.addEventListener("mousemove", showControls);
    }

    /**
     * Render all slides for printing
     */
    function renderForPrint() {
        if (!state.active || state.slides.length === 0) {
            return;
        }

        // Create print container with all slides
        printContainer = document.createElement("div");
        printContainer.id = "print-container";

        state.slides.forEach((slide) => {
            const page = document.createElement("div");
            page.className = "slide-page";
            page.innerHTML = slide.html;
            printContainer.appendChild(page);
        });

        container.appendChild(printContainer);

        // Hide the interactive slide content during print
        slideContent.style.display = "none";
    }

    /**
     * Clean up after printing
     */
    function cleanupAfterPrint() {
        if (printContainer) {
            printContainer.remove();
            printContainer = null;
        }

        // Restore interactive slide content
        if (slideContent) {
            slideContent.style.display = "";
        }
    }

    /**
     * Initialize presentation module
     */
    function init() {
        // Get checkbox if it exists
        checkbox = document.getElementById("presentation-mode");
        if (checkbox) {
            checkbox.addEventListener("change", () => {
                if (checkbox.checked) {
                    enter();
                } else {
                    exit();
                }
            });
        }

        // Create presentation elements
        createElements();

        // Add keyboard listener
        document.addEventListener("keydown", handleKeydown);

        // Add print event listeners for PDF export
        window.addEventListener("beforeprint", renderForPrint);
        window.addEventListener("afterprint", cleanupAfterPrint);
    }

    // Initialize on DOM ready
    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", init);
    } else {
        init();
    }

    // Export public API
    window.presentation = {
        toggle: toggle,
        enter: enter,
        exit: exit,
        onContentUpdate: onContentUpdate,
        isActive: () => state.active,
        getCurrentSlide: () => state.currentIndex,
        getSlideCount: () => state.slides.length,
    };
})();
