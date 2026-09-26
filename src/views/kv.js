import { invoke, on, openFiles } from "../bridge.js";
import { confirmAction, formDialog } from "../components/dialog.js";
import { pageHeader, segmented, writeButton } from "../components/page.js";
import { messageCard } from "../components/payload.js";
import { dataTable } from "../components/table.js";
import { badge, button, empty, errorBox, h, mount } from "../dom.js";
import { bytes, num } from "../format.js";

function createBucket(app) {
  formDialog({
    title: "Create key-value bucket",
    fields: [
      { key: "bucket", label: "Bucket", required: true },
      { key: "description", label: "Description" },
      { key: "history", label: "History (revisions per key)", type: "number", value: 1 },
      { key: "ttlSecs", label: "TTL (seconds)", type: "number", value: "", hint: "Empty = keys never expire" },
      { key: "storage", label: "Storage", type: "select", value: "file", options: [["file", "File"], ["memory", "Memory"]] },
    ],
    submitLabel: "Create",
    onSubmit: async (bucketSpec) => {
      await invoke("nats/kvBucketCreate", { bucketSpec });
      app.refresh("kv");
      app.openBucket(bucketSpec.bucket);
    },
  });
}

export function bucketsTab(panel, app) {
  const table = dataTable({
    label: "buckets",
    columns: [
      { key: "bucket", label: "Bucket", render: (b) => h("span", null, h("strong", null, b.bucket), b.compressed ? " " : "", b.compressed && badge("compressed")) },
      { key: "values", label: "Entries", num: true, render: (b) => num(b.values) },
      { key: "bytes", label: "Size", num: true, render: (b) => bytes(b.bytes) },
      { key: "history", label: "History", num: true },
      { key: "ttl", label: "TTL" },
      { key: "storage", label: "Storage" },
    ],
    onOpen: (b) => app.openBucket(b.bucket),
    emptyText: "No key-value buckets on this server.",
  });
  const load = async () => {
    try {
      table.setRows((await invoke("nats/kvBuckets")).buckets);
    } catch (err) {
      mount(panel, errorBox(err));
    }
  };
  mount(panel, pageHeader("Key-Value", "Buckets backed by JetStream.", [
    button("Refresh", load), writeButton("Create bucket", () => createBucket(app), { iconName: "plus", primary: true }),
  ]), table.el);
  load();
}

export function bucketTab(panel, app, bucket) {
  const detail = h("div", { class: "detail" }, empty("Select a key to see its value and history."));
  const watchLog = h("div", { class: "msg-list" });
  let stopWatch = null;
  let feedId = null;

  const keys = dataTable({
    label: "keys",
    columns: [{ key: "key", label: "Key", wrap: true, render: (k) => h("code", null, k.key) }],
    onOpen: (k) => showKey(k.key),
    emptyText: "This bucket has no keys.",
  });

  async function loadKeys() {
    try {
      const res = await invoke("nats/kvKeys", { bucket });
      keys.setRows(res.keys.sort().map((key) => ({ key })));
    } catch (err) {
      mount(detail, errorBox(err));
    }
  }

  async function showKey(key) {
    mount(detail, empty("Loading value…"));
    try {
      const { history } = await invoke("nats/kvHistory", { bucket, key });
      const latest = history[0];
      mount(detail,
        h("div", { class: "toolbar" }, h("code", { class: "grow" }, key),
          writeButton("Edit", () => putKey(key, latest.encoding === "utf8" ? latest.data : ""), { iconName: "edit" }),
          writeButton("Delete", () => deleteKey(key), { iconName: "trash", danger: true })),
        h("h3", { class: "section-title" }, `History (${history.length})`),
        history.map((e) => messageCard({ ...e, time: e.created }, [badge(`rev ${e.revision}`), badge(e.operation)])));
    } catch (err) {
      mount(detail, errorBox(err));
    }
  }

  function putKey(key = "", value = "") {
    formDialog({
      title: key ? `Edit ${key}` : "Put key",
      fields: [{ key: "key", label: "Key", value: key, required: true, disabled: !!key }, { key: "value", label: "Value", type: "textarea", value }],
      submitLabel: "Save",
      onSubmit: async (v) => {
        await invoke("nats/kvPut", { bucket, key: v.key || key, data: v.value });
        await loadKeys();
        showKey(v.key || key);
      },
    });
  }

  async function deleteKey(key) {
    const purge = await confirmAction({ title: `Delete ${key}?`, message: "A delete marker is added; earlier revisions stay in history until purged.", confirmLabel: "Delete", danger: true });
    if (!purge) return;
    await invoke("nats/kvDelete", { bucket, key }).then(() => { loadKeys(); mount(detail, empty("Key deleted.")); }, (err) => detail.prepend(errorBox(err)));
  }

  async function toggleWatch(btn) {
    if (feedId) {
      await invoke("nats/feedStop", { feedId }).catch(() => {});
      stopWatch?.();
      feedId = null;
      btn.textContent = "Watch changes";
      return;
    }
    const res = await invoke("nats/kvWatch", { bucket, pattern: ">" });
    feedId = res.feedId;
    btn.textContent = "Stop watching";
    mount(watchLog);
    stopWatch = on("nats/kvChange", (c) => {
      if (c.feedId !== feedId) return;
      watchLog.prepend(messageCard({ ...c, subject: c.key, time: c.created }, [badge(c.operation), badge(`rev ${c.revision}`)]));
      while (watchLog.childElementCount > 200) watchLog.lastChild.remove();
    });
  }

  async function removeBucket() {
    if (!(await confirmAction({ title: `Delete bucket ${bucket}?`, message: "Every key and its history is permanently deleted.", confirmLabel: "Delete bucket", danger: true, typeToConfirm: bucket }))) return;
    await invoke("nats/kvBucketDelete", { bucket });
    app.refresh("kv");
    app.closeTab(`kv:${bucket}`);
  }

  const watchBtn = button("Watch changes", () => toggleWatch(watchBtn));
  const keysView = h("div", { class: "split" }, keys.el, detail);
  const watchView = h("div", { hidden: true },
    h("div", { class: "toolbar" }, h("p", { class: "hint grow" }, "Live changes to any key in this bucket, newest first."), watchBtn), watchLog);
  mount(panel,
    pageHeader(bucket, "Key-value bucket", [
      button("Browse as files", () => openFiles(`kv/${encodeURIComponent(bucket)}`)),
      button("Refresh", loadKeys),
      writeButton("Put key", () => putKey(), { iconName: "plus", primary: true }),
      writeButton("Delete bucket", removeBucket, { iconName: "trash", danger: true }),
    ]),
    segmented([["keys", "Keys"], ["watch", "Watch"]], "keys", (id) => { keysView.hidden = id !== "keys"; watchView.hidden = id !== "watch"; }),
    keysView,
    watchView);
  loadKeys();
  return () => feedId && invoke("nats/feedStop", { feedId }).catch(() => {});
}
