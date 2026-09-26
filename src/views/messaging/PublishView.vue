<script setup lang="ts">
import { ref } from "vue";
import { invoke } from "../../api/bridge";
import type { Reply } from "../../api/types";
import ErrorBox from "../../components/ErrorBox.vue";
import MessageCard from "../../components/MessageCard.vue";
import PageHeader from "../../components/PageHeader.vue";
import { parseHeaders } from "../../lib/format";

const subject = ref("");
const timeoutSecs = ref(5);
const headers = ref("");
const data = ref("");
const busy = ref(false);
const error = ref<unknown>(null);
const published = ref("");
const reply = ref<Reply>();

const params = () => ({ subject: subject.value.trim(), data: data.value, headers: parseHeaders(headers.value) });

async function run(action: () => Promise<void>) {
  busy.value = true;
  error.value = null;
  published.value = "";
  reply.value = undefined;
  try {
    await action();
  } catch (err) {
    error.value = err;
  } finally {
    busy.value = false;
  }
}

const publish = () => run(async () => {
  const p = params();
  await invoke("nats/publish", p);
  published.value = `Published to ${p.subject} at ${new Date().toLocaleTimeString()}.`;
});

const request = () => run(async () => {
  const secs = Math.min(60, Math.max(1, Number(timeoutSecs.value) || 5));
  reply.value = await invoke<Reply>("nats/request", { ...params(), timeoutMs: secs * 1000 }, secs * 1000 + 5000);
});
</script>

<template>
  <PageHeader title="Publish" subtitle="Send a message, or a request that waits for one reply." />
  <div class="form">
    <div class="row">
      <label class="field">
        <span>Subject</span>
        <input v-model="subject" class="dbx-input" placeholder="orders.created" spellcheck="false" @keydown.enter.prevent="publish" />
        <span class="hint">Wildcards are not allowed when publishing.</span>
      </label>
      <label class="field">
        <span>Request timeout (s)</span>
        <input v-model.number="timeoutSecs" class="dbx-input" type="number" min="1" max="60" />
      </label>
    </div>
    <label class="field">
      <span>Headers</span>
      <textarea v-model="headers" class="dbx-textarea mono" rows="2" placeholder="X-Trace-Id: 123" spellcheck="false" />
      <span class="hint">One Key: value per line (optional).</span>
    </label>
    <label class="field">
      <span>Payload</span>
      <textarea v-model="data" class="dbx-textarea mono" rows="8" placeholder='{"id": 1}' spellcheck="false" />
    </label>
    <div class="toolbar">
      <button type="button" class="dbx-btn dbx-btn--primary" :disabled="busy" @click="publish">Publish</button>
      <button type="button" class="dbx-btn" :disabled="busy" @click="request">Send request</button>
    </div>
    <div aria-live="polite">
      <ErrorBox :error="error" />
      <p v-if="published" class="ok">{{ published }}</p>
      <template v-if="reply">
        <h3 class="section-title">Reply in {{ reply.elapsedMs.toFixed(1) }} ms</h3>
        <MessageCard :message="reply" />
      </template>
    </div>
  </div>
</template>
