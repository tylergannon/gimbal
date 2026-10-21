import { expect } from '@playwright/test';
import type { WorkflowMapElement } from '../../web/src/lib/run/graphReadiness.js';
import { createBdd } from 'playwright-bdd';
import {
	delayNextRunsData,
	hydrated,
	setRunsFixture,
	test,
	updateFailedRunSummary
} from './fixtures.js';

const { Given, When, Then } = createBdd(test);

Given('I open the project runs', async ({ page, browserState }) => {
	await page.goto('/');
	await hydrated(page);
	await page.locator('section.projects a').first().click();
	await hydrated(page);
	expect(browserState.documents).toBe(1);
});

Given('I open the controlled project runs', async ({ page, browserState }) => {
	managedFixtureOnly();
	await page.goto('/');
	await hydrated(page);
	await page.locator('section.projects a').first().click();
	await hydrated(page);
	await page.getByRole('button', { name: /All runs/ }).click();
	expect(browserState.documents).toBe(1);
});

Then('the recorded run shows its identity, failure and cost', async ({ page, browserState }) => {
  const card = page.getByRole('button', { name: 'Open implement fixture-failed', exact: true });
  await expect(card).toBeVisible();
  await expect(card.locator('strong')).toHaveText('implement');
  await expect(card.getByTitle('Failed', { exact: true })).toBeVisible();
  await expect(card.getByTitle('Elapsed', { exact: true })).toHaveText('1m 00s');
  await expect(card.getByTitle('Total cost', { exact: true })).toHaveText('$0.00');
  expect(browserState.pageErrors).toEqual([]);
});

When('I open the recorded fixture run', async ({ page }) => {
  await page.getByRole('button', { name: 'Open implement fixture-failed', exact: true }).click();
  await expect(page).toHaveURL(/\/runs\/fixture-failed$/);
});

Then('the recorded transcript is visible', async ({ page, browserState }) => {
  await expect(page.locator('.run-transcript')).toBeVisible();
  await expect(page.locator('.run-transcript .ended')).toHaveText('This run has ended.');
  expect(browserState.documents).toBe(1);
});

When('I open its workflow map and await the graph API', async ({ page, browserState }) => {
  await page.getByRole('button', { name: 'Map', exact: true }).first().click();
  const map = page.getByRole('region', { name: /workflow map/ });
  browserState.graphReady = await map.evaluate(element => (element as WorkflowMapElement).whenReady());
  browserState.graphElement = await map.elementHandle() ?? undefined;
  expect(browserState.graphReady.nodes).toBeGreaterThan(0);
});

Then('the graph shows the recorded scope', async ({ page }) => {
  const map = page.getByRole('region', { name: /workflow map/ });
  await expect(map).toBeVisible();
  const outcome = map.getByRole('button', { name: /^outcome,/ });
  await expect(outcome).toBeVisible();
  expect(await outcome.evaluate(element => element.closest('.node')?.getAttribute('data-selection-key'))).toContain('outcome.1');
});

When('I select that recorded scope', async ({ page }) => {
  await page.getByRole('region', { name: /workflow map/ }).getByRole('button', { name: /^outcome,/ }).click();
});

Then('the detail pane describes that scope', async ({ page }) => {
  const detail = page.locator('aside.detail');
  await expect(detail.getByRole('heading', { name: 'outcome.1', exact: true })).toBeVisible();
  await expect(detail.locator('.placement')).toHaveText('outcome.1');
});

When('I return to the transcript and reopen the map', async ({ page }) => {
  await page.getByRole('button', { name: '← Transcript', exact: true }).click();
  await expect(page.locator('.run-transcript')).toBeVisible();
  await page.getByRole('button', { name: 'Map', exact: true }).first().click();
  await page.getByRole('region', { name: /workflow map/ }).evaluate(element => (element as WorkflowMapElement).whenReady());
});

Then('the same graph and selection are retained without a document reload', async ({ page, browserState }) => {
  const map = page.getByRole('region', { name: /workflow map/ });
  expect(await map.evaluate((element, previous) => element === previous, browserState.graphElement!)).toBe(true);
  const ready = await map.evaluate(element => (element as WorkflowMapElement).whenReady());
  expect(ready).toEqual(browserState.graphReady);
  await expect(map.getByRole('button', { name: /^outcome,/ })).toHaveAttribute('aria-pressed', 'true');
  await expect(page.locator('aside.detail .placement')).toHaveText('outcome.1');
  expect(browserState.documents).toBe(1);
  expect(page.context().pages()).toHaveLength(1);
  expect(browserState.pageErrors).toEqual([]);
});

When('I follow the About link', async ({ page }) => {
	await page.getByRole('link', { name: 'About' }).click();
});

Then('About is visible without a document reload', async ({ page, browserState }) => {
	await expect(page).toHaveURL(/\/about$/);
	await expect(page.getByTestId('title')).toHaveText('About Gimbal');
	expect(browserState.documents).toBe(1);
	expect(browserState.pageErrors).toEqual([]);
});

When('I load the About route directly', async ({ page }) => {
	await page.goto(`${projectBase(page)}/about`);
});

Then('About is visible in a new document', async ({ page, browserState }) => {
	await expect(page.getByTestId('title')).toHaveText('About Gimbal');
	expect(browserState.documents).toBe(2);
	expect(browserState.pageErrors).toEqual([]);
});

Given('I open About with recorded runs available', async ({ page, browserState }) => {
	managedFixtureOnly();
	await page.goto('/');
	await hydrated(page);
	await page.locator('section.projects a').first().click();
	await page.getByRole('link', { name: 'About' }).click();
	await hydrated(page);
	expect(browserState.documents).toBe(1);
});

