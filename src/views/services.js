import { invoke } from "../bridge.js";
import { pageHeader } from "../components/page.js";
import { dataTable } from "../components/table.js";
import { badge, button, empty, errorBox, h, mount } from "../dom.js";
import { num, time } from "../format.js";

export function servicesTab(panel) {
  const body = h("div");
  const load = async () => {
    mount(body, empty("Discovering services (1s)…"));
    try {
      const { services } = await invoke("nats/services");
      if (!services.length) return mount(body, empty("No NATS micro services answered $SRV.INFO."));
      mount(body, services.map((s) => h("section", { class: "card service" },
        h("header", { class: "toolbar" }, h("strong", null, s.name), badge(`v${s.version}`), h("code", { class: "hint" }, s.id),
          h("span", { class: "hint grow" }, s.description || ""), h("span", { class: "hint" }, `since ${time(s.started)}`)),
        dataTable({
          label: `${s.name} endpoints`,
          filterable: false,
          columns: [
            { key: "name", label: "Endpoint" },
            { key: "subject", label: "Subject", render: (e) => h("code", null, e.subject) },
            { key: "queueGroup", label: "Queue" },
            { key: "requests", label: "Requests", num: true, render: (e) => num(e.requests) },
            { key: "errors", label: "Errors", num: true, render: (e) => num(e.errors) },
            { key: "avgMs", label: "Avg ms", num: true, render: (e) => e.avgMs.toFixed(2) },
            { key: "lastError", label: "Last error", wrap: true },
          ],
          rows: s.endpoints,
        }).el)));
    } catch (err) {
      mount(body, errorBox(err));
    }
  };
  mount(panel, pageHeader("Services", "NATS micro services discovered via $SRV.INFO and $SRV.STATS.", [button("Refresh", load)]), body);
  load();
}
