<script lang="ts">
  import { goto } from "$app/navigation";
  import { logout } from "../../routes/auth.remote";
  import { navigateOnUnauthorized } from "./unauthorized";
  import { Button } from "#lib/components/ui/button/index.js";

  let { name }: { name: string } = $props();
  let error = $state("");
  async function signOut() {
    error = "";
    try {
      await logout();
      await goto("/auth/login");
    } catch (cause) {
      if (!navigateOnUnauthorized(cause))
        error = cause instanceof Error ? cause.message : "Could not sign out. Please try again.";
    }
  }
</script>

<div class="flex items-center justify-end gap-3 border-b px-5 py-2 text-sm">
  <span>{name}</span>
  <Button variant="ghost" size="sm" onclick={signOut} disabled={!!logout.pending}>
    {logout.pending ? "Signing out…" : "Sign out"}
  </Button>
  {#if error}<p role="alert" class="text-destructive">{error}</p>{/if}
</div>
