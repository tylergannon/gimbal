import { expect } from '@playwright/test';
import { createBdd } from 'playwright-bdd';
import { hydrated, test } from './fixtures.js';

const { Given, When, Then } = createBdd(test);

Given('I open the project runs', async ({ page, browserState }) => {
	await page.goto('/');
	await hydrated(page);
	expect(browserState.documents).toBe(1);
});

Then('recorded runs show identity, status and time', async ({ page, browserState }) => {
	await expect(page.getByRole('heading', { name: 'Runs', exact: true })).toBeVisible();
	const rows = page.getByRole('region', { name: 'Recorded runs' }).getByRole('row').filter({
		has: page.locator('button[aria-label^="Open "]')
	});
	expect(await rows.count()).toBeGreaterThan(0);
	const cells = rows.first().getByRole('cell');
	await expect(cells.nth(0).getByRole('button')).not.toHaveText('');
	await expect(cells.nth(1)).toContainText(/Running|Ended|Failed|Cancelled/);
	await expect(cells.nth(3)).toContainText(/^(\d+ (s|min|h) ago|[A-Z][a-z]{2} \d+)$/);
	await expect(cells.nth(4)).not.toHaveText('');
	expect(browserState.pageErrors).toEqual([]);
});

When('I filter runs by the first recorded status', async ({ page }) => {
	const recorded = page.getByRole('region', { name: 'Recorded runs' });
	const firstRow = recorded.getByRole('row').filter({
		has: page.locator('button[aria-label^="Open "]')
	}).first();
	const status = (await firstRow.getByRole('cell').nth(1).innerText()).trim();
	const filter = status === 'Running' ? 'Active' : status;
	await recorded.getByRole('button', { name: new RegExp(`^${filter} ·`) }).click();
});

Then('only runs with that status remain', async ({ page }) => {
	const recorded = page.getByRole('region', { name: 'Recorded runs' });
	const selected = recorded.getByRole('button', { pressed: true });
	const filter = ((await selected.innerText()).split('·')[0] ?? '').trim();
	const status = filter === 'Active' ? 'Running' : filter;
	const rows = recorded.getByRole('row').filter({
		has: page.locator('button[aria-label^="Open "]')
	});
	expect(await rows.count()).toBeGreaterThan(0);
	for (const row of await rows.all()) {
		await expect(row.getByRole('cell').nth(1)).toContainText(status);
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
