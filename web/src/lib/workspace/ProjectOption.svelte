<script lang="ts">
  import * as Dropdown from '#lib/components/ui/dropdown-menu';
  import { watchRuns } from '#lib/runs.remote';
  import { projectConversations } from '../../routes/conversation.remote';
  import { conversationStatus } from './conversations';
  import type { ProjectChoice } from '#lib/skgo/host/types';
  let { choice, current, onchoose }: { choice:ProjectChoice; current:string; onchoose:(id:string)=>void } = $props();
  const runs = $derived(watchRuns(choice.id));
  const sessions = $derived(projectConversations(choice.id));
  const controllers = $derived(new Set(sessions.current?.map(item => item.workflow_run).filter(Boolean) ?? []));
  const attention = $derived(new Set(runs.current?.attention.filter(row => row.status === 'pending' && !controllers.has(row.run) && runs.current?.items.some(item => item.run.id === row.run && item.run.status === 'running')).map(row => row.run)).size + (sessions.current?.filter(row => conversationStatus(row) === 'needs-you').length ?? 0));
  const live = $derived((runs.current?.items.filter(item => item.run.status === 'running' && !controllers.has(item.run.id)).length ?? 0) + (sessions.current?.filter(item => conversationStatus(item) === 'running').length ?? 0));
</script>
<Dropdown.Item onclick={() => onchoose(choice.id)} class="project-option"><span class="check">{choice.id === current ? '✓' : ''}</span><span class="copy"><b>{choice.path.split('/').at(-1)}</b><span>{choice.path}</span></span>{#if attention || live}<span class="count">{attention ? `${attention} needs you` : `${live} live`}</span>{/if}</Dropdown.Item>
<style>
  .check { flex:none; width:14px; } .copy { display:flex; min-width:0; flex:1; flex-direction:column; gap:2px; } b { font:500 13px/18px var(--font-sans); } .copy>span { color:var(--muted-foreground); font:11px/16px var(--font-mono); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; } .count { flex:none; color:var(--muted-foreground); font-size:11px; }
</style>
