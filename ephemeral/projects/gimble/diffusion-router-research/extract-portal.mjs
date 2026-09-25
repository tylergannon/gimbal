import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import vm from 'node:vm';

// The Router's Connect guides are currently bundled as the static iO array.
// Evaluate only that array in a constrained context, then write each guide as
// JSON so the source text and code snippets remain exact and machine readable.
const bundle = readFileSync(process.argv[2], 'utf8');
const start = bundle.indexOf('iO=[');
const end = bundle.indexOf('];new Map(iO.map', start);
if (start < 0 || end < 0) throw new Error('Connect guide array not found');
const expression = bundle.slice(start + 3, end + 1);
const context = {
  rO: {
    openAIBaseURL: 'https://router.diffusion.io/v1',
    anthropicBaseURL: 'https://router.diffusion.io',
    keyEnv: 'DIFFUSION_API_KEY',
    consoleKeysURL: '/access',
    defaultModel: 'deepseek-4.1-flash',
  },
  $: 'DIFFUSION_API_KEY',
  nO: 'Full guide qualification on the current production release is pending.',
};
const guides = vm.runInNewContext(`(${expression})`, context, { timeout: 1000 });
if (!Array.isArray(guides) || guides.length < 10) throw new Error('Unexpected guide inventory');
const output = resolve(process.argv[3]);
function block(value) {
  if (!value) return '';
  const lines = [];
  if (value.title) lines.push(`### ${value.title}`, '');
  if (value.body) lines.push(value.body, '');
  if (value.code) {
    if (value.code.path) lines.push(`Path: \`${value.code.path}\``, '');
    lines.push(`\`\`\`${value.code.language || ''}`, value.code.text, '\`\`\`', '');
  }
  return lines.join('\n');
}
function markdown(guide) {
  const url = `https://connect.diffusion.io/connect?guide=${guide.id}`;
  const lines = [
    `# ${guide.name}`,
    '',
    `Source: ${url}`,
    `Group: ${guide.group} | Status: ${guide.status} | Portal review: ${guide.lastReviewedAt}`,
    '',
    guide.summary,
    '',
    ...(guide.status === 'beta' ? [context.nO, ''] : []),
    guide.statusNote,
    '',
    '## Prerequisites',
    '',
    ...guide.prerequisites.map(item => `- ${item}`),
    '',
    '## Setup',
    '',
    ...guide.setup.flatMap(item => [block(item)]),
    '## Restart',
    '',
    guide.restart,
    '',
    '## Verification',
    '',
    block(guide.verification),
    '## Troubleshooting',
    '',
    ...guide.troubleshooting.map(item => `- ${item}`),
    '',
    '## Removal',
    '',
    block(guide.removal),
    '## Rollback',
    '',
    ...guide.rollback.map(item => `- ${item}`),
    '',
  ];
  return lines.join('\n');
}
for (const guide of guides) {
  const group = guide.group.toLowerCase().replaceAll(/[^a-z0-9]+/g, '-');
  const directory = resolve(output, group);
  mkdirSync(directory, { recursive: true });
  writeFileSync(resolve(directory, `${guide.id}.json`), JSON.stringify(guide, null, 2) + '\n');
  writeFileSync(resolve(directory, `${guide.id}.md`), markdown(guide));
}
writeFileSync(resolve(output, 'inventory.json'), JSON.stringify(guides.map(({id, group, name, status, lastReviewedAt}) => ({id, group, name, status, lastReviewedAt})), null, 2) + '\n');
console.log(JSON.stringify(guides.map(g => ({id:g.id, group:g.group, keys:Object.keys(g)})), null, 2));
