const plugin = window.dbxPlugin;
const listeners = new Map();

export const state = { connectionId: "", readOnly: false };

export function invoke(method, params = {}, timeoutMs = 20000) {
  return plugin.invoke(method, { connectionId: state.connectionId, ...params }, { timeoutMs });
}

// on(method, fn) subscribes to backend events for the current connection; returns an unsubscribe.
export function on(method, fn) {
  if (!listeners.has(method)) listeners.set(method, new Set());
  listeners.get(method).add(fn);
  return () => listeners.get(method).delete(fn);
}

export function startEvents() {
  plugin.onEvent?.((event) => {
    const params = event.params || {};
    if (params.connectionId && params.connectionId !== state.connectionId) return;
    for (const fn of listeners.get(event.method) || []) fn(params);
  });
}

export const openFiles = (path = "") =>
  plugin.openFilesystem?.("com.astergaze.nats.files", { connectionId: state.connectionId, uri: `nats:///${path}` });

export { plugin };
