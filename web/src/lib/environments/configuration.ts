// Display/edit fields parsed from the shared file. Opaque route/plugin blocks
// remain in the original document and are preserved by the Go configuration store.
export type MachineConfiguration = {
  id: string;
  name: string;
  kind: string;
  ssh?: { host: string; port?: number; daemon_port?: number; workspace_root?: string };
  address?: string;
  executable?: string;
};
export function machineConfigurations(text: string): MachineConfiguration[] {
  try {
    const parsed = JSON.parse(text) as { machines?: unknown };
    if (!Array.isArray(parsed?.machines)) return [];
    return parsed.machines.filter(
      (item): item is MachineConfiguration =>
        !!item &&
        typeof item === "object" &&
        typeof item.id === "string" &&
        typeof item.name === "string" &&
        typeof item.kind === "string",
    );
  } catch {
    return [];
  }
}
