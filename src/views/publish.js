import { invoke } from "../bridge.js";
import { pageHeader } from "../components/page.js";
import { messageCard } from "../components/payload.js";
import { busy, button, errorBox, h, mount, onEnter } from "../dom.js";
import { parseHeaders } from "../format.js";

const field = (label, control, hint) =>
  h("label", { class: "field" }, h("span", null, label), control, hint && h("span", { class: "hint" }, hint));

export function publishTab(panel) {
  const subject = h("input", { class: "dbx-input", placeholder: "orders.created", spellcheck: "false" });
  const timeout = h("input", { class: "dbx-input", type: "number", min: "1", max: "60", value: "5" });
  const headers = h("textarea", { class: "dbx-textarea mono", rows: "2", placeholder: "X-Trace-Id: 123", spellcheck: "false" });
  const data = h("textarea", { class: "dbx-textarea mono", rows: "8", placeholder: '{"id": 1}', spellcheck: "false" });
  const result = h("div", { "aria-live": "polite" });

  const params = () => ({ subject: subject.value.trim(), data: data.value, headers: parseHeaders(headers.value) });
  const fail = (err) => mount(result, errorBox(err));

  const publish = () => busy(pubBtn, async () => {
    const p = params();
    await invoke("nats/publish", p);
    mount(result, h("p", { class: "ok" }, `Published to ${p.subject} at ${new Date().toLocaleTimeString()}.`));
  }, fail);
  const request = () => busy(reqBtn, async () => {
    const secs = Math.min(60, Math.max(1, Number(timeout.value) || 5));
    const reply = await invoke("nats/request", { ...params(), timeoutMs: secs * 1000 }, secs * 1000 + 5000);
    mount(result, h("h3", { class: "section-title" }, `Reply in ${reply.elapsedMs.toFixed(1)} ms`), messageCard(reply));
  }, fail);

  const pubBtn = button("Publish", publish, { variant: "dbx-btn--primary" });
  const reqBtn = button("Send request", request);
  onEnter(subject, publish);
  mount(panel,
    pageHeader("Publish", "Send a message, or a request that waits for one reply."),
    h("div", { class: "form" },
      h("div", { class: "row" }, field("Subject", subject, "Wildcards are not allowed when publishing."), field("Request timeout (s)", timeout)),
      field("Headers", headers, "One Key: value per line (optional)."),
      field("Payload", data),
      h("div", { class: "toolbar" }, pubBtn, reqBtn),
      result));
}
