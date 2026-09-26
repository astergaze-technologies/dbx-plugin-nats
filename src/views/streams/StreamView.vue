<script setup lang="ts">
import { ref } from "vue";
import { invoke, openFiles } from "../../api/bridge";
import type { StreamDetail } from "../../api/types";
import ErrorBox from "../../components/ErrorBox.vue";
import PageHeader from "../../components/PageHeader.vue";
import PropList from "../../components/PropList.vue";
import SegmentedControl from "../../components/SegmentedControl.vue";
import WriteButton from "../../components/WriteButton.vue";
import { bytes, num, time } from "../../lib/format";
import { useLoader } from "../../lib/useLoader";
import { confirmAction } from "../../stores/dialogs";
import { closeTab, refreshList } from "../../stores/workspace";
import StreamConsumers from "./StreamConsumers.vue";
import StreamMessages from "./StreamMessages.vue";
import { streamForm } from "./streamForm";

const props = defineProps<{ name: string }>();
const section = ref("messages");
const actionError = ref<unknown>(null);
const { data: detail, error, load } = useLoader(() => invoke<StreamDetail>("nats/stream", { stream: props.name }));
const version = ref(0); // remounts the active section on refresh
const refresh = () => {
  version.value++;
  load();
};

const config = (d: StreamDetail): [string, unknown][] => {
  const s = d.spec;
  return [
    ["Subjects", (s.subjects || []).join(", ") || "—"], ["Description", s.description || "—"],
    ["Storage", s.storage], ["Retention", s.retention], ["Discard", s.discard], ["Replicas", s.replicas],
    ["Max messages", s.maxMsgs > 0 ? num(s.maxMsgs) : "unlimited"], ["Max bytes", s.maxBytes > 0 ? bytes(s.maxBytes) : "unlimited"],
    ["Max age", s.maxAgeSecs ? `${s.maxAgeSecs}s` : "forever"], ["First / last sequence", `${d.firstSeq} / ${d.lastSeq}`],
    ["Created", time(d.created)],
  ];
};

async function purge() {
  const ok = await confirmAction({ title: `Purge ${props.name}?`, message: "Every message in the stream is removed. The stream and its consumers stay.",
    confirmLabel: "Purge", danger: true, typeToConfirm: props.name });
  if (!ok) return;
  try {
    await invoke("nats/streamPurge", { stream: props.name });
    refreshList("streams");
    refresh();
  } catch (err) {
    actionError.value = err;
  }
}

async function remove() {
  const ok = await confirmAction({ title: `Delete ${props.name}?`, message: "The stream, all its messages and consumers are permanently deleted.",
    confirmLabel: "Delete stream", danger: true, typeToConfirm: props.name });
  if (!ok) return;
  try {
    await invoke("nats/streamDelete", { stream: props.name });
    refreshList("streams");
    closeTab(`stream:${props.name}`);
  } catch (err) {
    actionError.value = err;
  }
}
</script>

<template>
  <ErrorBox :error="error" />
  <template v-if="detail">
    <PageHeader :title="name" :subtitle="`${detail.retention} · ${detail.storage} · ${num(detail.messages)} messages · ${bytes(detail.bytes)}`">
      <button type="button" class="dbx-btn" @click="openFiles('streams', name)">Browse as files</button>
      <button type="button" class="dbx-btn" @click="refresh">Refresh</button>
      <WriteButton icon="edit" @click="streamForm(detail.spec)">Edit</WriteButton>
      <WriteButton @click="purge">Purge</WriteButton>
      <WriteButton icon="trash" variant="danger" @click="remove">Delete</WriteButton>
    </PageHeader>
    <ErrorBox :error="actionError" />
    <SegmentedControl v-model="section"
      :options="[['messages', 'Messages'], ['consumers', `Consumers (${detail.consumers})`], ['info', 'Configuration']]" />
    <div class="tab-body">
      <StreamMessages v-if="section === 'messages'" :key="version" :stream="name" />
      <StreamConsumers v-else-if="section === 'consumers'" :key="version" :stream="name" />
      <PropList v-else :items="config(detail)" />
    </div>
  </template>
</template>
