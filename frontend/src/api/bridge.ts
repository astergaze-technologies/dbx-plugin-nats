import { reactive } from "vue";

interface DbxPlugin {
  ready: Promise<{ connectionId?: string } | undefined>;
  context?: { connectionId?: string };
  invoke<T>(method: string, params: object, options: { timeoutMs: number }): Promise<T>;
  onEvent?(fn: (event: { method: string; params?: Record<string, unknown> }) => void): void;
  onContext?(fn: (context: { connectionId?: string }) => void): void;
  openFilesystem?(providerId: string, options: { connectionId: string; uri: string }): void;
}

export const plugin = (window as unknown as { dbxPlugin?: DbxPlugin }).dbxPlugin;

export const session = reactive({ connectionId: "", readOnly: false });

export function invoke<T = unknown>(method: string, params: object = {}, timeoutMs = 20000): Promise<T> {
  return plugin!.invoke<T>(method, { connectionId: session.connectionId, ...params }, { timeoutMs });
}

type Listener = (params: any) => void;
const listeners = new Map<string, Set<Listener>>();

// on subscribes to backend events for the current connection and returns an unsubscribe.
export function on<T>(method: string, fn: (params: T) => void): () => void {
  if (!listeners.has(method)) listeners.set(method, new Set());
  listeners.get(method)!.add(fn);
  return () => listeners.get(method)!.delete(fn);
}

export function startEvents() {
  plugin?.onEvent?.((event) => {
    const params = event.params || {};
    if (params.connectionId && params.connectionId !== session.connectionId) return;
    for (const fn of listeners.get(event.method) || []) fn(params);
  });
}

export const canBrowseFiles = () => !!plugin?.openFilesystem;

export const openFiles = (...segments: string[]) =>
  plugin?.openFilesystem?.("com.astergaze.nats.files", {
    connectionId: session.connectionId,
    uri: `nats:///${segments.map(encodeURIComponent).join("/")}`,
  });
