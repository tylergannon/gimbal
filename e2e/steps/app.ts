import { expect } from '@playwright/test';
import type { WorkflowMapElement } from '../../web/src/lib/run/graphReadiness.js';
import { createBdd } from 'playwright-bdd';
import {
	hydrated,
	setRunsFixture,
	test,
	updateFailedRunSummary
} from './fixtures.js';

const { Given, When, Then } = createBdd(test);

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

Then('the project offers an empty Runs view', async ({ page }) => {
  await expect(page.getByText('No runs yet', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Run a workflow', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: /^Open implement / })).toHaveCount(0);
});

When('a recorded run becomes available', () => {
  setRunsFixture(true);
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
  expect(browserState.documents).toBe(1);
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

function managedFixtureOnly(): void {
	test.skip(Boolean(process.env.BASE_URL), 'requires the managed table-fixture server');
}
