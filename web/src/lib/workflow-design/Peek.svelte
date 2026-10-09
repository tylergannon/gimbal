<script lang="ts">
  import type { ViewNode } from './types';
  let {node,rect}: {node:ViewNode;rect:DOMRect}=$props();
  let code=$derived(node.control?.code?.split('\n')[0] || node.detail?.expression?.split('\n')[0]);
  let left=$derived(Math.max(12,Math.min(rect.left,window.innerWidth-352)));
  let below=$derived(rect.bottom + 140 < window.innerHeight);
</script>
<div class="peek" role="tooltip" style:left={`${left}px`} style:top={below?`${rect.bottom+10}px`:undefined} style:bottom={!below?`${window.innerHeight-rect.top+10}px`:undefined}>
<strong>{node.title || node.label}</strong>
{#if node.description}<p>{node.description}</p>{/if}
{#if code}<code>{code}</code>{/if}
</div>
<style>.peek{position:fixed;z-index:1000;pointer-events:none;width:max-content;max-width:min(328px,calc(100vw - 48px));box-sizing:border-box;padding:10px 12px;background:white;border:1px solid #484949;border-radius:4px;box-shadow:0 3px 14px #0002;color:#0b0c0c;font-size:13px;line-height:1.45}.peek p{margin:5px 0;white-space:pre-line}.peek code{display:block;font-size:12px;overflow-wrap:anywhere;white-space:pre-wrap;margin-top:6px;max-height:100px;overflow:hidden}</style>
