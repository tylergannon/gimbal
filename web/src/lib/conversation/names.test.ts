import { expect, test } from "vite-plus/test";
import { harnessName, modelName } from "./names";

test("models read as family and version, never a raw id", () => {
  expect(modelName("claude-opus-5-5")).toBe("Opus 5.5");
  expect(modelName("claude-opus-5")).toBe("Opus 5");
  expect(modelName("claude-haiku-4-5-20251001")).toBe("Haiku 4.5");
  expect(modelName("gpt-6-luna")).toBe("Luna 6");
  expect(modelName("gpt-5.6-terra")).toBe("Terra 5.6");
  expect(modelName("gemini-3.8-flash-low")).toBe("Flash 3.8 Low");
  expect(modelName("o4-mini")).toBe("o4-mini");
});

test("harnesses use their product names", () => {
  expect(harnessName("claude")).toBe("Claude Code");
  expect(harnessName("codex")).toBe("Codex");
  expect(harnessName("agy")).toBe("Antigravity");
});
