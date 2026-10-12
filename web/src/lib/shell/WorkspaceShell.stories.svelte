<script module lang="ts">
  import { defineMeta } from '@storybook/addon-svelte-csf';
  import WorkspaceDemo from './WorkspaceDemo.svelte';
  const { Story } = defineMeta({ title: 'Gimbal/Shell/Project workspace', component: WorkspaceDemo, parameters: { layout: 'fullscreen' } });
</script>

<Story name="Chat" args={{}} />
<Story name="File" args={{ file: true }} />
<Story name="Sidebar" args={{ sidebar: true }} />
<Story name="Overflow" args={{ many: true }} />
<Story name="Paused" args={{ recordingState: 'paused' }} />
<Story name="Saving" args={{ recordingState: 'saving' }} />
<Story name="Recording error" args={{ recordingState: 'error' }} />

<Story name="Dark sidebar" args={{ sidebar: true }} parameters={{ themes: { themeOverride: 'dark' } }} />
