import { h } from "../dom.js";
import { bytes, headerLine, payloadText, time } from "../format.js";

// A message-like record: { subject?, seq?, time?, size, headers?, data, encoding, truncated? }
export function messageCard(m, extra = []) {
  return h("article", { class: "msg" },
    h("header", { class: "msg-head" },
      m.seq != null && h("strong", null, `#${m.seq}`),
      m.subject && h("code", null, m.subject),
      extra,
      h("span", { class: "hint" }, time(m.time || m.receivedAt || m.created)),
      h("span", { class: "hint" }, bytes(m.size))),
    headerLine(m.headers) && h("div", { class: "hint mono" }, headerLine(m.headers)),
    h("pre", null, payloadText(m)));
}
