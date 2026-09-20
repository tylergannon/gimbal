import { access, readdir } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { run } from "@mermaid-js/mermaid-cli";

const root = fileURLToPath(new URL("..", import.meta.url));
const diagrams = path.join(root, "src/lib/generated/workflows");

const executablePath = await findBrowser();
const inputs = (await readdir(diagrams)).filter((file) => file.endsWith(".mmd")).sort();

for (const file of inputs) {
  const name = path.basename(file, ".mmd");
  await run(path.join(diagrams, file), path.join(diagrams, `${name}.svg`), {
    quiet: true,
    puppeteerConfig: { executablePath },
    parseMMDOptions: {
      backgroundColor: "transparent",
      svgId: `workflow-${name}`,
      mermaidConfig: {
        look: "classic",
        handDrawnSeed: 1,
        theme: "base",
        securityLevel: "strict",
        fontFamily: "Inter, ui-sans-serif, sans-serif",
        flowchart: { curve: "basis", htmlLabels: true, useMaxWidth: false },
        themeVariables: {
          background: "transparent",
          primaryColor: "#292524",
          primaryTextColor: "#fafaf9",
          primaryBorderColor: "#78716c",
          lineColor: "#a8a29e",
          clusterBkg: "#1c1917",
          clusterBorder: "#57534e",
          edgeLabelBackground: "#1c1917",
          tertiaryColor: "#1c1917",
        },
      },
    },
  });
}

async function findBrowser() {
  const candidates = [
    process.env.PUPPETEER_EXECUTABLE_PATH,
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    "/Applications/Chromium.app/Contents/MacOS/Chromium",
    "/usr/bin/google-chrome",
    "/usr/bin/chromium",
    "/usr/bin/chromium-browser",
  ].filter(Boolean);

  for (const candidate of candidates) {
    try {
      await access(candidate);
      return candidate;
    } catch {
      // Try the next conventional location.
    }
  }
  throw new Error(
    "Chrome or Chromium is required to render workflow diagrams. Set PUPPETEER_EXECUTABLE_PATH.",
  );
}
