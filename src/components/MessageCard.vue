<script setup lang="ts">
import type { MessageLike } from "../api/types";
import { bytes, headerLine, payloadText, time } from "../lib/format";

defineProps<{ message: MessageLike; at?: string }>();
</script>

<template>
  <article class="msg">
    <header class="msg-head">
      <strong v-if="message.seq != null">#{{ message.seq }}</strong>
      <code v-if="message.subject">{{ message.subject }}</code>
      <slot />
      <span class="hint">{{ time(at ?? message.time) }}</span>
      <span class="hint">{{ bytes(message.size) }}</span>
    </header>
    <div v-if="headerLine(message.headers)" class="hint mono">{{ headerLine(message.headers) }}</div>
    <pre>{{ payloadText(message) }}</pre>
  </article>
</template>
