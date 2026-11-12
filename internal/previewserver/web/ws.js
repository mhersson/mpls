(function () {
  "use strict";

  // ==========================================================================
  // Configuration
  // ==========================================================================

  const CONFIG = {
    MERMAID_RENDER_DELAY: 100,
    SCROLL_OFFSET: 150,
    SCROLL_RETRY_DELAY: 50,
    MAX_SCROLL_RETRIES: 10,
    RESIZE_DEBOUNCE_DELAY: 250,
    // Largest zoom for a BPMN diagram opened in the modal.
    BPMN_MODAL_MAX_SCALE: 2,
  };

  // ==========================================================================
  // State
  // ==========================================================================

  const state = {
    ws: null,
    isReloading: false,
    enableTabsMode: false,
  };

  // ==========================================================================
  // Utilities
  // ==========================================================================

  const $ = (id) => document.getElementById(id);
  const $$ = (sel) => document.querySelectorAll(sel);

  function debounce(fn, delay) {
    let timeout;
    return function (...args) {
      clearTimeout(timeout);
      timeout = setTimeout(() => fn.apply(this, args), delay);
    };
  }

  function escapeHtml(text) {
    const div = document.createElement("div");
    div.textContent = text;
    return div.innerHTML;
  }

  // ==========================================================================
  // Mermaid Module
  // ==========================================================================

  const mermaidRenderer = {
    // TODO: Consider cache eviction for long-running sessions
    svgCache: new Map(), // source content -> rendered SVG outerHTML

    renderError(el, error) {
      const errDiv = document.createElement("div");
      errDiv.className = "mermaid-error";
      errDiv.style.cssText = "color:red;border:1px solid red;padding:8px";
      errDiv.innerHTML = `<strong>Mermaid Error:</strong><pre>${escapeHtml(error.message || String(error))}</pre>`;
      el.innerHTML = "";
      el.appendChild(errDiv);
    },

    async render() {
      const elements = $$(".language-mermaid");
      if (elements.length === 0 || !window.mermaid) {
        return;
      }

      for (const el of elements) {
        // Skip if already has SVG (e.g., from initial page load)
        if (el.querySelector("svg")) continue;

        const sourceContent = el.textContent || "";

        // Check cache for pre-rendered SVG
        const cachedSvg = this.svgCache.get(sourceContent);
        if (cachedSvg) {
          el.innerHTML = cachedSvg;
          continue;
        }

        // Render new diagram
        el.setAttribute("data-rendering", "true");
        try {
          await window.mermaid.run({ nodes: [el] });
          // Cache the rendered SVG for future content updates
          const svg = el.querySelector("svg");
          if (svg) {
            this.svgCache.set(sourceContent, svg.outerHTML);
          }
        } catch (error) {
          console.error("Mermaid diagram failed:", error);
          this.renderError(el, error);
        } finally {
          el.removeAttribute("data-rendering");
        }
      }
    },
  };

  // ==========================================================================
  // BPMN Module
  // ==========================================================================
  //
  // Renders ```bpmn fenced blocks with the bpmn-js Viewer, loaded on demand
  // from /bpmn/ only when a page contains BPMN.
  //
  // LICENSE NOTE: bpmn-js may only be used if the bpmn.io watermark it draws
  // (a.bjs-powered-by) is never removed or changed and stays fully visible and
  // not overlapped (see web/bpmn/LICENSE). Therefore:
  //   - the live Viewer, which owns the watermark, stays in the DOM; diagrams
  //     are never exported to static SVG,
  //   - the drawing is fitted above a reserved strip at the bottom of the card
  //     so no shape is ever drawn under the watermark,
  //   - the card has a light background so the watermark keeps its contrast
  //     on dark themes (see .bpmn-diagram in styles.css).

  const bpmnRenderer = {
    loading: null,
    // source -> Viewer[]; a viewer whose card left the DOM (content update,
    // slide change) is attached to the new card instead of re-importing.
    viewers: new Map(),
    MAX_CACHED_VIEWERS: 20,
    PADDING: 16,
    // The watermark is 21px high and sits 15px above the bottom edge.
    WATERMARK_SPACE: 48,

    load() {
      if (window.BpmnJS) return Promise.resolve();
      if (!this.loading) {
        for (const href of ["/bpmn/diagram-js.css", "/bpmn/bpmn-js.css"]) {
          if (document.querySelector(`link[href="${href}"]`)) continue;
          const link = document.createElement("link");
          link.rel = "stylesheet";
          link.href = href;
          document.head.appendChild(link);
        }
        this.loading = new Promise((resolve, reject) => {
          const script = document.createElement("script");
          script.src = "/bpmn/bpmn-navigated-viewer.production.min.js";
          script.onload = resolve;
          script.onerror = () => {
            this.loading = null;
            script.remove();
            reject(new Error("Failed to load the BPMN renderer"));
          };
          document.head.appendChild(script);
        });
      }
      return this.loading;
    },

    renderError(card, error) {
      card.classList.add("bpmn-error");
      card.style.height = "";
      card.innerHTML = `<strong>BPMN Error:</strong><pre>${escapeHtml(error.message || String(error))}</pre>`;
    },

    isFree(viewer) {
      return (
        !viewer._mplsInModal &&
        (!viewer._mplsCard || !viewer._mplsCard.isConnected)
      );
    },

    // Takes a cached viewer for this source that is not shown anywhere else.
    takeViewer(source) {
      const pool = this.viewers.get(source) || [];
      return pool.find((v) => this.isFree(v));
    },

    // Creates a NavigatedViewer whose panning and zooming are off until it is
    // shown in the modal: an inline card cannot be dragged, and the mouse
    // wheel over it scrolls the page.
    createViewer(card) {
      // ZoomScroll re-reads this object on every mouseover, so it is kept
      // and flipped by setNavigation instead of toggling ZoomScroll once.
      const zoomScroll = { enabled: false };
      const viewer = new window.BpmnJS({ container: card, zoomScroll });
      viewer._mplsZoomScroll = zoomScroll;
      // Returning false stops MoveCanvas (priority 500) from starting a drag.
      viewer
        .get("eventBus")
        .on("element.mousedown", 1500, () =>
          viewer._mplsInModal ? undefined : false,
        );
      return viewer;
    },

    setNavigation(viewer, enabled) {
      viewer._mplsZoomScroll.enabled = enabled;
      viewer.get("zoomScroll").toggle(enabled);
    },

    cacheViewer(source, viewer) {
      const pool = this.viewers.get(source) || [];
      pool.push(viewer);
      this.viewers.set(source, pool);
    },

    evict() {
      let total = 0;
      this.viewers.forEach((pool) => (total += pool.length));
      for (const [source, pool] of this.viewers) {
        if (total <= this.MAX_CACHED_VIEWERS) break;
        for (const viewer of pool.slice()) {
          if (total <= this.MAX_CACHED_VIEWERS) break;
          if (!this.isFree(viewer)) continue;
          viewer.destroy();
          pool.splice(pool.indexOf(viewer), 1);
          total--;
        }
        if (pool.length === 0) this.viewers.delete(source);
      }
    },

    // Draws the diagram centred in `box`, above the watermark strip, at
    // most at `maxScale`. An inline card (fixedHeight false) fits the width
    // and takes the height of the diagram; the modal (fixedHeight true) keeps
    // its size and fits the diagram in both directions.
    fit(box, viewer, fixedHeight = false, maxScale = 1) {
      const width = box.clientWidth;
      if (!width) return;

      const canvas = viewer.get("canvas");
      canvas.resized();
      const inner = canvas.viewbox().inner;
      if (!inner.width || !inner.height) return;

      const pad = this.PADDING;
      const reserved = pad + this.WATERMARK_SPACE;
      let scale = Math.min(maxScale, (width - 2 * pad) / inner.width);
      let height;
      let top = pad;

      if (fixedHeight) {
        height = box.clientHeight;
        scale = Math.min(scale, (height - reserved) / inner.height);
        top = pad + (height - reserved - inner.height * scale) / 2;
      } else {
        height = Math.ceil(inner.height * scale + reserved);
        box.style.height = height + "px";
        canvas.resized();
      }
      if (!(scale > 0)) return;

      canvas.viewbox({
        x: inner.x - (width / scale - inner.width) / 2,
        y: inner.y - top / scale,
        width: width / scale,
        height: height / scale,
      });
    },

    refit() {
      $$(".bpmn-diagram").forEach((card) => {
        const viewer = card._mplsViewer;
        if (viewer && !viewer._mplsInModal) this.fit(card, viewer);
      });
      if (modal.bpmn) {
        this.fit(
          modal.elements.bpmnContent,
          modal.bpmn.viewer,
          true,
          CONFIG.BPMN_MODAL_MAX_SCALE,
        );
      }
    },

    async render(root = document) {
      const blocks = Array.from(root.querySelectorAll("pre > code.language-bpmn"));
      if (blocks.length === 0) return;

      const pending = [];
      for (const code of blocks) {
        const pre = code.parentElement;
        const next = pre.nextElementSibling;
        if (next && next.classList.contains("bpmn-diagram")) {
          // A card created by this script is live; anything else is a dead
          // copy from an innerHTML snapshot (presentation mode) and is
          // replaced by a live viewer.
          if (next._mplsBpmn) continue;
          next.remove();
        }

        const card = document.createElement("div");
        card.className = "bpmn-diagram";
        card._mplsBpmn = true;
        card.setAttribute("data-rendering", "true");
        pre.after(card);
        pending.push({ pre, card, source: code.textContent || "" });
      }
      if (pending.length === 0) return;

      try {
        await this.load();
      } catch (error) {
        console.error("BPMN renderer failed to load:", error);
        for (const { card } of pending) {
          this.renderError(card, error);
          card.removeAttribute("data-rendering");
        }
        return;
      }

      for (const { pre, card, source } of pending) {
        if (!card.isConnected) continue;
        let viewer = this.takeViewer(source);
        try {
          if (viewer) {
            viewer._mplsCard = card;
            viewer.attachTo(card);
          } else {
            viewer = this.createViewer(card);
            viewer._mplsCard = card;
            const { warnings } = await viewer.importXML(source);
            if (warnings && warnings.length > 0) {
              console.warn("BPMN import warnings:", warnings);
            }
            this.cacheViewer(source, viewer);
          }
          card._mplsViewer = viewer;
          pre.classList.add("bpmn-source-rendered");
          this.fit(card, viewer);
        } catch (error) {
          console.error("BPMN diagram failed:", error);
          if (viewer) {
            const pool = this.viewers.get(source);
            if (pool && pool.includes(viewer)) {
              pool.splice(pool.indexOf(viewer), 1);
            }
            viewer.destroy();
          }
          this.renderError(card, error);
        } finally {
          card.removeAttribute("data-rendering");
        }
      }

      this.evict();
    },

    // ------------------------------------------------------------------------
    // Printing
    //
    // A browser never splits an <svg> across pages: a live viewer taller than
    // a page is cut off at the page bottom, and its drawing is fitted to the
    // screen width, not to the paper. For printing, each diagram is therefore
    // laid out as page-sized slices: copies of the drawing in diagram units,
    // scaled by the browser to the paper width. Every slice carries its own
    // copy of the bpmn.io watermark, unchanged, in a reserved strip below the
    // drawing, so each printed part of a diagram shows the watermark. The live
    // viewer stays in the page (collapsed while printing) and the slices are
    // removed after printing.
    // ------------------------------------------------------------------------

    // Roughly the content width of A4 and Letter paper at default margins, in
    // CSS px. Narrower paper only makes the slices smaller.
    PRINT_WIDTH: 720,
    // Printed height of one slice including the watermark strip; fits A4 and
    // Letter pages at default margins.
    PRINT_SLICE_HEIGHT: 860,

    preparePrint() {
      this.cleanupPrint();

      $$(".bpmn-diagram").forEach((card) => {
        const viewer = card._mplsViewer;
        if (!viewer || viewer._mplsInModal || !card.isConnected) return;

        const svg = card.querySelector(".djs-container > svg");
        const watermark = card.querySelector(".bjs-powered-by");
        // Never print a diagram without its watermark: leave it as it is.
        if (!svg || !watermark) return;

        const inner = viewer.get("canvas").viewbox().inner;
        if (!inner.width || !inner.height) return;

        const pad = this.PADDING;
        const x = inner.x - pad;
        const width = inner.width + 2 * pad;
        const top = inner.y - pad;
        const bottom = inner.y + inner.height + pad;
        const scale = Math.min(1, this.PRINT_WIDTH / width);
        const band = (this.PRINT_SLICE_HEIGHT - this.WATERMARK_SPACE) / scale;
        const edges = [top, ...this.sliceBreaks(viewer, top, bottom, band), bottom];

        const slices = document.createElement("div");
        slices.className = "bpmn-print";

        for (let i = 0; i < edges.length - 1; i++) {
          const sliceTop = edges.at(i);
          const sliceHeight = edges.at(i + 1) - sliceTop;
          const copy = svg.cloneNode(true);
          // Draw in diagram units; the viewBox selects this slice's band.
          copy.querySelector(".viewport")?.removeAttribute("transform");
          copy.removeAttribute("width");
          copy.removeAttribute("height");
          copy.removeAttribute("tabindex");
          copy.setAttribute(
            "viewBox",
            `${x} ${sliceTop} ${width} ${sliceHeight}`,
          );
          copy.setAttribute("preserveAspectRatio", "xMidYMin meet");
          copy.style.aspectRatio = `${width} / ${sliceHeight}`;

          const slice = document.createElement("div");
          slice.className = "bpmn-print-slice";
          // Never print larger than the diagram's own size.
          slice.style.maxWidth = Math.ceil(width) + "px";
          slice.appendChild(copy);
          slice.appendChild(watermark.cloneNode(true));
          slices.appendChild(slice);
        }

        card.after(slices);
        card.classList.add("bpmn-printing");
      });
    },

    // Returns where print slices end, in diagram units. Each slice is filled
    // up to `band`, but ends in a gap between shapes and labels when there is
    // one in its lower half, so no shape is cut in two. Pools, lanes and other
    // shapes taller than half a slice cannot avoid a cut and are ignored.
    sliceBreaks(viewer, top, bottom, band) {
      const spans = viewer
        .get("elementRegistry")
        .filter((e) => e.parent && !e.waypoints && e.height < band / 2)
        .map((e) => [e.y, e.y + e.height]);

      const breaks = [];
      let start = top;
      while (bottom - start > band) {
        const limit = start + band;
        let cut = limit;
        for (let guard = 0; guard < spans.length; guard++) {
          const hit = spans.find(([a, b]) => a < cut && cut < b);
          if (!hit) break;
          cut = hit[0] - 4;
        }
        if (cut <= start + band / 2) cut = limit;
        breaks.push(cut);
        start = cut;
      }
      return breaks;
    },

    cleanupPrint() {
      $$(".bpmn-print").forEach((slices) => slices.remove());
      $$(".bpmn-printing").forEach((card) =>
        card.classList.remove("bpmn-printing"),
      );
    },
  };

  // Used by presentation.js to render diagrams on the current slide.
  window.mplsBpmn = {
    render: (root) => bpmnRenderer.render(root),
  };

  // ==========================================================================
  // Scroll Module
  // ==========================================================================

  const scroll = {
    lastTarget: null,

    toEdit(retryCount = 0, fileChanged = false) {
      const disableScrolling = $("disable-scrolling");
      if (disableScrolling?.checked) {
        return;
      }

      const targetElement = document.querySelector("[data-mpls-scroll-anchor]");
      if (!targetElement) {
        this.lastTarget = null;
        // If file changed and no anchor, scroll to top
        if (fileChanged && window.scrollY > 0) {
          window.scrollTo({
            top: 0,
            behavior: "smooth",
          });
        }
        return;
      }

      // Check if any mermaid diagrams are still rendering
      const renderingElements = $$('[data-rendering="true"]');
      if (
        renderingElements.length > 0 &&
        retryCount < CONFIG.MAX_SCROLL_RETRIES
      ) {
        setTimeout(
          () => this.toEdit(retryCount + 1, fileChanged),
          CONFIG.SCROLL_RETRY_DELAY,
        );
        return;
      }

      // Store target for future reference
      this.lastTarget = targetElement;

      const elementRect = targetElement.getBoundingClientRect();
      const elementTop = elementRect.top + window.scrollY;
      const targetScrollPosition = elementTop - CONFIG.SCROLL_OFFSET;

      // Only scroll if we're not already at the target position
      if (Math.abs(window.scrollY - targetScrollPosition) > 5) {
        window.scrollTo({
          top: targetScrollPosition,
          behavior: "smooth",
        });
      }
    },

    handleResize: debounce(() => {
      if (scroll.lastTarget && !$("disable-scrolling")?.checked) {
        scroll.toEdit();
      }
    }, CONFIG.RESIZE_DEBOUNCE_DELAY),
  };

  // ==========================================================================
  // Modal Module
  // ==========================================================================

  const modal = {
    elements: {
      modal: null,
      closeBtn: null,
      mermaidContent: null,
      imageContent: null,
      bpmnContent: null,
    },

    // The BPMN diagram shown in the modal: { card, viewer }. Its live viewer
    // (and with it the bpmn.io watermark) is moved into the modal and back.
    bpmn: null,

    init() {
      this.elements.modal = $("contentModal");
      this.elements.closeBtn = $("closeModal");
      this.elements.mermaidContent = $("mermaidContent");
      this.elements.imageContent = $("imageContent");
      this.elements.bpmnContent = $("bpmnContent");

      if (
        !this.elements.modal ||
        !this.elements.closeBtn ||
        !this.elements.mermaidContent ||
        !this.elements.imageContent ||
        !this.elements.bpmnContent
      ) {
        console.warn("Modal elements not found");
        return false;
      }

      // Event listeners
      this.elements.closeBtn.addEventListener("click", () => this.hide());
      this.elements.modal.addEventListener("click", (event) => {
        if (event.target === this.elements.modal) {
          this.hide();
        }
      });

      // Keyboard navigation
      document.addEventListener("keydown", (event) => {
        if (
          event.key === "Escape" &&
          this.elements.modal.style.display === "flex"
        ) {
          this.hide();
        }
      });

      return true;
    },

    show(type, content) {
      const { mermaidContent, imageContent, bpmnContent, modal: modalEl } =
        this.elements;

      // Reset modal content
      this.releaseBpmn();
      bpmnContent.style.display = "none";
      mermaidContent.style.display = "none";
      imageContent.style.display = "none";
      mermaidContent.innerHTML = "";
      imageContent.src = "";

      if (type === "image" && content.src) {
        imageContent.src = content.src;
        imageContent.style.display = "flex";
      } else if (type === "mermaid") {
        const svgClone = content.cloneNode(true);
        svgClone.classList.add("mermaid-modal-svg");
        mermaidContent.appendChild(svgClone);
        mermaidContent.style.display = "flex";
      }

      modalEl.style.display = "flex";

      if (type === "bpmn" && content._mplsViewer) {
        // The modal must be visible before the diagram can be measured.
        const viewer = content._mplsViewer;
        bpmnContent.style.display = "block";
        viewer._mplsInModal = true;
        viewer.attachTo(bpmnContent);
        bpmnRenderer.setNavigation(viewer, true);
        this.bpmn = { card: content, viewer };
        bpmnRenderer.fit(
          bpmnContent,
          viewer,
          true,
          CONFIG.BPMN_MODAL_MAX_SCALE,
        );
      }
    },

    // Moves the BPMN viewer from the modal back to its card. If the card was
    // replaced by a content update meanwhile, the viewer is only detached and
    // stays cached for reuse.
    releaseBpmn() {
      if (!this.bpmn) return;
      const { card, viewer } = this.bpmn;
      this.bpmn = null;
      viewer._mplsInModal = false;
      bpmnRenderer.setNavigation(viewer, false);
      if (card.isConnected && viewer._mplsCard === card) {
        viewer.attachTo(card);
        bpmnRenderer.fit(card, viewer);
      } else {
        viewer.detach();
      }
      this.elements.bpmnContent.style.display = "none";
    },

    hide() {
      const { modal: modalEl, mermaidContent, imageContent } = this.elements;
      this.releaseBpmn();
      modalEl.style.display = "none";
      mermaidContent.innerHTML = "";
      imageContent.src = "";
    },

    attachHandlers() {
      // Setup image modals
      $$("img").forEach((img) => {
        if (img._modalHandlerAttached) return;

        img._modalHandler = () => this.show("image", img);
        img.addEventListener("click", img._modalHandler);
        img._modalHandlerAttached = true;
        img.style.cursor = "pointer";
      });

      // Setup Mermaid modals
      $$(".language-mermaid").forEach((div) => {
        const svg = div.querySelector("svg");
        if (svg && !div._modalHandlerAttached) {
          div._modalHandler = () => this.show("mermaid", svg);
          div.addEventListener("click", div._modalHandler);
          div._modalHandlerAttached = true;
          div.style.cursor = "pointer";
        }
      });

      // Setup BPMN modals. A click on the bpmn.io watermark keeps its own
      // behaviour (the bpmn.io notice) and does not open the modal.
      $$(".bpmn-diagram").forEach((card) => {
        if (!card._mplsViewer || card._modalHandlerAttached) return;
        // A drag across the card that ends in a click is not a click.
        // Capture phase: diagram-js stops mousedown propagation on shapes.
        let down = null;
        card.addEventListener(
          "mousedown",
          (event) => {
            down = { x: event.clientX, y: event.clientY };
          },
          true,
        );
        card._modalHandler = (event) => {
          if (event.target.closest(".bjs-powered-by")) return;
          if (
            down &&
            Math.hypot(event.clientX - down.x, event.clientY - down.y) > 5
          ) {
            return;
          }
          this.show("bpmn", card);
        };
        card.addEventListener("click", card._modalHandler);
        card._modalHandlerAttached = true;
        card.style.cursor = "zoom-in";
        card.title = "Click to enlarge";
      });
    },
  };

  // ==========================================================================
  // Heading Anchors Module
  // ==========================================================================

  const headingAnchors = {
    attach() {
      const headings = $$("h1[id], h2[id], h3[id], h4[id], h5[id], h6[id]");
      for (const heading of headings) {
        if (heading.querySelector(".heading-anchor")) continue;
        const anchor = document.createElement("a");
        anchor.className = "heading-anchor";
        anchor.href = "#" + heading.id;
        anchor.setAttribute("aria-label", "Link to this section");
        anchor.innerHTML = '<svg viewBox="0 0 16 16"><path d="M7.775 3.275a.75.75 0 001.06 1.06l1.25-1.25a2 2 0 112.83 2.83l-2.5 2.5a2 2 0 01-2.83 0 .75.75 0 00-1.06 1.06 3.5 3.5 0 004.95 0l2.5-2.5a3.5 3.5 0 00-4.95-4.95l-1.25 1.25zm-4.69 9.64a2 2 0 010-2.83l2.5-2.5a2 2 0 012.83 0 .75.75 0 001.06-1.06 3.5 3.5 0 00-4.95 0l-2.5 2.5a3.5 3.5 0 004.95 4.95l1.25-1.25a.75.75 0 00-1.06-1.06l-1.25 1.25a2 2 0 01-2.83 0z"></path></svg>';
        heading.prepend(anchor);
      }
    },
  };

  // ==========================================================================
  // Content Module
  // ==========================================================================

  const content = {
    update(renderedHtml) {
      const contentElement = $("content");
      if (contentElement) {
        contentElement.innerHTML = renderedHtml;
      }
    },
  };

  // ==========================================================================
  // Links Module
  // ==========================================================================

  const links = {
    setup() {
      document.addEventListener("click", (event) => this.handleClick(event));
    },

    handleClick(event) {
      const link = event.target.closest("a[href]");
      if (!link) return;

      const href = link.getAttribute("href");

      // Anchor-only links - let browser handle
      if (href.startsWith("#")) {
        return;
      }

      // External links - let browser handle
      if (this.isExternal(href)) {
        return;
      }

      // Internal markdown links
      const isInternal = link.hasAttribute("data-mpls-internal");
      const target = link.getAttribute("data-mpls-target");

      if (isInternal && target) {
        event.preventDefault();

        const message = {
          type: "openDocument",
          uri: target,
          takeFocus: true,
        };

        // In single-page mode, also update the preview
        if (!state.enableTabsMode) {
          message.updatePreview = true;
        }

        state.ws.send(JSON.stringify(message));
      }
    },

    isExternal(href) {
      return href.startsWith("http://") || href.startsWith("https://");
    },
  };

  // ==========================================================================
  // WebSocket Module
  // ==========================================================================

  const websocket = {
    handlers: {
      config: (data) => websocket.handleConfig(data),
      closeDocument: (data) => websocket.handleClose(data),
    },

    init(ws) {
      state.ws = ws;

      ws.addEventListener("open", () => {
        console.log("WebSocket connection established");
        links.setup();
      });

      ws.addEventListener("message", (event) => this.onMessage(event));

      ws.addEventListener("close", (event) => {
        console.log("WebSocket connection closed:", event);
        if (!state.isReloading) {
          window.close();
        }
      });

      ws.addEventListener("error", (event) => {
        console.error("WebSocket error:", event);
      });
    },

    async onMessage(event) {
      try {
        const response = JSON.parse(event.data);

        // Check for special message types
        const handler = this.handlers[response.Type];
        if (handler) {
          handler(response);
          return;
        }

        // Guard against unknown message types
        if (!response.HTML) {
          console.warn("Unknown message type:", response.Type);
          return;
        }

        // Default: content update
        await this.handleContent(response);
      } catch (error) {
        console.error("Failed to process WebSocket message:", error);
      }
    },

    handleConfig(data) {
      state.enableTabsMode = data.EnableTabs || false;
      console.log(
        `Preview mode: ${state.enableTabsMode ? "multi-tab" : "single-page"}`,
      );
    },

    handleClose(data) {
      // Close window if it's the last document (in single-page mode)
      if (data.IsLastDocument) {
        console.log("Last document closed, closing preview");
        window.close();
        return;
      }

      // Close window in multi-tab mode if it matches this tab
      if (
        state.enableTabsMode &&
        data.DocumentURI === window.location.pathname
      ) {
        console.log(`Closing preview for ${data.DocumentURI}`);
        window.close();
      }
    },

    async handleContent(response) {
      const {
        HTML: renderedHtml,
        Title: responseTitle,
        Meta: meta,
        DocumentURI: documentURI,
      } = response;

      // If DocumentURI is provided, check if it matches current page
      if (documentURI && documentURI !== window.location.pathname) {
        console.log(
          `Ignoring update for ${documentURI}, current page is ${window.location.pathname}`,
        );
        return;
      }

      const title = `mpls - ${responseTitle}`;
      const pin = $("pin");

      // Check if preview is pinned
      if (pin?.checked && title !== document.title) {
        console.log("Preview is pinned - ignoring event");
        return;
      }

      // Update title if changed
      let titleChanged = false;
      if (title !== document.title) {
        titleChanged = true;
        const headerSummary = $("header-summary");
        if (headerSummary) {
          headerSummary.innerText = responseTitle;
        }
        document.title = title;
      }

      // Update meta if changed
      const headerMeta = $("header-meta");
      if (headerMeta && headerMeta.innerHTML !== meta) {
        headerMeta.innerHTML = meta;
      }

      // Update content
      content.update(renderedHtml);

      // Notify presentation module of content update
      if (window.presentation) {
        window.presentation.onContentUpdate(renderedHtml);
      }

      // Render and scroll
      await renderDiagramsAndScroll(titleChanged);
    },
  };

  // ==========================================================================
  // Lazy Loading
  // ==========================================================================

  function setupLazyLoading() {
    const observerOptions = {
      root: null,
      rootMargin: "50px",
      threshold: 0.01,
    };

    const imageObserver = new IntersectionObserver((entries) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting) {
          const img = entry.target;
          if (img.dataset.src && !img.src) {
            img.src = img.dataset.src;
            imageObserver.unobserve(img);
          }
        }
      });
    }, observerOptions);

    // Observe all images with data-src
    $$("img[data-src]").forEach((img) => {
      imageObserver.observe(img);
    });
  }

  // ==========================================================================
  // Printing Module
  // ==========================================================================

  const printing = {
    init() {
      window.addEventListener("beforeprint", () => this.onBeforePrint());
      window.addEventListener("afterprint", () => bpmnRenderer.cleanupPrint());
    },

    onBeforePrint() {
      // Presentation mode owns its own print lifecycle (see presentation.js).
      if (window.presentation?.isActive()) return;

      this.forceLoadImages();
      this.closeModalIfOpen();
      bpmnRenderer.preparePrint();
    },

    forceLoadImages() {
      $$("img[data-src]").forEach((img) => {
        if (!img.src && img.dataset.src) {
          img.src = img.dataset.src;
        }
        img.removeAttribute("data-src");
      });
    },

    closeModalIfOpen() {
      if (modal.elements.modal?.style.display === "flex") {
        modal.hide();
      }
    },
  };

  // ==========================================================================
  // Combined Operations
  // ==========================================================================

  async function renderDiagramsAndScroll(fileChanged = false) {
    // First render diagrams
    await Promise.all([mermaidRenderer.render(), bpmnRenderer.render()]);

    // Add anchor links to headings
    headingAnchors.attach();

    // Then attach modal handlers
    modal.attachHandlers();

    // Finally scroll to edit position
    // Use setTimeout to ensure DOM has settled
    setTimeout(() => {
      scroll.toEdit(0, fileChanged);
    }, CONFIG.MERMAID_RENDER_DELAY);
  }

  // ==========================================================================
  // Initialization
  // ==========================================================================

  function init() {
    const ws = new WebSocket("ws://localhost:%d/ws");

    modal.init();
    printing.init();
    websocket.init(ws);

    window.addEventListener("load", async () => {
      setupLazyLoading();
      // Render diagrams that are already in the HTML from HTTP GET
      await renderDiagramsAndScroll();
    });

    window.addEventListener("beforeunload", () => {
      state.isReloading = true;
    });

    window.addEventListener("resize", scroll.handleResize);
    window.addEventListener(
      "resize",
      debounce(() => bpmnRenderer.refit(), CONFIG.RESIZE_DEBOUNCE_DELAY),
    );
  }

  document.addEventListener("DOMContentLoaded", init);
})();
