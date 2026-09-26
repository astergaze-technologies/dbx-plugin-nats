import { invoke, openFiles } from "../bridge.js";
import { confirmAction, formDialog } from "../components/dialog.js";
import { iconButton, pageHeader, writeButton } from "../components/page.js";
import { dataTable } from "../components/table.js";
import { badge, button, empty, errorBox, h, mount } from "../dom.js";
import { bytes, time } from "../format.js";

const MAX_UPLOAD = 4 * 1024 * 1024;

export function storesTab(panel, app) {
  const table = dataTable({
    label: "object stores",
    columns: [
      { key: "store", label: "Store", render: (s) => h("span", null, h("strong", null, s.store), s.sealed ? " " : "", s.sealed && badge("sealed")) },
      { key: "description", label: "Description" },
      { key: "size", label: "Size", num: true, render: (s) => bytes(s.size) },
      { key: "storage", label: "Storage" },
      { key: "ttl", label: "TTL" },
    ],
    onOpen: (s) => app.openStore(s.store),
    emptyText: "No object stores on this server.",
  });
  const load = async () => {
    try {
      table.setRows((await invoke("nats/objectStores")).stores);
    } catch (err) {
      mount(panel, errorBox(err));
    }
  };
  const create = () => formDialog({
    title: "Create object store",
    fields: [{ key: "store", label: "Store", required: true }, { key: "description", label: "Description" }],
    submitLabel: "Create",
    onSubmit: async (v) => {
      await invoke("nats/objectStoreCreate", v);
      app.refresh("objects");
      app.openStore(v.store);
    },
  });
  mount(panel, pageHeader("Object Store", "Large objects chunked over JetStream.", [
    button("Refresh", load), writeButton("Create store", create, { iconName: "plus", primary: true }),
  ]), table.el);
  load();
}

function readFile(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result).split(",")[1] || "");
    reader.onerror = () => reject(reader.error);
    reader.readAsDataURL(file);
  });
}

export function storeTab(panel, app, store) {
  const preview = h("div", { class: "detail" }, empty("Select an object to preview it."));
  const status = h("div");
  const table = dataTable({
    label: "objects",
    columns: [
      { key: "name", label: "Object", wrap: true, render: (o) => h("code", null, o.name) },
      { key: "size", label: "Size", num: true, render: (o) => bytes(o.size) },
      { key: "modified", label: "Modified", render: (o) => time(o.modified) },
    ],
    onOpen: (o) => show(o),
    emptyText: "This store is empty.",
    actions: (o) => iconButton("trash", `Delete ${o.name}`, async () => {
      if (await confirmAction({ title: `Delete ${o.name}?`, message: "The object is permanently deleted.", confirmLabel: "Delete", danger: true })) {
        await invoke("nats/objectDelete", { store, name: o.name }).then(load, (err) => mount(status, errorBox(err)));
      }
    }, { danger: true, write: true }),
  });

  async function load() {
    try {
      table.setRows((await invoke("nats/objects", { store })).objects);
    } catch (err) {
      mount(status, errorBox(err));
    }
  }

  async function show(o) {
    mount(preview, empty("Loading object…"));
    try {
      const obj = await invoke("nats/objectGet", { store, name: o.name }, 60000);
      const raw = atob(obj.data || "");
      const text = /^[\x09\x0A\x0D\x20-\x7E -￿]*$/.test(raw.slice(0, 4096)) ? raw : `[binary, ${bytes(obj.size)}]`;
      mount(preview,
        h("dl", { class: "props card" }, [["Name", obj.name], ["Size", bytes(obj.size)], ["Chunks", obj.chunks], ["Digest", obj.digest], ["Modified", time(obj.modified)]]
          .map(([k, v]) => [h("dt", null, k), h("dd", null, String(v))])),
        h("pre", null, text.length > 65536 ? `${text.slice(0, 65536)}\n… truncated` : text));
    } catch (err) {
      mount(preview, errorBox(err));
    }
  }

  const fileInput = h("input", { type: "file", hidden: true });
  fileInput.addEventListener("change", async () => {
    const file = fileInput.files[0];
    fileInput.value = "";
    if (!file) return;
    if (file.size > MAX_UPLOAD) return mount(status, errorBox(`Upload limit is ${bytes(MAX_UPLOAD)}; use the nats CLI for larger objects.`));
    mount(status, empty(`Uploading ${file.name}…`));
    try {
      await invoke("nats/objectPut", { store, name: file.name, dataBase64: await readFile(file) }, 60000);
      mount(status);
      load();
    } catch (err) {
      mount(status, errorBox(err));
    }
  });

  async function removeStore() {
    if (!(await confirmAction({ title: `Delete store ${store}?`, message: "Every object in the store is permanently deleted.", confirmLabel: "Delete store", danger: true, typeToConfirm: store }))) return;
    await invoke("nats/objectStoreDelete", { store });
    app.refresh("objects");
    app.closeTab(`objects:${store}`);
  }

  const upload = writeButton("Upload", () => fileInput.click(), { iconName: "upload", primary: true });
  mount(panel,
    pageHeader(store, "Object store", [
      button("Browse as files", () => openFiles(`objects/${encodeURIComponent(store)}`)),
      button("Refresh", load), upload, writeButton("Delete store", removeStore, { iconName: "trash", danger: true }),
    ]),
    fileInput, status, h("div", { class: "split" }, table.el, preview));
  load();
}
