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
  // Scroll Module
  // ==========================================================================

  const scroll = {
    lastTarget: null,

    toEdit(retryCount = 0, fileChanged = false) {
      const disableScrolling = $("disable-scrolling");
      if (disableScrolling?.checked) {
        return;
      }

      const targetElement = $("mpls-scroll-anchor");
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
    },

    init() {
      this.elements.modal = $("contentModal");
      this.elements.closeBtn = $("closeModal");
      this.elements.mermaidContent = $("mermaidContent");
      this.elements.imageContent = $("imageContent");

      if (
        !this.elements.modal ||
        !this.elements.closeBtn ||
        !this.elements.mermaidContent ||
        !this.elements.imageContent
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
      const { mermaidContent, imageContent, modal: modalEl } = this.elements;

      // Reset modal content
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
    },

    hide() {
      const { modal: modalEl, mermaidContent, imageContent } = this.elements;
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
      await renderMermaidAndScroll(titleChanged);
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
  // Combined Operations
  // ==========================================================================

  async function renderMermaidAndScroll(fileChanged = false) {
    // First render mermaid
    await mermaidRenderer.render();

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
    websocket.init(ws);

    window.addEventListener("load", async () => {
      setupLazyLoading();
      // Render mermaid diagrams that are already in the HTML from HTTP GET
      await renderMermaidAndScroll();
    });

    window.addEventListener("beforeunload", () => {
      state.isReloading = true;
    });

    window.addEventListener("resize", scroll.handleResize);
  }

  document.addEventListener("DOMContentLoaded", init);
})();
