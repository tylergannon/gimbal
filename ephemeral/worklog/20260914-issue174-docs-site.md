friction: using `path` as a zsh loop variable overwrote the shell's command-search array -> use task-specific names such as `route_path` in repository proof commands
decision: issue 174 keeps `docs-site/` as a standalone Vite Plus package; no root JavaScript workspace or monorepo configuration
decision: the Pages artifact has no SPA fallback; adapter-static strict mode and root prerendering enforce the fully static boundary
friction: adapter-static 4.0.0-next.4 emits its own deprecated config access warning against SvelteKit 3.0.0-next.27 -> keep the compatible edge adapter and do not hide the successful build warning
