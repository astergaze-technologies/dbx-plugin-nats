<script setup lang="ts">
import { ref } from "vue";
import { invoke } from "../../api/bridge";
import type { KeyValue } from "../../api/types";
import DataTable, { type Column } from "../../components/DataTable.vue";
import ErrorBox from "../../components/ErrorBox.vue";
import MessageCard from "../../components/MessageCard.vue";
import WriteButton from "../../components/WriteButton.vue";
import { useLoader } from "../../lib/useLoader";
import { confirmAction, formDialog } from "../../stores/dialogs";

const props = defineProps<{ bucket: string }>();
const columns: Column<{ key: string }>[] = [{ key: "key", label: "Key", wrap: true }];

const { data: keys, error, load } = useLoader(async () => {
  const res = await invoke<{ keys: string[] }>("nats/kvKeys", { bucket: props.bucket });
  return res.keys.sort().map((key) => ({ key }));
});

const selected = ref("");
const history = ref<KeyValue[]>();
const detailError = ref<unknown>(null);
const status = ref("Select a key to see its value and history.");

async function show(key: string) {
  selected.value = key;
  history.value = undefined;
  detailError.value = null;
  status.value = "Loading value…";
  try {
    history.value = (await invoke<{ history: KeyValue[] }>("nats/kvHistory", { bucket: props.bucket, key })).history;
  } catch (err) {
    detailError.value = err;
  }
}

async function reload(key = selected.value) {
  await load();
  if (key) await show(key);
}
defineExpose({ reload });

function edit() {
  const latest = history.value?.[0];
  formDialog({
    title: `Edit ${selected.value}`,
    fields: [
      { key: "key", label: "Key", value: selected.value, disabled: true },
      { key: "value", label: "Value", type: "textarea", value: latest?.encoding === "utf8" ? latest.data : "" },
    ],
    async onSubmit(v) {
      await invoke("nats/kvPut", { bucket: props.bucket, key: selected.value, data: v.value });
      await show(selected.value);
    },
  });
}

async function remove() {
  const key = selected.value;
  const ok = await confirmAction({ title: `Delete ${key}?`, message: "A delete marker is added; earlier revisions stay in history until purged.",
    confirmLabel: "Delete", danger: true });
  if (!ok) return;
  try {
    await invoke("nats/kvDelete", { bucket: props.bucket, key });
    selected.value = "";
    history.value = undefined;
    status.value = "Key deleted.";
    load();
  } catch (err) {
    detailError.value = err;
  }
}
</script>

<template>
  <div>
  <ErrorBox :error="error" />
  <div class="split">
    <DataTable label="keys" :columns="columns" :rows="keys ?? []" empty-text="This bucket has no keys." @open="(k) => show(k.key)">
      <template #key="{ row }"><code>{{ row.key }}</code></template>
    </DataTable>
    <div class="detail">
      <ErrorBox :error="detailError" />
      <template v-if="history">
        <div class="toolbar">
          <code class="grow">{{ selected }}</code>
          <WriteButton icon="edit" @click="edit">Edit</WriteButton>
          <WriteButton icon="trash" variant="danger" @click="remove">Delete</WriteButton>
        </div>
        <h3 class="section-title">History ({{ history.length }})</h3>
        <MessageCard v-for="e in history" :key="e.revision" :message="e" :at="e.created">
          <span class="dbx-badge">rev {{ e.revision }}</span>
          <span class="dbx-badge">{{ e.operation }}</span>
        </MessageCard>
      </template>
      <div v-else-if="!detailError" class="empty">{{ status }}</div>
    </div>
  </div>
  </div>
</template>
