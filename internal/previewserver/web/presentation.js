// Presentation mode for mpls
(function () {
    "use strict";

    const CONTROLS_HIDE_DELAY = 2000;

    // State
    let state = {
        active: false,
        slides: [],
        currentIndex: 0,
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

            slides.push({
                html: content,
                index: slides.length,
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
     * Render current slide
     */
    function renderSlide() {
        if (!slideContent || state.slides.length === 0) {
            return;
        }

        const slide = state.slides[state.currentIndex];
        if (slide) {
            slideContent.innerHTML = slide.html;

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
        if (prevBtn) {
            prevBtn.disabled = state.currentIndex === 0;
        }
        if (nextBtn) {
            nextBtn.disabled = state.currentIndex >= state.slides.length - 1;
        }
    }

    /**
     * Navigate to next slide
     */
    function nextSlide() {
        if (state.currentIndex < state.slides.length - 1) {
            state.currentIndex++;
            renderSlide();
        }
    }

    /**
     * Navigate to previous slide
     */
    function prevSlide() {
        if (state.currentIndex > 0) {
            state.currentIndex--;
            renderSlide();
        }
    }

    /**
     * Navigate to first slide
     */
    function firstSlide() {
        state.currentIndex = 0;
        renderSlide();
    }

    /**
     * Navigate to last slide
     */
    function lastSlide() {
        if (state.slides.length > 0) {
            state.currentIndex = state.slides.length - 1;
            renderSlide();
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

        if (container) {
            container.classList.add("active");
        }
        if (checkbox) {
            checkbox.checked = true;
        }

        renderSlide();
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
            if (slides[i].html.includes('id="mpls-scroll-anchor"')) {
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

            renderSlide();
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
