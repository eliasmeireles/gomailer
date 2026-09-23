const POLL_INTERVAL_MS = 3000;

async function loadPanel(element) {
  try {
    const response = await fetch(element.dataset.src);
    element.innerHTML = await response.text();
  } catch (error) {
    element.innerHTML = `<div class="notice error">Falha ao atualizar: ${error.message}</div>`;
  }
}

function refreshPanels() {
  if (document.hidden) return;
  document.querySelectorAll("[data-poll]").forEach(loadPanel);
}

function setupPreview() {
  const editor = document.getElementById("html");
  const preview = document.getElementById("html-preview");
  const render = () => { preview.srcdoc = editor.value; };
  editor.addEventListener("input", render);
  render();
}

function setupForm() {
  const form = document.getElementById("send-form");
  const result = document.getElementById("result");
  const button = document.getElementById("send-button");
  const html = document.getElementById("html");
  const invalidBody = form.elements.invalidBody;

  invalidBody.addEventListener("change", () => { html.required = !invalidBody.checked; });

  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    button.disabled = true;
    try {
      const response = await fetch("/send", { method: "POST", body: new FormData(form) });
      result.innerHTML = await response.text();
      refreshPanels();
    } catch (error) {
      result.innerHTML = `<div class="notice error">Falha ao publicar: ${error.message}</div>`;
    } finally {
      button.disabled = false;
    }
  });
}

function setupClearButtons() {
  document.querySelectorAll("[data-clear]").forEach((button) => {
    button.addEventListener("click", async () => {
      const target = document.getElementById(button.dataset.target);
      const response = await fetch(button.dataset.clear, { method: "POST" });
      target.innerHTML = await response.text();
    });
  });
}

setupPreview();
setupForm();
setupClearButtons();
refreshPanels();
setInterval(refreshPanels, POLL_INTERVAL_MS);
