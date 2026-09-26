import { invoke, plugin, startEvents, state } from "./bridge.js";
import { createNavigator } from "./components/navigator.js";
import { createTabs } from "./components/tabs.js";
import { empty, h, mount } from "./dom.js";
import { bucketTab, bucketsTab } from "./views/kv.js";
import { storesTab, storeTab } from "./views/objects.js";
import { publishTab } from "./views/publish.js";
import { overviewTab } from "./views/server.js";
import { servicesTab } from "./views/services.js";
import { streamsTab, streamTab } from "./views/streams.js";
import { subscribeTab } from "./views/subscribe.js";

const root = document.getElementById("app");
const nav = h("nav", { class: "navigator", "aria-label": "NATS navigator" });
const tabBar = h("div", { class: "tab-bar", role: "tablist" });
const tabBody = h("div", { class: "tab-body-host" });

const tabs = createTabs(tabBar, tabBody);
let navigator;

const app = {
  refresh: (section) => navigator.refresh(section),
  closeTab: (id) => tabs.close(id),
  openStream: (name, reopen) => open(`stream:${name}`, name, "streams", (p) => streamTab(p, app, name), reopen),
  openBucket: (bucket) => open(`kv:${bucket}`, bucket, "kv", (p) => bucketTab(p, app, bucket)),
  openStore: (store) => open(`objects:${store}`, store, "objects", (p) => storeTab(p, app, store)),
};

function open(id, title, iconName, render, reopen = false) {
  if (reopen) tabs.close(id);
  tabs.open(id, { title, iconName, render });
}

const list = (method, key, label) => async () =>
  (await invoke(method))[key].map(label).filter(Boolean).sort((a, b) => a.label.localeCompare(b.label));

function buildNavigator() {
  navigator = createNavigator(nav, [
    { id: "server", label: "Server", iconName: "server", open: () => tabs.select("server") },
    { id: "streams", label: "Streams", iconName: "streams", open: () => open("streams", "Streams", "streams", (p) => streamsTab(p, app)),
      load: list("nats/streams", "streams", (s) => s.system ? null : ({ id: s.name, label: s.name, meta: s.messages ? s.messages.toLocaleString() : "", open: () => app.openStream(s.name) })) },
    { id: "kv", label: "Key-Value", iconName: "kv", open: () => open("kv", "Key-Value", "kv", (p) => bucketsTab(p, app)),
      load: list("nats/kvBuckets", "buckets", (b) => ({ id: b.bucket, label: b.bucket, meta: b.values ? b.values.toLocaleString() : "", open: () => app.openBucket(b.bucket) })) },
    { id: "objects", label: "Object Store", iconName: "objects", open: () => open("objects", "Object Store", "objects", (p) => storesTab(p, app)),
      load: list("nats/objectStores", "stores", (s) => ({ id: s.store, label: s.store, open: () => app.openStore(s.store) })) },
    { id: "services", label: "Services", iconName: "services", open: () => open("services", "Services", "services", servicesTab) },
    { id: "publish", label: "Publish", iconName: "send", open: () => open("publish", "Publish", "send", publishTab) },
    { id: "subscribe", label: "Subscribe", iconName: "radio", open: () => open("subscribe", "Subscribe", "radio", subscribeTab) },
  ]);
}

function start(connectionId) {
  state.connectionId = connectionId;
  state.readOnly = false;
  tabs.closeAll();
  if (!connectionId) {
    mount(root, empty("Open a saved NATS connection from the sidebar to use this workbench."));
    return;
  }
  mount(root, h("div", { class: "layout" }, nav, h("main", { class: "workspace" }, tabBar, tabBody)));
  buildNavigator();
  tabs.open("server", { title: "Server", iconName: "server", closable: false, render: (p) => overviewTab(p) });
}

if (!plugin) {
  mount(root, empty("This page must be opened inside DBX."));
} else {
  startEvents();
  let current = null;
  const apply = (context) => {
    const next = context?.connectionId || "";
    if (next !== current) {
      current = next;
      start(next);
    }
  };
  plugin.ready.then((context) => {
    apply(context || plugin.context);
    plugin.onContext?.(apply);
  });
}
