/** Agent prose may cite checkout files that the document viewer cannot open.
 * Retain their readable labels without navigating the workspace to a file URL.
 * This only annotates server-rendered Markdown; it grants no file access. */
export function sourceReferences(node: HTMLElement, html: string) {
  function update(_html: string) {
    for (const link of node.querySelectorAll<HTMLAnchorElement>("a[href]")) {
      const href = link.getAttribute("href") ?? "";
      if (/^(https?:|mailto:|#)/i.test(href)) continue;
      link.removeAttribute("href");
      link.title = `Source reference: ${href} (local file opening unavailable)`;
    }
  }
  update(html);
  return { update };
}
