import { invoke, openFiles } from "../bridge.js";
import { confirmAction, formDialog } from "../components/dialog.js";
import { iconButton, pageHeader, segmented, writeButton } from "../components/page.js";
import { messageCard } from "../components/payload.js";
import { dataTable } from "../components/table.js";
import { badge, button, empty, errorBox, h, mount } from "../dom.js";
import { bytes, num, time } from "../format.js";

const specFields = (spec = {}, editing = false) => [
  { key: "name", label: "Name", value: spec.name, required: true, disabled: editing },
  { key: "subjects", label: "Subjects", value: (spec.subjects || []).join(", "), hint: "Comma separated; wildcards allowed (orders.>)" },
  { key: "description", label: "Description", value: spec.description },
  { key: "storage", label: "Storage", type: "select", value: spec.storage || "file", options: [["file", "File"], ["memory", "Memory"]], disabled: editing },
  { key: "retention", label: "Retention", type: "select", value: spec.retention || "limits", disabled: editing,
    options: [["limits", "Limits"], ["interest", "Interest"], ["workqueue", "Work queue"]] },
  { key: "discard", label: "When full, discard", type: "select", value: spec.discard || "old", options: [["old", "Old messages"], ["new", "New messages"]] },
  { key: "replicas", label: "Replicas", type: "number", value: spec.replicas || 1 },
  { key: "maxMsgs", label: "Max messages", type: "number", value: spec.maxMsgs > 0 ? spec.maxMsgs : "", hint: "Empty = unlimited" },
  { key: "maxBytes", label: "Max bytes", type: "number", value: spec.maxBytes > 0 ? spec.maxBytes : "", hint: "Empty = unlimited" },
  { key: "maxAgeSecs", label: "Max age (seconds)", type: "number", value: spec.maxAgeSecs || "", hint: "Empty = keep forever" },
];

export function streamForm(app, existing) {
  formDialog({
    title: existing ? `Edit stream ${existing.name}` : "Create stream",
    fields: specFields(existing, !!existing),
    submitLabel: existing ? "Save" : "Create",
    onSubmit: async (v) => {
      const streamSpec = { ...existing, ...v, subjects: v.subjects.split(",").map((s) => s.trim()).filter(Boolean) };
      await invoke("nats/streamSave", { streamSpec, update: !!existing });
      app.refresh("streams");
      app.openStream(streamSpec.name, true);
    },
  });
}

export function streamsTab(panel, app) {
  const table = dataTable({
    label: "streams",
    columns: [
      { key: "name", label: "Stream", render: (s) => h("strong", null, s.name) },
      { key: "subjects", label: "Subjects", wrap: true, value: (s) => (s.subjects || []).join(", "), render: (s) => h("code", null, (s.subjects || []).join(", ")) },
      { key: "retention", label: "Retention" },
      { key: "messages", label: "Messages", num: true, render: (s) => num(s.messages) },
      { key: "bytes", label: "Size", num: true, render: (s) => bytes(s.bytes) },
      { key: "consumers", label: "Consumers", num: true },
    ],
    onOpen: (s) => app.openStream(s.name),
    emptyText: "No streams on this server.",
  });
  let all = [];
  const showSystem = h("input", { type: "checkbox", id: "show-system" });
  const draw = () => table.setRows(showSystem.checked ? all : all.filter((s) => !s.system));
  showSystem.addEventListener("change", draw);
  const load = async () => {
    try {
      all = (await invoke("nats/streams")).streams;
      draw();
    } catch (err) {
      mount(panel, errorBox(err));
    }
  };
  mount(panel, pageHeader("Streams", "JetStream streams on this server.", [
    h("label", { class: "field inline hint", for: "show-system" }, showSystem, "Show KV / Object Store streams"),
    button("Refresh", load), writeButton("Create stream", () => streamForm(app), { iconName: "plus", primary: true }),
  ]), table.el);
  load();
}

