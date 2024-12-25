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
  console.log(event.data);
  // Update the entire document body with the received data
  document.body.innerHTML = `<div id="content">${event.data}</div>`;

  renderMermaid();
};

// Optional: Handle WebSocket errors
ws.onerror = function (event) {
  console.error("WebSocket error observed:", event);
};

// Optional: Handle WebSocket connection close
ws.onclose = function (event) {
  window.close();
  console.log("WebSocket connection closed:", event);
};
