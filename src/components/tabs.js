import { h, mount } from "../dom.js";
import { icon } from "../icons.js";

// Closable tabs; panels stay mounted while hidden so live views keep their state.
export function createTabs(bar, body) {
  const tabs = new Map();
  let active = null;

  function select(id) {
    active = id;
    for (const [key, tab] of tabs) {
      const on = key === id;
      tab.button.setAttribute("aria-selected", String(on));
      tab.button.tabIndex = on ? 0 : -1;
      tab.panel.hidden = !on;
    }
  }

  function close(id) {
    const tab = tabs.get(id);
    if (!tab || !tab.closable) return;
    tab.dispose?.();
    tab.el.remove();
    tab.panel.remove();
    tabs.delete(id);
    if (active === id) select([...tabs.keys()].at(-1));
  }

  function open(id, { title, iconName, render, closable = true }) {
    if (tabs.has(id)) return select(id);
    const panel = h("section", { class: "tab-panel", role: "tabpanel", "aria-label": title });
    const button = h("button", { type: "button", role: "tab", class: "tab", onclick: () => select(id) },
      iconName && icon(iconName, 14), h("span", { class: "tab-title" }, title));
    const el = h("div", { class: "tab-wrap" }, button,
      closable && h("button", { type: "button", class: "tab-close", "aria-label": `Close ${title}`, onclick: () => close(id) }, icon("close", 12)));
    const tab = { el, button, panel, closable };
    tabs.set(id, tab);
    bar.append(el);
    body.append(panel);
    tab.dispose = render(panel, { close: () => close(id) });
    select(id);
  }

  function closeAll() {
    for (const id of [...tabs.keys()]) {
      tabs.get(id).closable = true;
      close(id);
    }
    mount(bar);
    mount(body);
  }

  return { open, close, closeAll, select };
}
