<script lang="ts">
  import { asset } from "$app/paths";

  interface Props {
    src: Parameters<typeof asset>[0];
    alt: string;
    label?: string;
    eager?: boolean;
    // crop shows the map alone, cutting the detail pane off the right edge.
    crop?: boolean;
  }

  let { src, alt, label = "127.0.0.1:8080", eager = false, crop = false }: Props = $props();
</script>

<div class="frame">
  <div class="frame-bar"><i></i><i></i><i></i><span>{label}</span></div>
  <img class={crop ? "crop" : undefined} src={asset(src)} {alt} width="2880" height="1800" loading={eager ? "eager" : "lazy"} />
</div>

<style>
  .crop {
    width: 139%;
    max-width: none;
  }
</style>
