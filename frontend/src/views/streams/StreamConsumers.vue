<script setup lang="ts">
import { invoke } from "../../api/bridge";
import type { Consumer } from "../../api/types";
import DataTable, { type Column } from "../../components/DataTable.vue";
import ErrorBox from "../../components/ErrorBox.vue";
import IconButton from "../../components/IconButton.vue";
import { num } from "../../lib/format";
import { useLoader } from "../../lib/useLoader";
import { confirmAction } from "../../stores/dialogs";

const props = defineProps<{ stream: string }>();
const { data, error, load } = useLoader(async () =>
  (await invoke<{ consumers: Consumer[] }>("nats/consumers", { stream: props.stream })).consumers);

const filter = (c: Consumer) => (c.filterSubjects || []).join(", ");
const columns: Column<Consumer>[] = [
  { key: "name", label: "Consumer" },
  { key: "filter", label: "Filter", value: filter },
  { key: "numPending", label: "Pending", num: true },
  { key: "numAckPending", label: "Ack pending", num: true },
  { key: "numRedelivered", label: "Redelivered", num: true },
  { key: "deliveredSeq", label: "Delivered", num: true },
  { key: "ackPolicy", label: "Ack" },
];

async function remove(c: Consumer) {
  const ok = await confirmAction({ title: "Delete consumer?", message: `Consumer ${c.name} and its delivery state will be removed.`, confirmLabel: "Delete", danger: true });
  if (!ok) return;
  await invoke("nats/consumerDelete", { stream: props.stream, consumer: c.name }).catch((err) => (error.value = err));
  load();
}
</script>

<template>
  <ErrorBox :error="error" />
  <div v-if="!data && !error" class="empty">Loading consumers…</div>
  <DataTable v-if="data" label="consumers" :columns="columns" :rows="data" empty-text="No consumers on this stream.">
    <template #name="{ row }">
      <strong>{{ row.name }}</strong> <span class="dbx-badge">{{ row.pull ? "pull" : "push" }}</span>
      <span v-if="!row.durable" class="dbx-badge">ephemeral</span>
    </template>
    <template #filter="{ row }"><code>{{ filter(row) || "all" }}</code></template>
    <template #numPending="{ row }">{{ num(row.numPending) }}</template>
    <template #numAckPending="{ row }">{{ num(row.numAckPending) }}</template>
    <template #numRedelivered="{ row }">{{ num(row.numRedelivered) }}</template>
    <template #actions="{ row }">
      <IconButton icon="trash" :label="`Delete consumer ${row.name}`" danger write @click="remove(row)" />
    </template>
  </DataTable>
</template>
