import { h, mount } from "../dom.js";

// column: { key, label, num?, render?(row), value?(row) }
export function dataTable({ columns, rows = [], onOpen, actions, emptyText = "No rows", filterable = true, label }) {
  let data = rows;
  let sort = { key: columns[0].key, desc: false };
  const filter = h("input", { class: "dbx-input", type: "search", placeholder: "Filter…", "aria-label": `Filter ${label || "rows"}` });
  const count = h("span", { class: "hint" });
  const head = h("tr");
  const body = h("tbody");
  const valueOf = (col, row) => (col.value ? col.value(row) : row[col.key]);

  function drawHead() {
    mount(head, columns.map((col) => {
      const on = sort.key === col.key;
      return h("th", { scope: "col", class: col.num ? "num" : "", "aria-sort": on ? (sort.desc ? "descending" : "ascending") : "none" },
        h("button", { type: "button", class: "th-sort", onclick: () => { sort = { key: col.key, desc: on ? !sort.desc : !!col.num }; draw(); } },
          col.label, on ? (sort.desc ? " ↓" : " ↑") : ""));
    }), actions && h("th", { scope: "col" }, h("span", { class: "sr-only" }, "Actions")));
  }

  function draw() {
    drawHead();
    const q = filter.value.toLowerCase();
    const col = columns.find((c) => c.key === sort.key);
    const visible = data
      .filter((row) => !q || columns.some((c) => String(valueOf(c, row) ?? "").toLowerCase().includes(q)))
      .sort((a, b) => {
        const [x, y] = [valueOf(col, a), valueOf(col, b)];
        const cmp = typeof x === "number" && typeof y === "number" ? x - y : String(x ?? "").localeCompare(String(y ?? ""));
        return sort.desc ? -cmp : cmp;
      });
    count.textContent = q ? `${visible.length} of ${data.length}` : `${data.length}`;
    if (!visible.length) {
      return mount(body, h("tr", null, h("td", { class: "empty", colspan: columns.length + (actions ? 1 : 0) }, q ? "No matches" : emptyText)));
    }
    mount(body, visible.map((row) => {
      const open = onOpen && (() => onOpen(row));
      return h("tr", { tabindex: open ? "0" : null, class: open ? "clickable" : "", onclick: open,
        onkeydown: open && ((e) => { if (e.key === "Enter") open(); }) },
        columns.map((c) => h("td", { class: c.num ? "num" : c.wrap ? "wrap" : "" }, c.render ? c.render(row) : valueOf(c, row))),
        actions && h("td", { class: "row-actions", onclick: (e) => e.stopPropagation() }, actions(row)));
    }));
  }

  filter.addEventListener("input", draw);
  draw();
  const el = h("div", { class: "table-block" },
    filterable && h("div", { class: "toolbar" }, filter, count),
    h("div", { class: "scroll" }, h("table", { class: "dbx-table" }, h("thead", null, head), body)));
  return { el, setRows(next) { data = next; draw(); } };
}
