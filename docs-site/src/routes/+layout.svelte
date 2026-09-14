<script lang="ts">
  import "../app.css";
  import { asset } from "$app/paths";
  import Footer from "#lib/components/Footer.svelte";
  import Header from "#lib/components/Header.svelte";

  let { children } = $props();
</script>

<svelte:head>
  <link rel="icon" type="image/svg+xml" href={asset("favicon.svg")} />
</svelte:head>

<Header />
{@render children()}
<Footer />
