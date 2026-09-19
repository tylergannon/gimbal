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
	await expect(page.locator('aside').getByRole('heading')).toBeVisible();
	await expect(page.locator('aside')).not.toContainText('Select a sheet, call, command, interview, or watcher');
	expect(browserState.pageErrors).toEqual([]);
});
