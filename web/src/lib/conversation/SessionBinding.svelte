<script lang="ts">
 import type { Conversation } from '#lib/skgo/conversation/types';
 import { conversationSetup, configureConversationModel, projectConversations } from '../../routes/conversation.remote';
 import { harnessName, modelName } from './names';
 import HarnessPicker from './HarnessPicker.svelte';
 import { errorMessage } from '#lib/errors';
 import { navigateOnUnauthorized } from '#lib/auth/unauthorized';
 let { project, record, onchanged }: { project:string; record:Conversation; onchanged?:()=>void } = $props();
 const local = $derived(!record.placement || record.placement.machine.id === 'local');
 const editable = $derived(record.provider === 'codex' && local && record.live && !record.active_request && ['idle','canceled'].includes(record.status) && !record.cleanup_pending && record.contact !== 'unreachable' && !record.requests.some(request => request.status === 'accepted'));
 const setup = $derived(editable ? conversationSetup({project,environment:record.environment || 'local'}) : undefined);
 let pending = $state(false); let failure = $state('');
 async function pick(choice:{model:string;effort?:string}) {
   if (pending) return;
   pending = true; failure = '';
   try { await configureConversationModel({project,conversation:record.id,model:choice.model,effort:choice.effort || ''}); await projectConversations(project).refresh(); onchanged?.(); }
   catch(error) { if (!navigateOnUnauthorized(error)) failure = errorMessage(error); }
   finally { pending = false; }
 }
</script>
{#if editable && setup?.current}<fieldset disabled={pending}><HarnessPicker harnesses={setup.current.harnesses.filter(item => item.id === 'codex')} states={setup.current.harness_states} machine={setup.current.machine} choice={{provider:record.provider,model:record.model,effort:record.effort}} onpick={pick} /></fieldset>{:else}<details class="binding"><summary>{harnessName(record.provider)} · {modelName(record.model)}{record.effort ? ` · ${record.effort}` : ''}<span>⌄</span></summary><div class="binding-menu"><b>Session binding</b><code>{record.model}</code><span>{record.provider === 'codex' && local ? 'Model and effort can change when this session is idle.' : 'This session keeps its original harness and model.'}</span><a href={`/projects/${project}/conversations/new`}>New session with another binding</a></div></details>{/if}
{#if pending}<span class="feedback" role="status">Saving model…</span>{/if}{#if failure}<span class="feedback" role="alert">{failure}</span>{/if}
<style>
 fieldset { margin:0; padding:0; border:0; } fieldset :global(.chip),summary { display:flex; align-items:center; gap:8px; padding:6px 10px; border:1px solid var(--border); border-radius:6px; background:var(--background); color:var(--foreground); font:13px/18px var(--font-sans); cursor:pointer; list-style:none; } summary::-webkit-details-marker { display:none; } .binding { position:relative; } .binding-menu { position:absolute; top:38px; left:0; z-index:20; display:grid; gap:10px; width:290px; padding:14px; border:1px solid var(--border); border-radius:7px; background:var(--background); box-shadow:0 8px 24px #0002; font-size:12px; } code { font:11px var(--font-mono); } a { color:var(--foreground); } .feedback { font-size:12px; color:var(--muted-foreground); }
</style>
