<script setup lang="ts">
import { watch } from "vue";
import { invoke } from "../../api/bridge";
import type { ObjectStore } from "../../api/types";
import DataTable, { type Column } from "../../components/DataTable.vue";
import ErrorBox from "../../components/ErrorBox.vue";
import PageHeader from "../../components/PageHeader.vue";
import WriteButton from "../../components/WriteButton.vue";
import { bytes } from "../../lib/format";
import { useLoader } from "../../lib/useLoader";
import { refreshList, workspace } from "../../stores/workspace";
import { formDialog } from "../../stores/dialogs";
import { openStore } from "../../stores/navigation";

const { data, error, load } = useLoader(async () => (await invoke<{ stores: ObjectStore[] }>("nats/objectStores")).stores);
watch(() => workspace.listVersion.objects, load);

const columns: Column<ObjectStore>[] = [
  { key: "store", label: "Store" },
  { key: "description", label: "Description" },
  { key: "size", label: "Size", num: true },
  { key: "storage", label: "Storage" },
  { key: "ttl", label: "TTL" },
];

function create() {
  formDialog({
    title: "Create object store",
    fields: [{ key: "store", label: "Store", required: true }, { key: "description", label: "Description" }],
    submitLabel: "Create",
    async onSubmit(v) {
      await invoke("nats/objectStoreCreate", v);
      refreshList("objects");
      openStore(String(v.store));
    },
  });
}
</script>

<template>
  <PageHeader title="Object Store" subtitle="Large objects chunked over JetStream.">
    <button type="button" class="dbx-btn" @click="load">Refresh</button>
    <WriteButton icon="plus" variant="primary" @click="create">Create store</WriteButton>
  </PageHeader>
  <ErrorBox :error="error" />
  <DataTable label="object stores" :columns="columns" :rows="data ?? []" empty-text="No object stores on this server."
    @open="(s) => openStore(s.store)">
    <template #store="{ row }"><strong>{{ row.store }}</strong> <span v-if="row.sealed" class="dbx-badge">sealed</span></template>
    <template #size="{ row }">{{ bytes(row.size) }}</template>
  </DataTable>
</template>
