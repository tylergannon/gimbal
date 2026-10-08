// Combine the built viewer with gimbalgen's static source projection.
import { readFile, writeFile } from 'node:fs/promises';
const [input, output] = process.argv.slice(2);
if (!input || !output) throw new Error('Usage: npm run page -- workflow.json workflow.html');
const page = JSON.parse(await readFile(input, 'utf8'));
const template = await readFile(new URL('./dist/index.html', import.meta.url), 'utf8');
const safeJSON = JSON.stringify(page).replaceAll('<', '\\u003c').replaceAll('>', '\\u003e').replaceAll('&', '\\u0026');
await writeFile(output, template.replace('"SOURCE_DATA_PLACEHOLDER"', () => safeJSON));
