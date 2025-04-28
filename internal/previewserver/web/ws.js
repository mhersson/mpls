document.addEventListener("DOMContentLoaded", () => {
  const ws = new WebSocket("ws://localhost:%d/ws");

  const DEBOUNCE_DELAY = 1000;
  let debounceTimeout;
  let isReloading = false;

  const themeToggle = document.getElementById("theme-toggle");
  const themeStylesheet = document.getElementById("theme-stylesheet");

  function setCookie(name, value, days) {
    let expires = "";
    if (days) {
      const date = new Date(Date.now() + days * 864e5);
      expires = "; expires=" + date.toUTCString();
    }
    document.cookie = `${name}=${encodeURIComponent(value)}${expires}; path=/; SameSite=Lax`;
  }

  function getCookie(name) {
    const match = document.cookie.match(
      new RegExp("(^| )" + encodeURIComponent(name) + "=([^;]*)(;|$)"),
    );
    return match ? decodeURIComponent(match[2]) : null;
  }

  const theme = getCookie("theme");
  if (theme) {
    themeStylesheet.href =
      theme === "dark" ? "colors-dark.css" : "colors-light.css";
    mermaid.initialize({
      startOnLoad: false,
      theme: theme === "dark" ? "dark" : "default",
    });

    themeToggle.checked = theme === "dark";
  }

  themeToggle.addEventListener("change", () => {
    const newTheme = themeToggle.checked ? "dark" : "light";
    themeStylesheet.href =
      newTheme === "dark" ? "colors-dark.css" : "colors-light.css";
    mermaid.initialize({
      startOnLoad: false,
      theme: newTheme === "dark" ? "dark" : "default",
    });

    setCookie("theme", newTheme, 365);
  });

  function debounce(func, delay) {
    return function (...args) {
      clearTimeout(debounceTimeout);
      debounceTimeout = setTimeout(() => func.apply(this, args), delay);
    };
  }

  const saveContentToLocalStorage = debounce((renderedHtml) => {
    localStorage.setItem("savedContent", renderedHtml);
  }, DEBOUNCE_DELAY);

  function updateContent(renderedHtml) {
    document.getElementById("content").innerHTML = renderedHtml;
    // console.log(renderedHtml);
  }

  function initializeModal() {
    // Image modal setup
    const images = document.querySelectorAll("img");
    images.forEach((img) => {
      img.addEventListener("click", function () {
        showModal("image", img);
      });
    });

    // Mermaid modal setup (with timeout for rendering)
    setTimeout(() => {
      const mermaidDivs = document.querySelectorAll(".language-mermaid");
      mermaidDivs.forEach((div) => {
        const svg = div.querySelector("svg");
        if (svg) {
          div.addEventListener("click", function () {
            showModal("mermaid", svg);
          });
        }
      });
    }, 1500);

    const modal = document.getElementById("contentModal");
    const closeModal = document.getElementById("closeModal");

    function showModal(type, content) {
      const mermaidContent = document.getElementById("mermaidContent");
      const imageContent = document.getElementById("imageContent");

      mermaidContent.style.display = "none";
      imageContent.style.display = "none";

      if (type === "image") {
        imageContent.src = content.src;
        imageContent.style.display = "flex";
      } else if (type === "mermaid") {
        mermaidContent.innerHTML = "";
        const svgClone = content.cloneNode(true);
        svgClone.classList.add("mermaid-modal-svg");
        mermaidContent.appendChild(svgClone);
        mermaidContent.style.display = "flex";
      }

      modal.style.display = "flex";
    }

    closeModal.addEventListener("click", function () {
      modal.style.display = "none";
    });

    // Close modal when clicking outside of the content
    modal.addEventListener("click", function (event) {
      if (event.target === modal) {
        modal.style.display = "none";
      }
    });
  }
  const renderMermaid = async () => {
    const mermaidElements = document.querySelectorAll(".language-mermaid");
    if (mermaidElements.length > 0) {
      await window.mermaid.run({
        querySelector: ".language-mermaid",
      });
    }
  };

  window.addEventListener("load", function () {
    const savedContent = localStorage.getItem("savedContent");
    if (savedContent) {
      updateContent(savedContent);
    }
  });

  window.addEventListener("beforeunload", function () {
    isReloading = true;
  });

  ws.onclose = function (event) {
    if (!isReloading) {
      window.close();
    }
    console.log("WebSocket connection closed:", event);
  };

  ws.onopen = function (event) {
    console.log("WebSocket connection established");
  };

  ws.onmessage = function (event) {
    console.log("Got WebSocket message");

    const response = JSON.parse(event.data);
    const renderedHtml = response.HTML;
    const section = response.Section;
    const title = "mpls - " + response.Title;
    const meta = response.Meta;

    const pin = document.getElementById("pin");
    if (pin.checked && title !== document.title) {
      console.log("Preview is pinned - ignoring event");
      return;
    }

    if (title !== document.title) {
      document.getElementById("header-summary").innerText = response.Title;
      document.title = title;
    }

    const headerMeta = document.getElementById("header-meta");
    if (headerMeta !== meta) {
      document.getElementById("header-meta").innerHTML = meta;
    }

    updateContent(renderedHtml);
    saveContentToLocalStorage(renderedHtml);

    renderMermaid();
    initializeModal();

    if (section) {
      const targetElement = document.querySelector(`#${section}`);

      if (targetElement) {
        targetElement.scrollIntoView({ behavior: "smooth", block: "start" });

        const offset = 50;
        const elementRect = targetElement.getBoundingClientRect();
        const elementTop = elementRect.top + window.scrollY;

        window.scrollTo({
          top: elementTop - offset,
          behavior: "smooth",
        });
      } else {
        console.warn("Target element not found for cursor position:", section);
      }
    }
  };
});
