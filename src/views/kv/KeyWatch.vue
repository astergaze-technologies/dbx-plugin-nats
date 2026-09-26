<script setup lang="ts">
import { onUnmounted, ref } from "vue";
import { invoke, on } from "../../api/bridge";
import type { KVChange } from "../../api/types";
import ErrorBox from "../../components/ErrorBox.vue";
import MessageCard from "../../components/MessageCard.vue";

const MAX_CHANGES = 200;
const props = defineProps<{ bucket: string }>();
const changes = ref<(KVChange & { n: number })[]>([]);
const feedId = ref("");
const error = ref<unknown>(null);
let n = 0;

const off = on<KVChange>("nats/kvChange", (c) => {
  if (c.feedId !== feedId.value) return;
  changes.value.unshift({ ...c, n: n++ });
  if (changes.value.length > MAX_CHANGES) changes.value.length = MAX_CHANGES;
});

async function stop() {
  if (feedId.value) await invoke("nats/feedStop", { feedId: feedId.value }).catch(() => {});
  feedId.value = "";
}

async function toggle() {
  if (feedId.value) return stop();
  error.value = null;
  changes.value = [];
  try {
    feedId.value = (await invoke<{ feedId: string }>("nats/kvWatch", { bucket: props.bucket, pattern: ">" })).feedId;
  } catch (err) {
    error.value = err;
  }
}

onUnmounted(() => {
  off();
  stop();
});
</script>

<template>
  <div>
    <div class="toolbar">
      <p class="hint grow">Live changes to any key in this bucket, newest first.</p>
      <button type="button" class="dbx-btn" @click="toggle">{{ feedId ? "Stop watching" : "Watch changes" }}</button>
    </div>
    <ErrorBox :error="error" />
    <div class="msg-list">
      <MessageCard v-for="c in changes" :key="c.n" :message="{ ...c, subject: c.key }" :at="c.created">
        <span class="dbx-badge">{{ c.operation }}</span>
        <span class="dbx-badge">rev {{ c.revision }}</span>
      </MessageCard>
    </div>
  </div>
</template>
