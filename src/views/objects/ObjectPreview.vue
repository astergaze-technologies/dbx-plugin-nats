<script setup lang="ts">
import { computed } from "vue";
import { invoke } from "../../api/bridge";
import type { ObjectInfo } from "../../api/types";
import ErrorBox from "../../components/ErrorBox.vue";
import PropList from "../../components/PropList.vue";
import { bytes, time } from "../../lib/format";
import { useLoader } from "../../lib/useLoader";

const PREVIEW_CHARS = 65536;
const props = defineProps<{ store: string; name: string }>();
const { data: obj, error } = useLoader(() =>
  invoke<ObjectInfo & { data: string }>("nats/objectGet", { store: props.store, name: props.name }, 60000));

const text = computed(() => {
  if (!obj.value) return "";
  const raw = new TextDecoder("utf-8", { fatal: false }).decode(Uint8Array.from(atob(obj.value.data || ""), (c) => c.charCodeAt(0)));
  if (raw.slice(0, 4096).includes("�") || /[\x00-\x08\x0E-\x1F]/.test(raw.slice(0, 4096))) return `[binary, ${bytes(obj.value.size)}]`;
  return raw.length > PREVIEW_CHARS ? `${raw.slice(0, PREVIEW_CHARS)}\n… truncated` : raw;
});
</script>

<template>
  <ErrorBox :error="error" />
  <div v-if="!obj && !error" class="empty">Loading object…</div>
  <template v-if="obj">
    <PropList :items="[['Name', obj.name], ['Size', bytes(obj.size)], ['Chunks', obj.chunks], ['Digest', obj.digest], ['Modified', time(obj.modified)]]" />
    <pre>{{ text }}</pre>
  </template>
</template>
