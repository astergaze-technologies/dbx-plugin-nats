<script setup lang="ts">
import { watch } from "vue";
import { invoke } from "../../api/bridge";
import type { Bucket } from "../../api/types";
import DataTable, { type Column } from "../../components/DataTable.vue";
import ErrorBox from "../../components/ErrorBox.vue";
import PageHeader from "../../components/PageHeader.vue";
import WriteButton from "../../components/WriteButton.vue";
import { bytes, num } from "../../lib/format";
import { useLoader } from "../../lib/useLoader";
import { refreshList, workspace } from "../../stores/workspace";
import { formDialog } from "../../stores/dialogs";
import { openBucket } from "../../stores/navigation";

const { data, error, load } = useLoader(async () => (await invoke<{ buckets: Bucket[] }>("nats/kvBuckets")).buckets);
watch(() => workspace.listVersion.kv, load);

const columns: Column<Bucket>[] = [
  { key: "bucket", label: "Bucket" },
  { key: "values", label: "Entries", num: true },
  { key: "bytes", label: "Size", num: true },
  { key: "history", label: "History", num: true },
  { key: "ttl", label: "TTL" },
  { key: "storage", label: "Storage" },
];

function create() {
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
    async onSubmit(bucketSpec) {
      await invoke("nats/kvBucketCreate", { bucketSpec });
      refreshList("kv");
      openBucket(String(bucketSpec.bucket));
    },
  });
}
</script>

<template>
  <PageHeader title="Key-Value" subtitle="Buckets backed by JetStream.">
    <button type="button" class="dbx-btn" @click="load">Refresh</button>
    <WriteButton icon="plus" variant="primary" @click="create">Create bucket</WriteButton>
  </PageHeader>
  <ErrorBox :error="error" />
  <DataTable label="buckets" :columns="columns" :rows="data ?? []" empty-text="No key-value buckets on this server."
    @open="(b) => openBucket(b.bucket)">
    <template #bucket="{ row }"><strong>{{ row.bucket }}</strong> <span v-if="row.compressed" class="dbx-badge">compressed</span></template>
    <template #values="{ row }">{{ num(row.values) }}</template>
    <template #bytes="{ row }">{{ bytes(row.bytes) }}</template>
  </DataTable>
</template>
