import { expect, test } from 'vite-plus/test';
import { groupTabs } from './tabs';
import type { WorkspaceTab } from './types';
const parent: WorkspaceTab = { id: 'session', title: 'Build checkout', kind: 'chat', status: 'ended' };
const run = (id: string, status: WorkspaceTab['status']): WorkspaceTab => ({ id, title: id, kind: 'run', status, workflow: 'review', origin: { id: parent.id, title: parent.title } });
test('origin grouping preserves real panels and independently aggregates urgency and unseen', () => {
 const source = [parent, run('one','running'), { ...run('two','failed'), unseen: true }, run('three','needs-you')];
 const grouped = groupTabs(source, 'origin');
 expect(grouped).toHaveLength(2);
 expect(grouped[1]).toMatchObject({ title: 'runs · Build checkout', count: 3, status: 'needs-you', unseen: true });
 expect(grouped[1].members?.map(item => item.id)).toEqual(['one','two','three']);
 expect(source).toHaveLength(4);
 expect(groupTabs(source, 'none')).toBe(source);
 expect(groupTabs(source, 'origin', [grouped[1].id])).toEqual(source);
 expect(groupTabs(source.slice(0,3), 'origin')).toHaveLength(3);
});
test('workflow grouping includes manual runs while origin never invents an owner', () => {
 const source = [run('one','ended'), run('two','failed'), { ...run('manual','running'), origin: undefined }];
 expect(groupTabs(source,'origin')).toHaveLength(3);
 expect(groupTabs(source,'workflow')[0]).toMatchObject({ title: 'runs · review', status: 'failed', count: 3 });
});
