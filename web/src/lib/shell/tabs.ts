import type { WorkspaceTab } from './types';

export type TabGrouping = 'origin' | 'workflow' | 'none';
export const tabUrgency = (tab: WorkspaceTab) => ({ 'needs-you': 0, failed: 1, running: 2, queued: 3, cancelled: 4, ended: 5 }[tab.status ?? 'ended']);

/** Group display entries only. Original tabs and their retained panels stay alive. */
export function groupTabs(tabs: WorkspaceTab[], grouping: TabGrouping, individual: string[] = []): WorkspaceTab[] {
  if (grouping === 'none') return tabs;
  const key = (tab: WorkspaceTab) => tab.kind !== 'run' ? undefined : grouping === 'origin' ? tab.origin?.id : tab.workflow;
  const buckets = new Map<string, WorkspaceTab[]>();
  for (const tab of tabs) { const id = key(tab); if (id) buckets.set(id, [...(buckets.get(id) ?? []), tab]); }
  const emitted = new Set<string>();
  return tabs.flatMap(tab => {
    const id = key(tab);
    const members = id ? buckets.get(id)! : [];
    const groupID = `group:${grouping}:${id}`;
    if (!id || members.length < 3 || individual.includes(groupID)) return [tab];
    if (emitted.has(id)) return [];
    emitted.add(id);
    const urgent = [...members].sort((a,b) => tabUrgency(a)-tabUrgency(b))[0];
    return [{ id: groupID, title: `runs · ${grouping === 'origin' ? tab.origin!.title : tab.workflow}`, kind: 'runs', status: urgent.status, unseen: members.some(member => member.unseen), count: members.length, origin: grouping === 'origin' ? tab.origin : undefined, members } satisfies WorkspaceTab];
  });
}
