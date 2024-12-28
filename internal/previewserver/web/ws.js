const ws = new WebSocket("ws://localhost:%d/ws");

const renderMermaid = async () => {
  if (typeof window.mermaid !== "undefined") {
    await window.mermaid.run({
      querySelector: ".language-mermaid",
    });
  } else {
    console.error("Mermaid is not defined yet.");
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

  //console.log(renderedHtml);

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

ws.onerror = function (event) {
  console.error("WebSocket error observed:", event);
};

ws.onclose = function (event) {
  window.close();
  console.log("WebSocket connection closed:", event);
};
