import type { Plugin } from "vite-plus";
export function workflowSource(options?: {
  workflows?: { name: string; dir: string; entry: string; descriptor?: boolean }[];
}): Plugin;