Given('I open About with an empty project', async ({ page, browserState }) => {
	managedFixtureOnly();
	setRunsFixture(false);
	await page.goto('/');
	await hydrated(page);
	await page.locator('section.projects a').first().click();
	await page.getByRole('link', { name: 'About' }).click();
	await hydrated(page);
	expect(browserState.documents).toBe(1);
});

When('I follow Runs while its data is delayed', async ({ page, browserState }) => {
	delayNextRunsData(page, browserState);
	browserState.pendingNavigation = page.getByRole('link', { name: 'Runs' }).click();
	await browserState.runsRequestSeen;
});

Then('accessible Runs loading cards remain until data arrives', async ({ page, browserState }) => {
	await expect(page.getByRole('status').filter({ hasText: 'Loading runs…' })).toBeVisible();
	await expect(page.getByRole('region', { name: 'Runs' })).toHaveAttribute('aria-busy', 'true');
	await expect(page.locator('[data-loading-card]')).toHaveCount(2);
	await expect(page.getByTestId('title')).toHaveCount(0);
	expect(browserState.documents).toBe(1);
	expect(browserState.pageErrors).toEqual([]);
});

When('the delayed Runs data arrives', async ({ browserState }) => {
	browserState.releaseRunsRequest?.();
	await browserState.pendingNavigation;
});

Then('recorded cards replace the loading feedback', async ({ page, browserState }) => {
	await expect(page.getByRole('status').filter({ hasText: 'Loading runs…' })).toHaveCount(0);
	await expect(page.getByRole('button', { name: 'Open implement fixture-failed' })).toBeVisible();
	expect(browserState.pageErrors).toEqual([]);
});

Then('the empty project replaces the loading feedback', async ({ page, browserState }) => {
	await expect(page.getByRole('status').filter({ hasText: 'Loading runs…' })).toHaveCount(0);
	await expect(page.getByText('No runs yet')).toBeVisible();
	expect(browserState.pageErrors).toEqual([]);
});

When('a background Runs refresh is delayed', async ({ page, browserState }) => {
	delayNextRunsData(page, browserState);
	await browserState.runsRequestSeen;
});

Then('the current Runs card stays visible and usable', async ({ page, browserState }) => {
	const card = page.getByRole('button', { name: 'Open implement fixture-failed' });
	await expect(card).toBeVisible();
	await card.focus();
	await expect(card).toBeFocused();
	await expect(page.getByRole('status').filter({ hasText: 'Loading runs…' })).toHaveCount(0);
	expect(browserState.pageErrors).toEqual([]);
});

When('the background Runs refresh arrives with updated data', async ({ browserState }) => {
	updateFailedRunSummary('Refresh completed with updated fixture data.');
	browserState.releaseRunsRequest?.();
});

Then('the card updates without navigation loading feedback', async ({ page, browserState }) => {
	const card = page.getByRole('button', { name: 'Open implement fixture-failed' });
	await expect(card).toContainText('Refresh completed with updated fixture data.');
	await expect(page.getByRole('status').filter({ hasText: 'Loading runs…' })).toHaveCount(0);
	expect(browserState.pageErrors).toEqual([]);
});

Then(
	'the failed Runs card is contained with a reachable action at desktop and phone widths',
	async ({ page, browserState }) => {
		const assertContained = async () => {
			const card = page.getByRole('button', { name: 'Open implement fixture-failed' });
			await expect(card).toBeVisible();
			await expect(card.locator('.open-label')).toBeVisible();
			await card.focus();
			await expect(card).toBeFocused();
			const metrics = await page.evaluate(() => {
				const summary = document.querySelector<HTMLElement>('.run-card .summary > span:last-child');
				const open = document.querySelector<HTMLElement>('.run-card .open-label');
				if (!summary || !open) throw new Error('Runs card summary or action is missing');
				const openBox = open.getBoundingClientRect();
				return {
					pageWidth: document.documentElement.clientWidth,
					pageScrollWidth: document.documentElement.scrollWidth,
					openLeft: openBox.left,
					openRight: openBox.right,
					overflowWrap: getComputedStyle(summary).overflowWrap,
					summaryHeight: summary.getBoundingClientRect().height,
					lineHeight: Number.parseFloat(getComputedStyle(summary).lineHeight)
				};
			});
			expect(metrics.pageScrollWidth).toBeLessThanOrEqual(metrics.pageWidth);
			expect(metrics.openLeft).toBeGreaterThanOrEqual(0);
			expect(metrics.openRight).toBeLessThanOrEqual(metrics.pageWidth);
			expect(metrics.overflowWrap).toBe('anywhere');
			expect(metrics.summaryHeight).toBeGreaterThan(metrics.lineHeight);
			expect(metrics.summaryHeight).toBeLessThanOrEqual(metrics.lineHeight * 2 + 1);
		};

		await page.setViewportSize({ width: 1280, height: 800 });
		await assertContained();
		await page.setViewportSize({ width: 390, height: 844 });
		await assertContained();
		expect(browserState.pageErrors).toEqual([]);
	}
);

function managedFixtureOnly(): void {
	test.skip(Boolean(process.env.BASE_URL), 'requires the managed table-fixture server');
}

function projectBase(page: { url(): string }): string {
	const match = new URL(page.url()).pathname.match(/^\/projects\/[^/]+/);
	if (!match) throw new Error('not viewing a project');
	return match[0];
}
