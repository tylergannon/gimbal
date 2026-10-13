<script lang="ts">
  import { page } from "$app/state";
  import { APP_NAME } from "#lib/app.js";
  import { Button } from "#lib/components/ui/button/index.js";
  import { beginLogin } from "../../auth.remote";
  let error = $state("");
  async function signIn() {
    error = "";
    try {
      const url = await beginLogin();
      window.location.assign(url);
    } catch (cause) {
      error = cause instanceof Error ? cause.message : "Could not start sign-in. Please try again.";
    }
  }
</script>

<svelte:head><title>Sign in · {APP_NAME}</title></svelte:head>
<main class="grid min-h-screen place-items-center px-6">
  <section class="w-full max-w-sm rounded-xl border bg-card p-8 text-center shadow-sm">
    <h1 class="text-2xl font-semibold tracking-tight">{APP_NAME}</h1>
    {#if page.data.authEnabled}
      <p class="mt-3 mb-6 text-sm text-muted-foreground">Sign in with your authorized GitHub account to continue.</p>
      <Button onclick={signIn} disabled={!!beginLogin.pending} class="w-full">
        {beginLogin.pending ? "Opening GitHub…" : "Sign in with GitHub"}
      </Button>
    {:else}
      <p class="mt-3 mb-6 text-sm text-muted-foreground">This instance does not require sign-in.</p>
      <a href="/" class="text-sm underline">Open {APP_NAME}</a>
    {/if}
    {#if error}<p role="alert" class="mt-4 text-sm text-destructive">{error}</p>{/if}
  </section>
</main>
