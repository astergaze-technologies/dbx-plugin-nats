<script setup lang="ts">
import { ref } from "vue";
import { invoke, openFiles } from "../../api/bridge";
import type { ObjectInfo } from "../../api/types";
import DataTable, { type Column } from "../../components/DataTable.vue";
import ErrorBox from "../../components/ErrorBox.vue";
import IconButton from "../../components/IconButton.vue";
import PageHeader from "../../components/PageHeader.vue";
import WriteButton from "../../components/WriteButton.vue";
import { bytes, time } from "../../lib/format";
import { useLoader } from "../../lib/useLoader";
import { confirmAction } from "../../stores/dialogs";
import { closeTab, refreshList } from "../../stores/workspace";
import ObjectPreview from "./ObjectPreview.vue";

const MAX_UPLOAD = 4 * 1024 * 1024; // matches the backend limit

const props = defineProps<{ store: string }>();
const fileInput = ref<HTMLInputElement>();
const selected = ref("");
const status = ref("");
const error = ref<unknown>(null);

const { data, error: loadError, load } = useLoader(async () =>
  (await invoke<{ objects: ObjectInfo[] }>("nats/objects", { store: props.store })).objects);

const columns: Column<ObjectInfo>[] = [
  { key: "name", label: "Object", wrap: true },
  { key: "size", label: "Size", num: true },
  { key: "modified", label: "Modified" },
];

const readBase64 = (file: File) =>
  new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result).split(",")[1] || "");
    reader.onerror = () => reject(reader.error);
    reader.readAsDataURL(file);
  });

async function upload() {
  const file = fileInput.value!.files?.[0];
  fileInput.value!.value = "";
  if (!file) return;
  error.value = null;
  if (file.size > MAX_UPLOAD) {
    error.value = `Upload limit is ${bytes(MAX_UPLOAD)}; use the nats CLI for larger objects.`;
    return;
  }
  status.value = `Uploading ${file.name}…`;
  try {
    await invoke("nats/objectPut", { store: props.store, name: file.name, dataBase64: await readBase64(file) }, 60000);
    load();
  } catch (err) {
    error.value = err;
  } finally {
    status.value = "";
  }
}

async function remove(o: ObjectInfo) {
  const ok = await confirmAction({ title: `Delete ${o.name}?`, message: "The object is permanently deleted.", confirmLabel: "Delete", danger: true });
  if (!ok) return;
  try {
    await invoke("nats/objectDelete", { store: props.store, name: o.name });
    if (selected.value === o.name) selected.value = "";
    load();
  } catch (err) {
    error.value = err;
  }
}

async function removeStore() {
  const ok = await confirmAction({ title: `Delete store ${props.store}?`, message: "Every object in the store is permanently deleted.",
    confirmLabel: "Delete store", danger: true, typeToConfirm: props.store });
  if (!ok) return;
  try {
    await invoke("nats/objectStoreDelete", { store: props.store });
    refreshList("objects");
    closeTab(`objects:${props.store}`);
  } catch (err) {
    error.value = err;
  }
}
</script>

<template>
  <PageHeader :title="store" subtitle="Object store">
    <button type="button" class="dbx-btn" @click="openFiles('objects', store)">Browse as files</button>
    <button type="button" class="dbx-btn" @click="load">Refresh</button>
    <WriteButton icon="upload" variant="primary" @click="fileInput?.click()">Upload</WriteButton>
    <WriteButton icon="trash" variant="danger" @click="removeStore">Delete store</WriteButton>
  </PageHeader>
  <input ref="fileInput" type="file" hidden @change="upload" />
  <div v-if="status" class="empty">{{ status }}</div>
  <ErrorBox :error="error || loadError" />
  <div class="split">
    <DataTable label="objects" :columns="columns" :rows="data ?? []" empty-text="This store is empty." @open="(o) => (selected = o.name)">
      <template #name="{ row }"><code>{{ row.name }}</code></template>
      <template #size="{ row }">{{ bytes(row.size) }}</template>
      <template #modified="{ row }">{{ time(row.modified) }}</template>
      <template #actions="{ row }">
        <IconButton icon="trash" :label="`Delete ${row.name}`" danger write @click="remove(row)" />
      </template>
    </DataTable>
    <div class="detail">
      <ObjectPreview v-if="selected" :key="selected" :store="store" :name="selected" />
      <div v-else class="empty">Select an object to preview it.</div>
    </div>
  </div>
</template>
