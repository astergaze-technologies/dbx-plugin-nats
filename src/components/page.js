import { state } from "../bridge.js";
import { button, h } from "../dom.js";
import { icon } from "../icons.js";

export function pageHeader(title, subtitle, actions = []) {
  return h("header", { class: "page-head" },
    h("div", { class: "page-titles" }, h("h2", null, title), subtitle && h("p", { class: "hint" }, subtitle)),
    h("div", { class: "page-actions" }, actions));
}

// writeButton is disabled with an explanation on read-only connections.
export function writeButton(label, onclick, { iconName, danger = false, primary = false } = {}) {
  const variant = danger ? "dbx-btn--danger-ghost" : primary ? "dbx-btn--primary" : "";
  return button(label, onclick, {
    variant, icon: iconName && icon(iconName, 14),
    disabled: state.readOnly, title: state.readOnly ? "Read-only connection" : undefined,
  });
}

export const iconButton = (iconName, label, onclick, { danger = false, write = false } = {}) =>
  h("button", {
    type: "button", class: `icon-btn ${danger ? "danger" : ""}`, onclick, "aria-label": label, title: label,
    disabled: write && state.readOnly,
  }, icon(iconName, 14));

export function segmented(options, value, onChange) {
  const el = h("div", { class: "segmented", role: "tablist" });
  const draw = (current) => el.replaceChildren(...options.map(([id, label]) =>
    h("button", { type: "button", role: "tab", "aria-selected": String(id === current), onclick: () => { draw(id); onChange(id); } }, label)));
  draw(value);
  return el;
}
