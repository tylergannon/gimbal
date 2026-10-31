import { expect } from '@playwright/test';
import type { WorkflowMapElement } from '../../web/src/lib/run/graphReadiness.js';
import { createBdd } from 'playwright-bdd';
import {
	hydrated,
	publishFailedRunFixture,
	publishInventoryFixture,
	completeInventoryFixture,
	test,
	updateFailedRunSummary
} from './fixtures.js';

const { Given, When, Then } = createBdd(test);

Given('I open the controlled project home', async ({ page }) => {
  managedFixtureOnly();
  await page.goto('/');
  await hydrated(page);
  await page.locator('section.projects a').first().click();
  await hydrated(page);
});

Then('the project home offers an empty activity list', async ({ page }) => {
  const home = page.getByRole('region', { name: 'Project home' });
  await expect(home.getByText('No activity yet. Start a session or run a workflow.')).toBeVisible();
  await expect(home.getByText('Loading runs…', { exact: true })).toHaveCount(0);
});

Then('the project home discovers the recorded run', async ({ page, browserState }) => {
  const home = page.getByRole('region', { name: 'Project home' });
  const row = home.getByRole('row').filter({ has: page.getByRole('link', { name: 'implement · failed run', exact: true }) });
  await expect(row).toContainText('Failed');
  await expect(home.getByText('Loading runs…', { exact: true })).toHaveCount(0);
  await expect(page.getByRole('complementary', { name: 'Project sidebar' }).getByRole('button', { name: /All runs 3/ })).toBeVisible();
  expect(browserState.pageErrors).toEqual([]);
});

When('I reload the project home', async ({ page, browserState }) => {
  await page.reload();
  await hydrated(page);
  browserState.expectedDocuments++;
});

When('I open All runs', async ({ page }) => {
  await page.getByRole('complementary', { name: 'Project sidebar' }).getByRole('button', { name: /All runs/ }).click();
});

