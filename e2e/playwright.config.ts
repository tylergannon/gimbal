import { defineConfig, devices } from '@playwright/test';
import { defineBddConfig } from 'playwright-bdd';
import { fileURLToPath } from 'node:url';

const run = process.env.SKGO_E2E_RUN ?? 'run';
const artifacts = process.env.SKGO_E2E_ARTIFACTS ?? '.';
const testDir = defineBddConfig({
	features: 'features/**/*.feature',
	steps: 'steps/**/*.ts',
	outputDir: '.features-gen'
});

export default defineConfig({
	testDir,
	fullyParallel: false,
	workers: 1,
	retries: 0,
	reporter: [
		['list'],
		['html', { open: 'never', outputFolder: `${artifacts}/playwright-report/${run}` }]
	],
	outputDir: `${artifacts}/test-results/${run}`,
	webServer: process.env.BASE_URL
		? undefined
		: {
				command: 'bin/gimble --port 0',
				cwd: fileURLToPath(new URL('..', import.meta.url)),
				env: { GIMBLE_WEB_PROXY: '', GIMBLE_WEB_ORIGIN: '' },
				wait: {
					stderr:
						/gimble: web application listening on 127\.0\.0\.1:(?<gimble_e2e_port>\d+) \(prod\)/
				},
				gracefulShutdown: { signal: 'SIGINT', timeout: 5_000 }
			},
	use: {
		trace: 'retain-on-failure'
	},
	projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }]
});
