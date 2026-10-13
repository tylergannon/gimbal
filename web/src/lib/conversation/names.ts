const harnesses: Record<string, { name: string; mark: string }> = {
  claude: { name: "Claude Code", mark: "C" },
  codex: { name: "Codex", mark: "X" },
  agy: { name: "Antigravity", mark: "A" },
};

export const harnessName = (id: string) => harnesses[id]?.name ?? id;
export const harnessMark = (id: string) => harnesses[id]?.mark ?? id.slice(0, 1).toUpperCase();

const word = (value: string) => value.charAt(0).toUpperCase() + value.slice(1);

/** A native model id as people read it: claude-opus-5-5 is "Opus 5.5". */
export function modelName(id: string): string {
  const model = id.replace(/-\d{8}$/, "");
  let match = /^claude-([a-z]+)-(\d+)(?:-(\d+))?$/.exec(model);
  if (match) return `${word(match[1])} ${match[2]}${match[3] ? `.${match[3]}` : ""}`;
  match = /^gpt-([\d.]+)-([a-z]+)$/.exec(model);
  if (match) return `${word(match[2])} ${match[1]}`;
  match = /^gemini-([\d.]+)-([a-z]+)(?:-(low|medium|high))?$/.exec(model);
  if (match) return `${word(match[2])} ${match[1]}${match[3] ? ` ${word(match[3])}` : ""}`;
  return id;
}
