import { expect } from '@playwright/test';
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
	expect(browserState.documents).toBe(1);
});

Given('I open the controlled project runs', async ({ page, browserState }) => {
	managedFixtureOnly();
	await page.goto('/');
	await hydrated(page);
	expect(browserState.documents).toBe(1);
});

Then('recorded runs show identity, status and time', async ({ page, browserState }) => {
	await expect(page.getByRole('heading', { name: 'Runs', exact: true })).toBeVisible();
	const cards = page.getByRole('region', { name: 'Recorded runs' }).getByRole('button', { name: /^Open / });
	expect(await cards.count()).toBeGreaterThan(0);
	const card = cards.first();
	await expect(card.locator('.identity strong')).not.toHaveText('');
	await expect(card.locator('.identity code')).not.toHaveText('');
	await expect(card.locator('.status')).toContainText(/Running|Ended|Failed|Cancelled/);
	await expect(card.locator('time')).toContainText(/^(\d+ s|\d+ (min|h) ago|[A-Z][a-z]{2} \d+)$/);
	await expect(card.locator('.stats')).toContainText('Total cost');
	await expect(card.locator('.stats')).toContainText('Elapsed');
	await expect(card.locator('.instruction')).toContainText('Latest instruction');
	expect(browserState.pageErrors).toEqual([]);
});

When('I filter runs by the first recorded status', async ({ page }) => {
	const recorded = page.getByRole('region', { name: 'Recorded runs' });
	const firstCard = recorded.getByRole('button', { name: /^Open / }).first();
	const status = (await firstCard.locator('.status').innerText()).trim();
	const filter = status === 'Running' ? 'Active' : status;
	await recorded.getByRole('button', { name: new RegExp(`^${filter} ·`) }).click();
});

Then('only runs with that status remain', async ({ page }) => {
	const recorded = page.getByRole('region', { name: 'Recorded runs' });
	const selected = recorded.getByRole('button', { pressed: true });
	const filter = ((await selected.innerText()).split('·')[0] ?? '').trim();
	const status = filter === 'Active' ? 'Running' : filter;
	const cards = recorded.getByRole('button', { name: /^Open / });
	expect(await cards.count()).toBeGreaterThan(0);
	for (const card of await cards.all()) {
		await expect(card.locator('.status')).toContainText(status);
	}
});

When('I open the first filtered run', async ({ page }) => {
	const open = page
		.getByRole('region', { name: 'Recorded runs' })
		.locator('button[aria-label^="Open "]')
		.first();
	await open.click();
	await expect(page).toHaveURL(/\/runs\/[^/]+$/);
});

When('I follow the About link', async ({ page }) => {
	await page.getByRole('link', { name: 'About' }).click();
});

Then('About is visible without a document reload', async ({ page, browserState }) => {
	await expect(page).toHaveURL(/\/about$/);
	await expect(page.getByTestId('title')).toHaveText('About Gimble');
	expect(browserState.documents).toBe(1);
	expect(browserState.pageErrors).toEqual([]);
});

When('I load the About route directly', async ({ page }) => {
	await page.goto('/about');
});

Then('About is visible in a new document', async ({ page, browserState }) => {
	await expect(page.getByTestId('title')).toHaveText('About Gimble');
	expect(browserState.documents).toBe(2);
	expect(browserState.pageErrors).toEqual([]);
});

Then('the run workspace shows its identity and observation', async ({ page, browserState }) => {
	await expect(page.getByRole('navigation', { name: 'Breadcrumb' }).getByRole('link', { name: 'Runs' })).toBeVisible();
	const map = page.locator('[aria-label$="workflow map"]');
	const history = page.getByRole('region', { name: 'Recorded run history' });
	expect((await map.count()) + (await history.count())).toBe(1);
	expect(browserState.pageErrors).toEqual([]);
});

When('I select recorded work in the workspace', async ({ page }) => {
	const mapSelection = page
		.locator(
			'[aria-label$="workflow map"] button[aria-label^="Select "]:not([aria-label$=" instance"])'
		)
		.first();
	if (await mapSelection.count()) {
		await mapSelection.click();
		return;
	}
	await page.getByRole('region', { name: 'Recorded run history' }).getByRole('button').first().click();
});

Then('the detail pane describes that selected work', async ({ page, browserState }) => {
	const detail = page.locator('aside');
	await expect(detail.getByRole('heading')).toBeVisible();
	await expect(detail.locator('.empty-selection')).toHaveCount(0);
	await expect(detail.locator('.placement')).toBeVisible();
	expect(browserState.pageErrors).toEqual([]);
});

Given('I open About with recorded runs available', async ({ page, browserState }) => {
	managedFixtureOnly();
	await page.goto('/about');
	await hydrated(page);
	expect(browserState.documents).toBe(1);
});

Given('I open About with an empty project', async ({ page, browserState }) => {
	managedFixtureOnly();
	setRunsFixture(false);
	await page.goto('/about');
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
	await expect(page.getByRole('button', { name: 'Open long-failure fixture-failed' })).toBeVisible();
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
	const card = page.getByRole('button', { name: 'Open long-failure fixture-failed' });
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
	const card = page.getByRole('button', { name: 'Open long-failure fixture-failed' });
	await expect(card).toContainText('Refresh completed with updated fixture data.');
	await expect(page.getByRole('status').filter({ hasText: 'Loading runs…' })).toHaveCount(0);
	expect(browserState.pageErrors).toEqual([]);
});

Then(
	'the failed Runs card is contained with a reachable action at desktop and phone widths',
	async ({ page, browserState }) => {
		const assertContained = async () => {
			const card = page.getByRole('button', { name: 'Open long-failure fixture-failed' });
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
