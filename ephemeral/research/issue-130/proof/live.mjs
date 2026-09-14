import { chromium } from '@playwright/test'
import { spawn } from 'node:child_process'
import { mkdtemp, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { createServer } from 'node:net'
const mode = process.argv[2] ?? 'codex'
const project = await mkdtemp(join(tmpdir(), `gimble-live-${mode}-`))
const listener = createServer()
await new Promise(resolve => listener.listen(0, '127.0.0.1', resolve))
const port = listener.address().port
await new Promise(resolve => listener.close(resolve))
const child = spawn('/tmp/gimble-observation-proof', [`-mode=${mode}`, `-project=${project}`, `-port=${port}`])
let stdout = '', stderr = ''
child.stdout.on('data', data => stdout += data)
child.stderr.on('data', data => stderr += data)
const browser = await chromium.launch({ channel: 'chrome', headless: true })
try {
  for (let i = 0; !stdout.includes('PROOF_READY=') && i < 300; i++) {
    if (child.exitCode !== null) throw new Error(stderr)
    await new Promise(resolve => setTimeout(resolve, 100))
  }
  const ready = stdout.split('\n').find(line => line.startsWith('PROOF_READY='))
  if (!ready) throw new Error('Runtime never became ready')
  const [origin, runID] = ready.slice('PROOF_READY='.length).split('|')
  const page = await browser.newPage()
  await page.addInitScript(() => {
    window.observedFrames = []
    const Native = window.EventSource
    window.EventSource = class extends Native {
      constructor(...args) {
        super(...args)
        for (const type of ['snapshot', 'event', 'lifecycle']) this.addEventListener(type, event => window.observedFrames.push({ type, data: JSON.parse(event.data) }))
      }
    }
  })
  await page.goto(`${origin}/runs/${encodeURIComponent(runID)}`)
  await page.locator('.status.completed, .status.failed, .status.cancelled').waitFor({ timeout: 120000 })
  const markers = mode === 'issue135'
    ? ['CODEX_FIRST_TOOL_MARKER', 'CODEX_FIRST_FINAL_MARKER', 'CODEX_SECOND_TOOL_MARKER', 'CODEX_SECOND_FINAL_MARKER', 'CODEX_SERIAL_FIRST_MARKER', 'CODEX_SERIAL_SECOND_MARKER', 'CODEX_SERIAL_FINAL_MARKER', 'CODEX_FORKED_TOOL_MARKER', 'CODEX_FORKED_FINAL_MARKER', 'CLAUDE_HAIKU_TOOL_MARKER', 'CLAUDE_HAIKU_FINAL_MARKER']
    : ['GIMBLE_LIVE_TOOL_MARKER', 'GIMBLE_LIVE_FINAL_MARKER']
  for (const marker of markers) await page.locator('article.assistant').filter({ hasText: marker }).waitFor({ timeout: 5000 })
  const body = await page.locator('body').innerText()
  const assistantText = (await page.locator('article.assistant').allInnerTexts()).join('\n')
  const articles = await page.locator('article.assistant').evaluateAll(nodes => nodes.map(node => ({ id: node.getAttribute('data-message-id'), text: node.textContent })))
  const frames = await page.evaluate(() => window.observedFrames)
  const snapshot = await (await page.request.get(`${origin}/api/runs/${encodeURIComponent(runID)}`)).json()
  await writeFile(join(project, 'browser.json'), JSON.stringify({ body, articles, frames, snapshot }, null, 2))
  await page.screenshot({ path: join(project, 'final.png'), fullPage: true })
  if (snapshot.run.status !== 'completed' || markers.some(marker => !assistantText.includes(marker))) throw new Error(`Live observation failed: ${snapshot.run.status}\n${body}`)
  if (mode === 'issue135') {
    for (const marker of ['CODEX_FIRST_TOOL_MARKER', 'CODEX_SECOND_TOOL_MARKER', 'CODEX_SERIAL_FIRST_MARKER', 'CODEX_FORKED_TOOL_MARKER']) {
      const row = articles.find(article => article.text?.includes(marker))
      const tokens = row?.text?.match(/(\d+) in · (\d+) out · (\d+) reasoning · (\d+) cache read · (\d+) cache write/)
      if (!tokens || tokens.slice(1).map(Number).reduce((sum, value) => sum + value, 0) === 0) throw new Error(`Codex marker row has no model-call tokens: ${marker}\n${row?.text}`)
    }
    for (const [toolMarker, finalMarker] of [['CODEX_FIRST_TOOL_MARKER', 'CODEX_FIRST_FINAL_MARKER'], ['CODEX_SECOND_TOOL_MARKER', 'CODEX_SECOND_FINAL_MARKER'], ['CODEX_SERIAL_FIRST_MARKER', 'CODEX_SERIAL_FINAL_MARKER']]) {
      const toolRow = articles.find(article => article.text?.includes(toolMarker))
      const finalRow = articles.find(article => article.text?.includes(finalMarker))
      if (!toolRow?.id || !finalRow?.id || toolRow.id === finalRow.id) throw new Error(`Codex tool and follow-up response did not render in distinct native response rows: ${toolMarker}`)
    }
    const serialRows = articles.filter(article => article.text?.includes('CODEX_SERIAL_FIRST_MARKER') || article.text?.includes('CODEX_SERIAL_SECOND_MARKER'))
    if (serialRows.length !== 1 || !serialRows[0].text?.includes('CODEX_SERIAL_FIRST_MARKER') || !serialRows[0].text?.includes('CODEX_SERIAL_SECOND_MARKER')) throw new Error(`Serial tools did not render in one native response row: ${JSON.stringify(serialRows)}`)
  }
  console.log(JSON.stringify({ project, runID, status: snapshot.run.status, frames: frames.length }))
} finally {
  await writeFile(join(project, 'stdout.log'), stdout)
  await writeFile(join(project, 'stderr.log'), stderr)
  await writeFile(join(project, 'stop'), '')
  await browser.close()
  if (child.exitCode === null) child.kill('SIGTERM')
  console.log(`Artifacts: ${project}`)
}
