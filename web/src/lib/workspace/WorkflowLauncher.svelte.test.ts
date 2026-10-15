import { expect, test, vi } from 'vite-plus/test';
import { render } from 'vitest-browser-svelte';
import Launcher from './WorkflowLauncher.svelte';
const remote = vi.hoisted(() => ({ opened: vi.fn(), guide: vi.fn() }));
vi.mock('../../routes/workflow.remote', () => ({ workflowCatalog: () => ({ current: { project: '/work/project', workflows: [{ name: 'review', summary: 'Read-only review', help: '', fields: [], roles: [], source_json: '' }] } }) }));
vi.mock('../../routes/launch.remote', () => {
 const field = (name: string) => ({ as: (type: string, value?: unknown) => ({ name, type, ...(value === undefined ? {} : { value: String(value) }) }) });
 return { launchWorkflow: { for: () => ({ pending: 0, fields: { workflow: field('workflow'), project: field('project'), workdir: field('workdir'), allIssues: () => [] }, enhance: (handler: (submitted: unknown) => Promise<void>) => ({ onsubmit: async (event: SubmitEvent) => { event.preventDefault(); await handler({ submit: async () => true, result: { project_id: 'project', run_id: 'admitted' } }); } }) }) } };
});

test('opening the guide removes the popup immediately even without animation frames', async () => {
 const noFrames = vi.spyOn(window, 'requestAnimationFrame').mockImplementation(() => 0);
 try {
  const screen = await render(Launcher, { project: 'project', onopen: remote.opened, onopenguide: remote.guide });
  await screen.getByRole('button', { name: 'Run a workflow', exact: true }).click();
  await screen.getByRole('button', { name: 'Read the guide' }).click();
  expect(remote.guide).toHaveBeenCalledWith('review');
  expect(document.querySelector('.workflow-popup')).toBeNull();
 } finally { noFrames.mockRestore(); }
});

test('admitting a run removes the popup before opening its retained view', async () => {
 const noFrames = vi.spyOn(window, 'requestAnimationFrame').mockImplementation(() => 0);
 try {
  const screen = await render(Launcher, { project: 'project', onopen: remote.opened, onopenguide: remote.guide });
  await screen.getByRole('button', { name: 'Run a workflow', exact: true }).click();
  await screen.getByRole('button', { name: 'Start run' }).click();
  expect(remote.opened).toHaveBeenCalledWith('/projects/project/runs/admitted');
  expect(document.querySelector('.workflow-popup')).toBeNull();
 } finally { noFrames.mockRestore(); }
});
