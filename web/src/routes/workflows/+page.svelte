<script lang="ts">
 import {workflows} from '#lib/workflow-design/generated';
</script>
<svelte:head><title>Workflows — Gimbal</title></svelte:head>
<section>
 <h1>Workflows</h1><p>Understand a workflow, review its steps, and learn how to run it.</p>
 <ul>{#each workflows as workflow}<li><a href={`/workflows/${workflow.name}`}>{workflow.name}</a><p>{workflow.summary}</p></li>{/each}</ul>
</section>
<style>
 section{max-width:760px;margin:2rem auto;padding:0 24px}h1{font-size:28px;font-weight:650}ul{list-style:none;padding:0}li{padding:18px 0;border-bottom:1px solid #b1b4b6}a{font-weight:600;color:#165a91;font-size:18px}li p{margin:5px 0;font-size:14px;color:#333b41}
</style>
