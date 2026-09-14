<script lang="ts">
  import { type VariantProps, tv } from "tailwind-variants";
  import { cn, type WithElementRef } from "#lib/utils.js";
  import type {
    HTMLAnchorAttributes,
    HTMLButtonAttributes,
  } from "svelte/elements";

  const buttonVariants = tv({
    base: "rounded-md border border-transparent bg-clip-padding text-sm font-medium focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 active:not-aria-[haspopup]:translate-y-px aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 dark:aria-invalid:border-destructive/50 dark:aria-invalid:ring-destructive/40 [&_svg:not([class*='size-'])]:size-4 group/button inline-flex shrink-0 items-center justify-center whitespace-nowrap transition-all outline-none select-none disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0",
    variants: {
      variant: {
        default: "bg-primary text-primary-foreground hover:bg-primary/80",
        outline:
          "border-border bg-background shadow-xs hover:bg-muted hover:text-foreground aria-expanded:bg-muted aria-expanded:text-foreground dark:border-input dark:bg-input/30 dark:hover:bg-input/50",
      },
      size: {
        default: "h-9 gap-1.5 px-2.5",
        sm: "h-8 gap-1 rounded-[min(var(--radius-md),10px)] px-2.5",
      },
    },
    defaultVariants: { variant: "default", size: "default" },
  });

  type ButtonVariant = VariantProps<typeof buttonVariants>["variant"];
  type ButtonSize = VariantProps<typeof buttonVariants>["size"];
  type ButtonProps = WithElementRef<HTMLButtonAttributes> &
    WithElementRef<HTMLAnchorAttributes> & {
      variant?: ButtonVariant;
      size?: ButtonSize;
    };
  let {
    class: className,
    variant = "default",
    size = "default",
    ref = $bindable(null),
    href = undefined,
    type = "button",
    disabled,
    children,
    ...restProps
  }: ButtonProps = $props();
</script>

{#if href}
  <a
    bind:this={ref}
    data-slot="button"
    class={cn(buttonVariants({ variant, size }), className)}
    href={disabled ? undefined : href}
    aria-disabled={disabled}
    role={disabled ? "link" : undefined}
    tabindex={disabled ? -1 : undefined}
    {...restProps}
  >
    {@render children?.()}
  </a>
{:else}
  <button
    bind:this={ref}
    data-slot="button"
    class={cn(buttonVariants({ variant, size }), className)}
    {type}
    {disabled}
    {...restProps}
  >
    {@render children?.()}
  </button>
{/if}
