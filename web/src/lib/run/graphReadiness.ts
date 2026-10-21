export type GraphReady = { revision: number; nodes: number; edges: number };
export type WorkflowMapElement = HTMLDivElement & { whenReady(): Promise<GraphReady> };

/** Readiness of this map's current layout, independent of its DOM selectors. */
export class GraphReadiness {
  private revision = 0;
  private ready?: GraphReady;
  private error?: Error;
  private disposed = false;
  private waiters = new Set<{
    resolve: (value: GraphReady) => void;
    reject: (error: Error) => void;
  }>();

  begin() {
    this.reject(new DOMException("Graph revision superseded", "AbortError"));
    this.ready = undefined;
    this.error = undefined;
    return ++this.revision;
  }

  applied(revision: number, nodes: number, edges: number) {
    if (this.disposed || revision !== this.revision) return;
    this.ready = { revision, nodes, edges };
    for (const waiter of this.waiters) waiter.resolve(this.ready);
    this.waiters.clear();
  }

  failed(revision: number, reason: unknown) {
    if (this.disposed || revision !== this.revision) return;
    this.ready = undefined;
    this.error = reason instanceof Error ? reason : new Error(String(reason));
    this.reject(this.error);
  }

  whenReady(): Promise<GraphReady> {
    if (this.disposed) return Promise.reject(new Error("Graph unmounted"));
    if (this.error) return Promise.reject(this.error);
    if (this.ready) return Promise.resolve(this.ready);
    return new Promise((resolve, reject) => this.waiters.add({ resolve, reject }));
  }

  dispose() {
    this.disposed = true;
    this.ready = undefined;
    this.reject(new Error("Graph unmounted"));
  }

  private reject(error: Error) {
    for (const waiter of this.waiters) waiter.reject(error);
    this.waiters.clear();
  }
}
