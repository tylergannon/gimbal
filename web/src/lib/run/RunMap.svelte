<script lang="ts">
  import { SvelteFlowProvider } from "@xyflow/svelte";
  import "@xyflow/svelte/dist/style.css";
  import type { RunSnapshot } from "../observation/index.js";
  import type { SourcePage, ViewNode } from "../workflow-design/types.js";
  import type { Graph } from "../workflow/types.js";
  import type { MapSelection } from "./types.js";
  import RunMapCanvas from "./RunMapCanvas.svelte";

  let props: {
    source: SourcePage;
    graph: Graph;
    snapshot: RunSnapshot;
    selected?: MapSelection;
    /** A step the run has no record of, shown in the designer's inspector. */
    inspected?: ViewNode;
    reveal?: { request: number; selection: MapSelection };
    onselect?: (selection: MapSelection) => void;
    oninspect?: (node: ViewNode) => void;
    onopen?: (selection: MapSelection) => void;
  } = $props();
</script>

<SvelteFlowProvider><RunMapCanvas {...props} /></SvelteFlowProvider>
