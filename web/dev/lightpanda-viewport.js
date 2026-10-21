// Vitest sizes its iframe through CSS variables. Lightpanda's geometry needs
// the same viewport as explicit attributes, including later viewport changes.
function updateViewport() {
  const body = document.body;
  if (!body) return;
  const width = Number.parseInt(body.style.getPropertyValue("--viewport-width"), 10);
  const height = Number.parseInt(body.style.getPropertyValue("--viewport-height"), 10);
  if (!width || !height) return;
  for (const iframe of document.querySelectorAll("iframe[data-vitest]")) {
    iframe.width = String(width);
    iframe.height = String(height);
  }
}

new MutationObserver(updateViewport).observe(document, {
  childList: true,
  subtree: true,
  attributes: true,
  attributeFilter: ["style"],
});
updateViewport();
