<script setup lang="ts">
import { computed, ref } from "vue";
import { invoke } from "../../api/bridge";
import type { StreamSummary } from "../../api/types";
import DataTable, { type Column } from "../../components/DataTable.vue";
import ErrorBox from "../../components/ErrorBox.vue";
import PageHeader from "../../components/PageHeader.vue";
import WriteButton from "../../components/WriteButton.vue";
import { bytes, num } from "../../lib/format";
import { useLoader } from "../../lib/useLoader";
import { openStream } from "../../stores/navigation";
import { streamForm } from "./streamForm";

const showSystem = ref(false);
const { data, error, load } = useLoader(async () => (await invoke<{ streams: StreamSummary[] }>("nats/streams")).streams);
const rows = computed(() => (data.value ?? []).filter((s) => showSystem.value || !s.system));

const columns: Column<StreamSummary>[] = [
  { key: "name", label: "Stream" },
  { key: "subjects", label: "Subjects", wrap: true, value: (s) => (s.subjects || []).join(", ") },
  { key: "retention", label: "Retention" },
  { key: "messages", label: "Messages", num: true },
  { key: "bytes", label: "Size", num: true },
  { key: "consumers", label: "Consumers", num: true },
];
</script>

<template>
  <PageHeader title="Streams" subtitle="JetStream streams on this server.">
    <label class="field inline hint"><input v-model="showSystem" type="checkbox" /> Show KV / Object Store streams</label>
    <button type="button" class="dbx-btn" @click="load">Refresh</button>
    <WriteButton icon="plus" variant="primary" @click="streamForm()">Create stream</WriteButton>
  </PageHeader>
  <ErrorBox :error="error" />
  <DataTable label="streams" :columns="columns" :rows="rows" empty-text="No streams on this server." @open="(s) => openStream(s.name)">
    <template #name="{ row }"><strong>{{ row.name }}</strong></template>
    <template #subjects="{ row }"><code>{{ (row.subjects || []).join(", ") }}</code></template>
    <template #messages="{ row }">{{ num(row.messages) }}</template>
    <template #bytes="{ row }">{{ bytes(row.bytes) }}</template>
  </DataTable>
</template>
