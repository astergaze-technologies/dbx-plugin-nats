import { invoke, openFiles, plugin, state } from "../bridge.js";
import { pageHeader } from "../components/page.js";
import { badge, button, empty, errorBox, h, mount } from "../dom.js";
import { bytes, num } from "../format.js";

const stat = (label, value, sub) =>
  h("div", { class: "stat" }, h("div", { class: "stat-label" }, label), h("div", { class: "stat-value" }, value), sub && h("div", { class: "hint" }, sub));

export function overviewTab(panel, onLoaded) {
  const load = async () => {
    mount(panel, empty("Loading server information…"));
    try {
      const o = await invoke("nats/overview");
      state.readOnly = o.readOnly;
      onLoaded?.(o);
      const s = o.server;
      const js = o.jetstream;
      const limit = (max) => (max > 0 ? `of ${bytes(max)}` : "no account limit");
      mount(panel,
        pageHeader(s.name, `NATS ${s.version}${s.cluster ? ` · cluster ${s.cluster}` : ""}`, [
          o.readOnly && badge("read-only"), s.tls && badge("TLS"), badge(js ? "JetStream" : "no JetStream"),
          plugin.openFilesystem && button("Browse as files", () => openFiles()),
          button("Refresh", load),
        ]),
        js
          ? h("div", { class: "stats" },
            stat("Streams", num(js.streams)), stat("Consumers", num(js.consumers)),
            stat("Storage", bytes(js.storage), limit(js.maxStore)), stat("Memory", bytes(js.memory), limit(js.maxMemory)))
          : h("p", { class: "hint" }, "JetStream is not enabled for this account: streams, key-value and object stores are unavailable."),
        h("section", { class: "card" }, h("dl", { class: "props" }, [
          ["Server ID", s.id], ["URL", s.url], ["Round trip", `${s.rttMs.toFixed(1)} ms`],
          ["Max payload", bytes(s.maxPayload)], ["Headers", s.headers ? "supported" : "not supported"],
        ].map(([k, v]) => [h("dt", null, k), h("dd", null, v)]))));
    } catch (err) {
      mount(panel, errorBox(err));
    }
  };
  load();
}