export function streamTab(panel, app, name) {
  const body = h("div", { class: "tab-body" });
  let detail;
  let section = "messages";

  const show = () => ({ info, messages, consumers })[section]();

  async function load() {
    try {
      detail = await invoke("nats/stream", { stream: name });
    } catch (err) {
      return mount(panel, errorBox(err));
    }
    mount(panel,
      pageHeader(name, `${detail.retention} · ${detail.storage} · ${num(detail.messages)} messages · ${bytes(detail.bytes)}`, [
        button("Browse as files", () => openFiles(`streams/${encodeURIComponent(name)}`)),
        button("Refresh", load),
        writeButton("Edit", () => streamForm(app, detail.spec), { iconName: "edit" }),
        writeButton("Purge", purge),
        writeButton("Delete", remove, { iconName: "trash", danger: true }),
      ]),
      segmented([["messages", "Messages"], ["consumers", `Consumers (${detail.consumers})`], ["info", "Configuration"]], section, (id) => { section = id; show(); }),
      body);
    show();
  }

  function info() {
    const s = detail.spec;
    const rows = [
      ["Subjects", (s.subjects || []).join(", ") || "—"], ["Description", s.description || "—"],
      ["Storage", s.storage], ["Retention", s.retention], ["Discard", s.discard], ["Replicas", s.replicas],
      ["Max messages", s.maxMsgs > 0 ? num(s.maxMsgs) : "unlimited"], ["Max bytes", s.maxBytes > 0 ? bytes(s.maxBytes) : "unlimited"],
      ["Max age", s.maxAgeSecs ? `${s.maxAgeSecs}s` : "forever"], ["First / last sequence", `${detail.firstSeq} / ${detail.lastSeq}`],
      ["Created", time(detail.created)],
    ];
    mount(body, h("dl", { class: "props card" }, rows.map(([k, v]) => [h("dt", null, k), h("dd", null, String(v))])));
  }

  function messages(before = 0, list) {
    if (!list) {
      list = h("div", { class: "msg-list" });
      mount(body, list);
    }
    list.querySelector(".more")?.remove();
    const loading = empty("Loading messages…");
    list.append(loading);
    invoke("nats/streamMessages", { stream: name, before, limit: 50 }, 30000).then((page) => {
      loading.remove();
      if (!page.messages.length && !before) list.append(empty("This stream has no messages."));
      for (const m of page.messages) {
        const card = messageCard(m, [iconButton("trash", `Delete message ${m.seq}`, async () => {
          if (await confirmAction({ title: "Delete message?", message: `Message #${m.seq} will be removed from ${name}.`, confirmLabel: "Delete", danger: true })) {
            await invoke("nats/messageDelete", { stream: name, seq: m.seq }).then(() => card.remove(), (err) => card.prepend(errorBox(err)));
          }
        }, { danger: true, write: true })]);
        list.append(card);
      }
      if (page.nextBefore) list.append(h("div", { class: "more" }, button("Load older", () => messages(page.nextBefore, list))));
    }, (err) => { loading.remove(); list.append(errorBox(err)); });
  }

  async function consumers() {
    mount(body, empty("Loading consumers…"));
    try {
      const { consumers: rows } = await invoke("nats/consumers", { stream: name });
      const table = dataTable({
        label: "consumers",
        columns: [
          { key: "name", label: "Consumer", render: (c) => h("span", null, h("strong", null, c.name), " ", badge(c.pull ? "pull" : "push"), c.durable ? "" : " ", !c.durable && badge("ephemeral")) },
          { key: "filter", label: "Filter", value: (c) => (c.filterSubjects || []).join(", "), render: (c) => h("code", null, (c.filterSubjects || []).join(", ") || "all") },
          { key: "numPending", label: "Pending", num: true, render: (c) => num(c.numPending) },
          { key: "numAckPending", label: "Ack pending", num: true, render: (c) => num(c.numAckPending) },
          { key: "numRedelivered", label: "Redelivered", num: true, render: (c) => num(c.numRedelivered) },
          { key: "deliveredSeq", label: "Delivered", num: true },
          { key: "ackPolicy", label: "Ack" },
        ],
        rows,
        emptyText: "No consumers on this stream.",
        actions: (c) => iconButton("trash", `Delete consumer ${c.name}`, async () => {
          if (await confirmAction({ title: "Delete consumer?", message: `Consumer ${c.name} and its delivery state will be removed.`, confirmLabel: "Delete", danger: true })) {
            await invoke("nats/consumerDelete", { stream: name, consumer: c.name });
            consumers();
          }
        }, { danger: true, write: true }),
      });
      mount(body, table.el);
    } catch (err) {
      mount(body, errorBox(err));
    }
  }

  async function purge() {
    if (!(await confirmAction({ title: `Purge ${name}?`, message: "Every message in the stream is removed. The stream and its consumers stay.", confirmLabel: "Purge", danger: true, typeToConfirm: name }))) return;
    await invoke("nats/streamPurge", { stream: name }).then(load, (err) => body.prepend(errorBox(err)));
    app.refresh("streams");
  }

  async function remove() {
    if (!(await confirmAction({ title: `Delete ${name}?`, message: "The stream, all its messages and consumers are permanently deleted.", confirmLabel: "Delete stream", danger: true, typeToConfirm: name }))) return;
    await invoke("nats/streamDelete", { stream: name }).then(() => { app.refresh("streams"); app.closeTab(`stream:${name}`); }, (err) => body.prepend(errorBox(err)));
  }

  load();
}