Then('the project offers an empty Runs view', async ({ page }) => {
  await expect(page.getByText('No runs yet', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Run a workflow', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: /^Open implement / })).toHaveCount(0);
});

When('I return to project home', async ({ page }) => {
  await page.getByRole('link', { name: 'Project home', exact: true }).click();
});

When('I open and reload All runs', async ({ page, browserState }) => {
  await page.getByRole('complementary', { name: 'Project sidebar' }).getByRole('button', { name: /All runs/ }).click();
  await expect(page).toHaveURL(/\/runs$/);
  await page.reload();
  await hydrated(page);
  browserState.expectedDocuments++;
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

When('pending and completed run records become available', () => {
  publishInventoryFixture();
});

Then('the live inventory shows the pending question and completed run', async ({ page, browserState }) => {
  const view = page.getByRole('region', { name: 'Project runs' });
  const pending = view.getByRole('button', { name: 'Open plan-trip foreign-pending', exact: true });
  const completed = view.getByRole('button', { name: 'Open release completed-release', exact: true });
  await expect(pending.getByTitle('Needs you', { exact: true })).toBeVisible();
  await expect(pending).toContainText('1 question waiting');
  await expect(completed.getByTitle('Completed', { exact: true })).toBeVisible();
  await expect(completed.getByTitle('Elapsed', { exact: true })).toHaveText('7m 30s');
  await view.getByRole('button', { name: 'Needs you', exact: true }).click();
  await expect(pending).toBeVisible();
  await expect(completed).toHaveCount(0);
  await view.getByRole('button', { name: 'All', exact: true }).click();
  await expect(completed).toBeVisible();
  await pending.focus();
  browserState.runElement = await pending.elementHandle() ?? undefined;
  expect(browserState.documents).toBe(browserState.expectedDocuments);
  expect(browserState.pageErrors).toEqual([]);
});

When('the foreign pending run completes', () => {
  completeInventoryFixture();
});

Then('its live inventory row completes without stale running state', async ({ page, browserState }) => {
  const view = page.getByRole('region', { name: 'Project runs' });
  const completed = view.getByRole('button', { name: 'Open plan-trip foreign-pending', exact: true });
  await expect(completed.getByTitle('Completed', { exact: true })).toBeVisible();
  await expect(completed.getByTitle('Elapsed', { exact: true })).toHaveText('1m 00s');
  await expect(completed).not.toContainText('question waiting');
  await expect(view.getByTitle('Needs you', { exact: true })).toHaveCount(0);
  await expect(view.getByTitle('Running', { exact: true })).toHaveCount(0);
  await expect(view.getByRole('button', { name: /^Actions for/ })).toHaveCount(0);
  await expect(completed).toBeFocused();
  expect(await completed.evaluate((element, previous) => element === previous, browserState.runElement!)).toBe(true);
  await view.getByRole('button', { name: 'Live', exact: true }).click();
  await expect(view.getByRole('status')).toHaveText('No runs match this filter or search.');
  await view.getByRole('button', { name: 'Ended', exact: true }).click();
  await expect(completed).toBeVisible();
  await expect(view.getByRole('button', { name: 'Open release completed-release', exact: true })).toBeVisible();
  expect(browserState.documents).toBe(browserState.expectedDocuments);
  expect(browserState.pageErrors).toEqual([]);
});

When('a recorded run becomes available', () => {
  publishFailedRunFixture();
});

Then('the long failed summary keeps its run reachable at desktop and phone widths', async ({ page }) => {
  const card = page.getByRole('button', { name: 'Open implement fixture-failed', exact: true });
  for (const viewport of [{ width: 1280, height: 800 }, { width: 390, height: 844 }]) {
    await page.setViewportSize(viewport);
    await expect(card).toBeVisible();
    await card.focus();
    await expect(card).toBeFocused();
    const bounds = await card.evaluate(element => {
      const summary = element.querySelector<HTMLElement>('.summary');
      if (!summary) throw new Error('Run summary is missing');
      const row = element.getBoundingClientRect();
      const text = summary.getBoundingClientRect();
      return { width: document.documentElement.clientWidth, scrollWidth: document.documentElement.scrollWidth,
        left: row.left, right: row.right, textRight: text.right,
        visibleTextWidth: summary.clientWidth, fullTextWidth: summary.scrollWidth };
    });
    expect(bounds.scrollWidth).toBeLessThanOrEqual(bounds.width);
    expect(bounds.left).toBeGreaterThanOrEqual(0);
    expect(bounds.right).toBeLessThanOrEqual(bounds.width);
    expect(bounds.textRight).toBeLessThanOrEqual(bounds.right);
    expect(bounds.fullTextWidth).toBeGreaterThan(bounds.visibleTextWidth);
  }
});

When('the recorded run summary changes', async ({ page, browserState }) => {
  const card = page.getByRole('button', { name: 'Open implement fixture-failed', exact: true });
  await card.focus();
  browserState.runElement = await card.elementHandle() ?? undefined;
  updateFailedRunSummary('Refresh completed with updated fixture data.');
});

Then('the live Runs list updates without replacing the focused row', async ({ page, browserState }) => {
  const card = page.getByRole('button', { name: 'Open implement fixture-failed', exact: true });
  await expect(card).toContainText('Refresh completed with updated fixture data.');
  expect(await card.evaluate((element, previous) => element === previous, browserState.runElement!)).toBe(true);
  await expect(card).toBeFocused();
  await expect(page.getByText('Loading runs…', { exact: true })).toHaveCount(0);
  expect(browserState.documents).toBe(browserState.expectedDocuments);
  expect(browserState.pageErrors).toEqual([]);
});

When('I open the recorded fixture run', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await page.getByRole('button', { name: 'Open implement fixture-failed', exact: true }).click();
  await expect(page).toHaveURL(/\/runs\/fixture-failed$/);
});

Then('the recorded transcript is visible', async ({ page, browserState }) => {
  await expect(page.locator('.run-transcript')).toBeVisible();
  await expect(page.locator('.run-transcript .ended')).toHaveText('This run has ended.');
  expect(browserState.documents).toBe(browserState.expectedDocuments);
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
  expect(browserState.documents).toBe(browserState.expectedDocuments);
  expect(page.context().pages()).toHaveLength(1);
  expect(browserState.pageErrors).toEqual([]);
});

function managedFixtureOnly(): void {
	test.skip(Boolean(process.env.BASE_URL), 'requires the managed table-fixture server');
}
