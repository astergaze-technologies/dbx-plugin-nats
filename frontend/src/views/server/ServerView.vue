<script setup lang="ts">
import { computed } from "vue";
import { canBrowseFiles, invoke, openFiles, session } from "../../api/bridge";
import type { Overview } from "../../api/types";
import ErrorBox from "../../components/ErrorBox.vue";
import PageHeader from "../../components/PageHeader.vue";
import PropList from "../../components/PropList.vue";
import { bytes, num } from "../../lib/format";
import { useLoader } from "../../lib/useLoader";

const { data: o, error, load } = useLoader(async () => {
  const overview = await invoke<Overview>("nats/overview");
  session.readOnly = overview.readOnly;
  return overview;
});

const limit = (max: number) => (max > 0 ? `of ${bytes(max)}` : "no account limit");

const stats = computed(() => {
  const js = o.value?.jetstream;
  if (!js) return [];
  return [
    ["Streams", num(js.streams)], ["Consumers", num(js.consumers)],
    ["Storage", bytes(js.storage), limit(js.maxStore)], ["Memory", bytes(js.memory), limit(js.maxMemory)],
  ];
});
</script>

<template>
  <ErrorBox :error="error" />
  <div v-if="!o && !error" class="empty">Loading server information…</div>
  <template v-if="o">
    <PageHeader :title="o.server.name" :subtitle="`NATS ${o.server.version}${o.server.cluster ? ` · cluster ${o.server.cluster}` : ''}`">
      <span v-if="o.readOnly" class="dbx-badge">read-only</span>
      <span v-if="o.server.tls" class="dbx-badge">TLS</span>
      <span class="dbx-badge">{{ o.jetstream ? "JetStream" : "no JetStream" }}</span>
      <button v-if="canBrowseFiles()" type="button" class="dbx-btn" @click="openFiles()">Browse as files</button>
      <button type="button" class="dbx-btn" @click="load">Refresh</button>
    </PageHeader>
    <div v-if="o.jetstream" class="stats">
      <div v-for="[label, value, sub] in stats" :key="label" class="stat">
        <div class="stat-label">{{ label }}</div>
        <div class="stat-value">{{ value }}</div>
        <div v-if="sub" class="hint">{{ sub }}</div>
      </div>
    </div>
    <p v-else class="hint">JetStream is not enabled for this account: streams, key-value and object stores are unavailable.</p>
    <PropList :items="[
      ['Server ID', o.server.id], ['URL', o.server.url], ['Round trip', `${o.server.rttMs.toFixed(1)} ms`],
      ['Max payload', bytes(o.server.maxPayload)], ['Headers', o.server.headers ? 'supported' : 'not supported'],
    ]" />
  </template>
</template>
