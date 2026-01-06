document.addEventListener("DOMContentLoaded", () => {
  const ws = new WebSocket("ws://localhost:%d/ws");

  const MERMAID_RENDER_DELAY = 100;
  const SCROLL_OFFSET = 150;
  const SCROLL_RETRY_DELAY = 50;
  const MAX_SCROLL_RETRIES = 10;

  let isReloading = false;
  let lastScrollTarget = null;
  let enableTabsMode = false; // Global variable for preview mode

  // Utility functions
  function $(id) {
    return document.getElementById(id);
  }

  function $$(selector) {
    return document.querySelectorAll(selector);
  }

  // Content management
  function updateContent(renderedHtml) {
    const contentElement = $("content");
    if (contentElement) {
      contentElement.innerHTML = renderedHtml;
    }
  }

  // Modal functionality
  function createModalHandler() {
    const modal = $("contentModal");
    const closeModal = $("closeModal");
    const mermaidContent = $("mermaidContent");
    const imageContent = $("imageContent");

    if (!modal || !closeModal || !mermaidContent || !imageContent) {
      console.warn("Modal elements not found");
      return { showModal: () => {}, initializeModal: () => {} };
    }

    function showModal(type, content) {
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

      modal.style.display = "flex";
    }

    function closeModalHandler() {
      modal.style.display = "none";
      // Clean up modal content
      mermaidContent.innerHTML = "";
      imageContent.src = "";
    }

    // Event listeners
    closeModal.addEventListener("click", closeModalHandler);
    modal.addEventListener("click", (event) => {
      if (event.target === modal) {
        closeModalHandler();
      }
    });

    // Keyboard navigation
    document.addEventListener("keydown", (event) => {
      if (event.key === "Escape" && modal.style.display === "flex") {
        closeModalHandler();
      }
    });

    function initializeModal() {
      // Setup image modals
      $$("img").forEach((img) => {
        // Skip if already has handler
        if (img._modalHandlerAttached) return;

        img._modalHandler = () => showModal("image", img);
        img.addEventListener("click", img._modalHandler);
        img._modalHandlerAttached = true;
        img.style.cursor = "pointer";
      });

      // Setup Mermaid modals
      $$(".language-mermaid").forEach((div) => {
        const svg = div.querySelector("svg");
        if (svg && !div._modalHandlerAttached) {
          div._modalHandler = () => showModal("mermaid", svg);
          div.addEventListener("click", div._modalHandler);
          div._modalHandlerAttached = true;
          div.style.cursor = "pointer";
        }
      });
    }

    return { showModal, initializeModal };
  }

  // Mermaid rendering with measurement
  async function renderMermaid() {
    const mermaidElements = $$(".language-mermaid");
    if (mermaidElements.length === 0 || !window.mermaid) {
      return Promise.resolve();
    }

    // Store pre-render scroll position
    const scrollBefore = window.scrollY;

    try {
      // Mark elements as rendering
      mermaidElements.forEach((el) => {
        el.setAttribute("data-rendering", "true");
      });

      await window.mermaid.run({
        querySelector: ".language-mermaid",
      });

      // Remove rendering markers
      mermaidElements.forEach((el) => {
        el.removeAttribute("data-rendering");
      });

      // Return scroll adjustment needed
      return window.scrollY - scrollBefore;
    } catch (error) {
      console.error("Mermaid rendering failed:", error);
      mermaidElements.forEach((el) => {
        el.removeAttribute("data-rendering");
      });
      return 0;
    }
  }

  // Improved scroll functionality
  function scrollToEdit(retryCount = 0) {
    const disableScrolling = $("disable-scrolling");
    if (disableScrolling?.checked) {
      return;
    }

    const targetElement = $("mpls-scroll-anchor");
    if (!targetElement) {
      lastScrollTarget = null;
      return;
    }

    // Check if any mermaid diagrams are still rendering
    const renderingElements = $$('[data-rendering="true"]');
    if (renderingElements.length > 0 && retryCount < MAX_SCROLL_RETRIES) {
      // Retry after a short delay
      setTimeout(() => scrollToEdit(retryCount + 1), SCROLL_RETRY_DELAY);
      return;
    }

    // Store target for future reference
    lastScrollTarget = targetElement;

    const elementRect = targetElement.getBoundingClientRect();
    const elementTop = elementRect.top + window.scrollY;
    const targetScrollPosition = elementTop - SCROLL_OFFSET;

    // Only scroll if we're not already at the target position
    if (Math.abs(window.scrollY - targetScrollPosition) > 5) {
      window.scrollTo({
        top: targetScrollPosition,
        behavior: "smooth",
      });
    }
  }

  // Combined render and scroll function
  async function renderMermaidAndScroll() {
    // First render mermaid
    await renderMermaid();

    // Then initialize modals
    modalHandler.initializeModal();

    // Finally scroll to edit position
    // Use setTimeout to ensure DOM has settled
    setTimeout(() => {
      scrollToEdit();
    }, MERMAID_RENDER_DELAY);
  }

  // WebSocket event handlers
  async function handleWebSocketMessage(event) {
    try {
      const response = JSON.parse(event.data);

      // Handle config message
      if (response.Type === "config") {
        enableTabsMode = response.EnableTabs || false;
        console.log(`Preview mode: ${enableTabsMode ? 'multi-tab' : 'single-page'}`);
        return;
      }

      // Handle closeDocument message
      if (response.Type === "closeDocument") {
        // Only close window in multi-tab mode
        if (enableTabsMode && response.DocumentURI === window.location.pathname) {
          console.log(`Closing preview for ${response.DocumentURI}`);
          window.close();
        }
        return;
      }

      const { HTML: renderedHtml, Title: responseTitle, Meta: meta, DocumentURI: documentURI } = response;

      // If DocumentURI is provided, check if it matches current page
      if (documentURI && documentURI !== window.location.pathname) {
        console.log(`Ignoring update for ${documentURI}, current page is ${window.location.pathname}`);
        return;
      }

      const title = `mpls - ${responseTitle}`;

      // Update title if changed
      if (title !== document.title) {
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
      updateContent(renderedHtml);

      // Render and scroll
      await renderMermaidAndScroll();
    } catch (error) {
      console.error("Failed to process WebSocket message:", error);
    }
  }

  function handleWebSocketClose(event) {
    console.log("WebSocket connection closed:", event);
    if (!isReloading) {
      window.close();
    }
  }

  function handleWebSocketOpen() {
    console.log("WebSocket connection established");
    setupLinkInterception();
  }

  function handleWebSocketError(event) {
    console.error("WebSocket error:", event);
  }

  // Link interception for multi-file navigation
  function setupLinkInterception() {
    document.addEventListener("click", (event) => {
      const link = event.target.closest("a[href]");
      if (!link) return;

      const href = link.getAttribute("href");
      const isInternal = link.hasAttribute("data-mpls-internal");
      const target = link.getAttribute("data-mpls-target");

      // Anchor-only links - let browser handle
      if (href.startsWith("#")) {
        return;
      }

      // External links - let browser handle
      if (href.startsWith("http://") || href.startsWith("https://")) {
        return;
      }

      // Internal markdown links
      if (isInternal && target) {
        event.preventDefault();

        if (enableTabsMode) {
          // MULTI-TAB MODE: Send message to open in editor (browser tab opens via TextDocumentDidOpen)
          ws.send(
            JSON.stringify({
              type: "openDocument",
              uri: target,
              takeFocus: true,
            })
          );
        } else {
          // SINGLE-PAGE MODE: Send message to open in editor + update preview
          ws.send(
            JSON.stringify({
              type: "openDocument",
              uri: target,
              takeFocus: true,
              updatePreview: true,
            })
          );
        }
      }
    });
  }

  // Intersection Observer for lazy loading (optional enhancement)
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

  // Initialize modal handler
  const modalHandler = createModalHandler();

  // WebSocket setup
  ws.addEventListener("open", handleWebSocketOpen);
  ws.addEventListener("message", handleWebSocketMessage);
  ws.addEventListener("close", handleWebSocketClose);
  ws.addEventListener("error", handleWebSocketError);

  // Window event listeners
  window.addEventListener("load", async () => {
    setupLazyLoading();
    // Render mermaid diagrams that are already in the HTML from HTTP GET
    await renderMermaidAndScroll();
  });

  window.addEventListener("beforeunload", () => {
    isReloading = true;
  });

  // Handle window resize to maintain scroll position
  let resizeTimeout;
  window.addEventListener("resize", () => {
    clearTimeout(resizeTimeout);
    resizeTimeout = setTimeout(() => {
      if (lastScrollTarget && !$("disable-scrolling")?.checked) {
        scrollToEdit();
      }
    }, 250);
  });
});
