import { test, expect } from '@playwright/test'
import { spawn, type ChildProcessWithoutNullStreams } from 'node:child_process'
import { createServer } from 'node:net'
import { mkdtemp, readdir, readFile, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'

const repo = resolve(import.meta.dirname, '../../../..')

async function freePort() {
	const server = createServer()
	await new Promise<void>((resolve, reject) => server.listen(0, '127.0.0.1', resolve).once('error', reject))
	const address = server.address()
	if (!address || typeof address === 'string') throw new Error('proof listener did not select a TCP port')
	await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()))
	return address.port
}

async function line(child: ChildProcessWithoutNullStreams, prefix: string, diagnostics: () => string) {
	let output = ''
	return await new Promise<string>((resolve, reject) => {
		const read = (chunk: Buffer) => {
			output += chunk.toString()
			const found = output.split('\n').find(value => value.startsWith(prefix))
			if (found) { child.stdout.off('data', read); resolve(found.slice(prefix.length)) }
		}
		child.stdout.on('data', read)
		child.once('exit', code => reject(new Error(`proof process exited ${code}: ${output}\n${diagnostics()}`)))
	})
}

async function jsonlBytes(path: string): Promise<number> {
	let total = 0
	for (const entry of await readdir(path, { withFileTypes: true })) {
		const child = join(path, entry.name)
		if (entry.isDirectory()) total += await jsonlBytes(child)
		else if (entry.name.endsWith('.jsonl')) total += (await readFile(child)).byteLength
	}
	return total
}

test('production runtime streams, resets, reconnects and renders native content', async ({ browser }) => {
	const project = await mkdtemp(join(tmpdir(), 'gimble-observation-proof-'))
	const port = await freePort()
	const child = spawn('go', ['run', './ephemeral/research/issue-130/proof', '-mode=deterministic', `-project=${project}`, `-port=${port}`], { cwd: repo, stdio: ['ignore', 'pipe', 'pipe'] })
	let stderr = ''; child.stderr.on('data', chunk => stderr += chunk.toString())
	try {
		const [url, runID] = (await line(child, 'PROOF_READY=', () => stderr)).split('|')
		const path = `${url}/runs/${encodeURIComponent(runID)}`

		const noJS = await browser.newContext({ javaScriptEnabled: false })
		const ssr = await noJS.newPage(); const ssrStarted = performance.now(); const response = await ssr.goto(path); const ssrNavigationMs = performance.now() - ssrStarted
		if (response?.status() !== 200) throw new Error(`SSR status ${response?.status()}: ${await ssr.textContent('body')}\nserver:\n${stderr}`)
		await expect(ssr.getByText('deterministic observation proof').first()).toBeVisible()
		const ssrBytes = Buffer.byteLength(await response.body())
		await noJS.close()

		const context = await browser.newContext()
		const page = await context.newPage()
		const deltaBytes: number[] = []
		await page.goto(path)
		const capture = new AbortController()
		const stream = await fetch(`${url}/api/runs/${encodeURIComponent(runID)}/events`, { signal: capture.signal })
		void (async () => {
			const reader = stream.body!.getReader(), decoder = new TextDecoder(); let buffered = ''
			for (;;) {
				const { value, done } = await reader.read(); if (done) return
				buffered += decoder.decode(value, { stream: true })
				for (;;) {
					const at = buffered.indexOf('\n\n'); if (at < 0) break
					const frame = buffered.slice(0, at); buffered = buffered.slice(at + 2)
					const data = frame.split('\n').find(line => line.startsWith('data: '))?.slice(6)
					if (data && String(JSON.parse(data).event?.type ?? '').endsWith('.delta')) deltaBytes.push(Buffer.byteLength(data))
				}
			}
		})().catch(() => {})
		const partialStarted = performance.now()
		await writeFile(join(project, 'start'), '')
		await expect(page.getByText(/draft-proof-native-/).first()).toBeVisible()
		await expect(page.getByText(/PROOF_TOOL_PROGRESS_proof-native-/).first()).toBeVisible()
		const partialIDs = await page.locator('[data-message-id]').evaluateAll(nodes => nodes.map(node => node.getAttribute('data-message-id')).filter(Boolean))
		expect(new Set(partialIDs).size).toBeGreaterThanOrEqual(2)
		const partialUpdateMs = performance.now() - partialStarted
		await expect.poll(() => deltaBytes.length).toBeGreaterThanOrEqual(4)

		await page.reload()
		await expect(page.getByText(/draft-proof-native-/).first()).toBeVisible()
		await expect(page.getByText(/PROOF_TOOL_PROGRESS_proof-native-/).first()).toBeVisible()
		await writeFile(join(project, 'finish'), '')
		await expect(page.getByText(/FINAL_TEXT_proof-native-/)).toHaveCount(2)
		await expect(page.getByText(/FINAL_REASONING_proof-native-/)).toHaveCount(2)
		await expect(page.getByText(/FINAL_TOOL_proof-native-/)).toHaveCount(2)
		await expect(page.locator('.run-header .status')).toHaveText('completed')
		const snapshotResponse = await page.request.get(`${url}/api/runs/${encodeURIComponent(runID)}`)
		const snapshotBody = await snapshotResponse.body()
		const measurements = {
			rawHistoryBytes: await jsonlBytes(join(project, 'runs', runID)),
			reducedStateBytes: snapshotBody.byteLength,
			ssrBytes,
			ssrNavigationMs,
			deltaBytes,
			partialDeltaBatchUpdateMs: partialUpdateMs
		}
		await writeFile(join(project, 'measurements.json'), JSON.stringify(measurements, null, 2))
		console.log(`MEASUREMENTS=${JSON.stringify(measurements)}`)
		capture.abort()
		await page.screenshot({ path: join(project, 'final.png'), fullPage: true })
		await context.close()
		await writeFile(join(project, 'stop'), '')
		await new Promise<void>((resolve, reject) => child.once('exit', code => code === 0 ? resolve() : reject(new Error(`proof exited ${code}: ${stderr}`))))
	} finally {
		if (child.exitCode === null) child.kill('SIGTERM')
	}
})
