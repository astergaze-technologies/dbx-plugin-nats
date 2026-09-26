<script setup lang="ts">
import { invoke } from "../../api/bridge";
import type { Service, ServiceEndpoint } from "../../api/types";
import DataTable, { type Column } from "../../components/DataTable.vue";
import ErrorBox from "../../components/ErrorBox.vue";
import PageHeader from "../../components/PageHeader.vue";
import { num, time } from "../../lib/format";
import { useLoader } from "../../lib/useLoader";

const { data: services, error, loading, load } = useLoader(async () => (await invoke<{ services: Service[] }>("nats/services")).services);

const columns: Column<ServiceEndpoint>[] = [
  { key: "name", label: "Endpoint" },
  { key: "subject", label: "Subject" },
  { key: "queueGroup", label: "Queue" },
  { key: "requests", label: "Requests", num: true },
  { key: "errors", label: "Errors", num: true },
  { key: "avgMs", label: "Avg ms", num: true },
  { key: "lastError", label: "Last error", wrap: true },
];
</script>

<template>
  <PageHeader title="Services" subtitle="NATS micro services discovered via $SRV.INFO and $SRV.STATS.">
    <button type="button" class="dbx-btn" :disabled="loading" @click="load">Refresh</button>
  </PageHeader>
  <ErrorBox :error="error" />
  <div v-if="loading" class="empty">Discovering services (1s)…</div>
  <div v-else-if="services && !services.length" class="empty">No NATS micro services answered $SRV.INFO.</div>
  <section v-for="s in loading ? [] : services" :key="s.id" class="card service">
    <header class="toolbar">
      <strong>{{ s.name }}</strong>
      <span class="dbx-badge">v{{ s.version }}</span>
      <code class="hint">{{ s.id }}</code>
      <span class="hint grow">{{ s.description }}</span>
      <span class="hint">since {{ time(s.started) }}</span>
    </header>
    <DataTable :label="`${s.name} endpoints`" :columns="columns" :rows="s.endpoints" :filterable="false">
      <template #subject="{ row }"><code>{{ row.subject }}</code></template>
      <template #requests="{ row }">{{ num(row.requests) }}</template>
      <template #errors="{ row }">{{ num(row.errors) }}</template>
      <template #avgMs="{ row }">{{ row.avgMs.toFixed(2) }}</template>
    </DataTable>
  </section>
</template>
