import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, fetch }) => {
  const response = await fetch(`/__workflow/${encodeURIComponent(params.workflow)}`);
  if (!response.ok) error(response.status, 'Workflow preview unavailable');
  return { name: params.workflow, snapshot: await response.json() };
};
