import { type Page } from '@playwright/test';
import { createBdd, test as base } from 'playwright-bdd';
import { mkdirSync } from 'node:fs';
import { join } from 'node:path';

export type BrowserState = {
	documents: number;
	remoteMark: number;
	remotes: string[];
	pageErrors: string[];
};

export const test = base.extend<{ browserState: BrowserState }>({
	baseURL: async ({}, use) => {
		await use(process.env.BASE_URL || `http://127.0.0.1:${process.env.GIMBLE_E2E_PORT}`);
	},
	browserState: [
		async ({ page }, use) => {
			const state: BrowserState = { documents: 0, remoteMark: 0, remotes: [], pageErrors: [] };
			page.on('request', (request) => {
				if (request.resourceType() === 'document') state.documents++;
				if (request.url().includes('/_app/remote/')) {
					state.remotes.push(`${request.method()} ${new URL(request.url()).pathname}`);
				}
			});
			page.on('pageerror', (error) => state.pageErrors.push(error.message));
			await use(state);
		},
		{ auto: true }
	]
});

const { AfterStep } = createBdd(test);

AfterStep(async ({ page, $bddContext, $testInfo }) => {
	const step = $bddContext.bddTestData?.steps?.[$bddContext.stepIndex];
	if (step?.keywordType !== 'Outcome') return;
	const directory = join(
		process.env.SKGO_E2E_ARTIFACTS ?? '.',
		'screenshots',
		process.env.SKGO_E2E_RUN ?? 'run'
	);
	mkdirSync(directory, { recursive: true });
	const file = join(
		directory,
		`${slug($testInfo.title)}-${String($bddContext.stepIndex + 1).padStart(2, '0')}.png`
	);
	await page.screenshot({ path: file, fullPage: true });
	await $testInfo.attach(step.textWithKeyword ?? $testInfo.title, { path: file, contentType: 'image/png' });
});

export async function hydrated(page: Page): Promise<void> {
	await page.waitForFunction(
		() => Object.keys(globalThis).some((key) => key.startsWith('__sveltekit_'))
	);
	await page.waitForFunction(() => history.scrollRestoration === 'manual');
}

function slug(text: string): string {
	return text
		.replace(/[^\w\s.-]/g, '')
		.trim()
		.replace(/\s+/g, '-')
		.slice(0, 80);
}
