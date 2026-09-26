import { h, mount } from "../dom.js";
import { icon } from "../icons.js";

// Tree of sections, each with lazily loaded children, like the DBX sidebar.
// section: { id, label, iconName, open(), load?() -> [{ id, label, iconName?, meta?, open }] }
export function createNavigator(el, sections) {
  const filter = h("input", { class: "dbx-input nav-filter", type: "search", placeholder: "Filter…", "aria-label": "Filter navigator" });
  const tree = h("ul", { class: "nav-tree", role: "tree", "aria-label": "NATS objects" });
  const nodes = new Map();

  for (const section of sections) {
    const count = h("span", { class: "nav-count" });
    const children = h("ul", { role: "group", hidden: true });
    const toggle = section.load
      ? h("button", { type: "button", class: "nav-toggle", "aria-label": `Expand ${section.label}`, onclick: () => expand(section.id) }, icon("chevron", 14))
      : h("span", { class: "nav-toggle" });
    const row = h("div", { class: "nav-row" }, toggle,
      h("button", { type: "button", class: "nav-label", onclick: section.open }, icon(section.iconName), h("span", null, section.label), count));
    const item = h("li", { role: "treeitem", "aria-expanded": section.load ? "false" : null }, row, children);
    nodes.set(section.id, { section, item, toggle, children, count, items: [], loaded: false });
    tree.append(item);
  }

  async function load(id) {
    const node = nodes.get(id);
    mount(node.children, h("li", { class: "nav-hint" }, "Loading…"));
    try {
      node.items = await node.section.load();
      node.loaded = true;
      node.count.textContent = String(node.items.length);
      draw(node);
    } catch (err) {
      mount(node.children, h("li", { class: "nav-hint error-text" }, err.message));
    }
  }

  function draw(node) {
    const q = filter.value.toLowerCase();
    const items = node.items.filter((it) => it.label.toLowerCase().includes(q));
    if (!items.length) return mount(node.children, h("li", { class: "nav-hint" }, q ? "No matches" : "Empty"));
    mount(node.children, items.map((it) =>
      h("li", { role: "treeitem" }, h("button", { type: "button", class: "nav-leaf", title: it.label, onclick: it.open },
        icon(it.iconName || node.section.iconName, 14), h("span", { class: "nav-leaf-label" }, it.label), it.meta && h("span", { class: "nav-count" }, it.meta)))));
  }

  function expand(id, force) {
    const node = nodes.get(id);
    const open = force ?? node.children.hidden;
    node.children.hidden = !open;
    node.item.setAttribute("aria-expanded", String(open));
    node.toggle.classList.toggle("open", open);
    if (open && !node.loaded) load(id);
  }

  filter.addEventListener("input", () => {
    for (const node of nodes.values()) {
      if (node.loaded) draw(node);
      if (filter.value && node.section.load && node.children.hidden) expand(node.section.id, true);
    }
  });

  mount(el, filter, tree);

  return {
    refresh(id) {
      const node = nodes.get(id);
      if (node?.loaded || !node?.children.hidden) load(id);
    },
    reset() {
      for (const node of nodes.values()) {
        node.loaded = false;
        node.items = [];
        node.count.textContent = "";
        if (!node.children.hidden) load(node.section.id);
      }
    },
  };
}
