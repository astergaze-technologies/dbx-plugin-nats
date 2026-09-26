<script setup lang="ts">
import { ref } from "vue";
import { invoke } from "../../api/bridge";
import type { StoredMessage } from "../../api/types";
import ErrorBox from "../../components/ErrorBox.vue";
import IconButton from "../../components/IconButton.vue";
import MessageCard from "../../components/MessageCard.vue";
import { confirmAction } from "../../stores/dialogs";

const props = defineProps<{ stream: string }>();
const messages = ref<StoredMessage[]>([]);
const nextBefore = ref(0);
const loading = ref(false);
const loaded = ref(false);
const error = ref<unknown>(null);

// Pages newest first; before=0 starts at the last message.
async function more(before = 0) {
  loading.value = true;
  error.value = null;
  try {
    const page = await invoke<{ messages: StoredMessage[]; nextBefore: number }>(
      "nats/streamMessages", { stream: props.stream, before, limit: 50 }, 30000);
    messages.value.push(...page.messages);
    nextBefore.value = page.nextBefore;
    loaded.value = true;
  } catch (err) {
    error.value = err;
  } finally {
    loading.value = false;
  }
}

async function remove(m: StoredMessage) {
  const ok = await confirmAction({ title: "Delete message?", message: `Message #${m.seq} will be removed from ${props.stream}.`, confirmLabel: "Delete", danger: true });
  if (!ok) return;
  try {
    await invoke("nats/messageDelete", { stream: props.stream, seq: m.seq });
    messages.value = messages.value.filter((x) => x.seq !== m.seq);
  } catch (err) {
    error.value = err;
  }
}

more();
</script>

<template>
  <div class="msg-list">
    <ErrorBox :error="error" />
    <div v-if="loaded && !messages.length" class="empty">This stream has no messages.</div>
    <MessageCard v-for="m in messages" :key="m.seq" :message="m">
      <IconButton icon="trash" :label="`Delete message ${m.seq}`" danger write @click="remove(m)" />
    </MessageCard>
    <div v-if="loading" class="empty">Loading messages…</div>
    <div v-else-if="nextBefore" class="more">
      <button type="button" class="dbx-btn" @click="more(nextBefore)">Load older</button>
    </div>
  </div>
</template>
