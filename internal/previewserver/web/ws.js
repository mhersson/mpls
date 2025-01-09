document.addEventListener("DOMContentLoaded", () => {
  const ws = new WebSocket("ws://localhost:%d/ws");

  let debounceTimeout;

  function debounce(func, delay) {
    return function (...args) {
      clearTimeout(debounceTimeout);
      debounceTimeout = setTimeout(() => func.apply(this, args), delay);
    };
  }

  const updateTitle = debounce((title) => {
    document.title = title;
  }, 200);

  const renderMermaid = async () => {
    const mermaidElements = document.querySelectorAll(".language-mermaid");
    if (mermaidElements.length > 0) {
      await window.mermaid.run({
        querySelector: ".language-mermaid",
      });
    }
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

    if (title !== document.title) {
      updateTitle(title);
    }

    // console.log(renderedHtml);

    document.body.innerHTML = `<div id="content">${renderedHtml}</div>`;

    renderMermaid();

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

  ws.onclose = function (event) {
    window.close();
    console.log("WebSocket connection closed:", event);
  };
});
