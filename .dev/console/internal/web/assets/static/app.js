const POLL_INTERVAL_MS = 3000;

async function replaceWith(element, response) {
  element.innerHTML = await response.text();
}

async function loadPanel(element) {
  try {
    await replaceWith(element, await fetch(element.dataset.src));
  } catch (error) {
    element.innerHTML = `<div class="notice error">Refresh failed: ${error.message}</div>`;
  }
}

function refreshPolledPanels() {
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

function setupSendForm() {
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
      await replaceWith(result, await fetch("/send", { method: "POST", body: new FormData(form) }));
      refreshPolledPanels();
    } catch (error) {
      result.innerHTML = `<div class="notice error">Send failed: ${error.message}</div>`;
    } finally {
      button.disabled = false;
    }
  });
}

// Panels rendered by the server contain forms (data-action) and buttons (data-clear) that post
// to a partial endpoint and replace the element named by data-target with the response.
function setupPartialActions() {
  document.addEventListener("submit", async (event) => {
    const form = event.target.closest("form[data-action]");
    if (!form) return;
    event.preventDefault();
    const target = document.getElementById(form.dataset.target);
    await replaceWith(target, await fetch(form.dataset.action, { method: "POST", body: new FormData(form) }));
  });

  document.addEventListener("click", async (event) => {
    const button = event.target.closest("[data-clear]");
    if (!button) return;
    event.preventDefault();
    const target = document.getElementById(button.dataset.target);
    await replaceWith(target, await fetch(button.dataset.clear, { method: "POST" }));
    refreshPolledPanels();
  });
}

setupPreview();
setupSendForm();
setupPartialActions();
document.querySelectorAll("[data-src]").forEach(loadPanel);
setInterval(refreshPolledPanels, POLL_INTERVAL_MS);
