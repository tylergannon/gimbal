import { expect } from '@playwright/test';
import { createBdd } from 'playwright-bdd';
import { hydrated, test } from './fixtures.js';

const { Given, When, Then } = createBdd(test);

Given('I open the Gimble guide', async ({ page, browserState }) => {
	await page.goto('/');
	await hydrated(page);
	expect(browserState.documents).toBe(1);
});

Then('the guide says Gimble runs agent workflows in Go', async ({ page, browserState }) => {
	await expect(page.getByTestId('title')).toHaveText('Agent workflows in Go');
	await expect(page.getByText('Gimble is a Go library for running agent work from ordinary Go code.')).toBeVisible();
	expect(browserState.documents).toBe(1);
	expect(browserState.pageErrors).toEqual([]);
});

Then('the guide says what Gimble does not do', async ({ page }) => {
	await expect(page.getByRole('heading', { name: 'What it does not do' })).toBeVisible();
	await expect(page.getByText('Gimble does not choose your process for you.')).toBeVisible();
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

Given('I open the selected Gimble run', async ({ page }) => {
	const run = process.env.SKGO_E2E_RUN;
	if (!run) throw new Error('SKGO_E2E_RUN must name the run used by the workspace scenario');
	await page.goto(`/runs/${encodeURIComponent(run)}`);
	await hydrated(page);
});

Then('the run workspace shows its identity and observation', async ({ page, browserState }) => {
	await expect(page.getByRole('navigation', { name: 'Breadcrumb' }).getByRole('link', { name: 'Runs' })).toBeVisible();
	const map = page.locator('[aria-label$="workflow map"]');
	const history = page.getByRole('region', { name: 'Recorded run history' });
	expect((await map.count()) + (await history.count())).toBe(1);
	expect(browserState.pageErrors).toEqual([]);
});

When('I select recorded work in the workspace', async ({ page }) => {
	const mapSelection = page.locator('[aria-label$="workflow map"] button[aria-label^="Select "]').first();
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
