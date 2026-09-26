<script setup lang="ts">
import { computed, onUnmounted, reactive, ref, shallowRef, triggerRef } from "vue";
import { invoke, on } from "../../api/bridge";
import type { LiveMessage } from "../../api/types";
import ErrorBox from "../../components/ErrorBox.vue";
import MessageCard from "../../components/MessageCard.vue";
import PageHeader from "../../components/PageHeader.vue";
import { num } from "../../lib/format";

const MAX_LIVE = 500;

interface Feed { subject: string; queue: string; count: number }
type Row = LiveMessage & { n: number };

const subject = ref("");
const queue = ref("");
const feeds = reactive(new Map<string, Feed>());
const live = shallowRef<Row[]>([]); // shallow: up to 500 rows re-rendered once per frame
const dropped = ref(0);
const paused = ref(false);
const error = ref<unknown>(null);
let n = 0;
let pending: Row[] = [];
let frame = 0;

const status = computed(() =>
  `${num(live.value.length)} shown (last ${MAX_LIVE})${dropped.value ? ` · ${num(dropped.value)} dropped by the 100 msg/s limit` : ""}${paused.value ? " · paused" : ""}`);

function flush() {
  frame = 0;
  live.value = [...pending.reverse(), ...live.value].slice(0, MAX_LIVE);
  pending = [];
}

const offMessage = on<LiveMessage>("nats/message", (m) => {
  const feed = feeds.get(m.feedId);
  if (!feed) return;
  feed.count++;
  dropped.value += m.dropped || 0;
  if (paused.value) return;
  pending.push({ ...m, n: n++ });
  frame ||= requestAnimationFrame(flush);
});
const offClosed = on<{ feedId: string }>("nats/feedClosed", (f) => feeds.delete(f.feedId));

async function add() {
  const s = subject.value.trim();
  if (!s) return;
  error.value = null;
  try {
    const { feedId } = await invoke<{ feedId: string }>("nats/subscribe", { subject: s, queue: queue.value.trim() });
    feeds.set(feedId, { subject: s, queue: queue.value.trim(), count: 0 });
    subject.value = "";
  } catch (err) {
    error.value = err;
  }
}

async function stop(id: string) {
  await invoke("nats/feedStop", { feedId: id }).catch(() => {});
  feeds.delete(id);
}

function clear() {
  live.value = [];
  dropped.value = 0;
  triggerRef(live);
}

onUnmounted(() => {
  offMessage();
  offClosed();
  cancelAnimationFrame(frame);
  for (const id of feeds.keys()) invoke("nats/feedStop", { feedId: id }).catch(() => {});
});
</script>

<template>
  <PageHeader title="Subscribe" subtitle="Watch subjects live. Each subscription forwards up to 100 messages per second." />
  <div class="toolbar">
    <input v-model="subject" class="dbx-input grow" placeholder="orders.> or events.*" spellcheck="false" aria-label="Subject" @keydown.enter.prevent="add" />
    <input v-model="queue" class="dbx-input queue" placeholder="queue group (optional)" spellcheck="false" aria-label="Queue group" @keydown.enter.prevent="add" />
    <button type="button" class="dbx-btn dbx-btn--primary" @click="add">Subscribe</button>
  </div>
  <ErrorBox :error="error" />
  <div class="chips">
    <span v-if="!feeds.size" class="hint">No active subscriptions. Wildcards like orders.* and events.&gt; work.</span>
    <span v-for="[id, f] in feeds" :key="id" class="chip">
      <code>{{ f.queue ? `${f.subject} (${f.queue})` : f.subject }}</code>
      <span class="hint">{{ num(f.count) }}</span>
      <button type="button" :aria-label="`Stop ${f.subject}`" title="Stop" @click="stop(id)">×</button>
    </span>
  </div>
  <div class="toolbar">
    <span class="hint grow" aria-live="polite">{{ status }}</span>
    <button type="button" class="dbx-btn" @click="paused = !paused">{{ paused ? "Resume" : "Pause" }}</button>
    <button type="button" class="dbx-btn" @click="clear">Clear</button>
  </div>
  <div class="msg-list">
    <div v-if="!live.length" class="empty">Messages appear here as they arrive.</div>
    <MessageCard v-for="m in live" :key="m.n" :message="m" :at="m.receivedAt">
      <span v-if="m.reply" class="hint mono">reply → {{ m.reply }}</span>
    </MessageCard>
  </div>
</template>
