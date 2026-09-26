<script setup lang="ts">
import { ref } from "vue";
import { invoke, openFiles } from "../../api/bridge";
import ErrorBox from "../../components/ErrorBox.vue";
import PageHeader from "../../components/PageHeader.vue";
import SegmentedControl from "../../components/SegmentedControl.vue";
import WriteButton from "../../components/WriteButton.vue";
import { confirmAction, formDialog } from "../../stores/dialogs";
import { closeTab, refreshList } from "../../stores/workspace";
import KeyBrowser from "./KeyBrowser.vue";
import KeyWatch from "./KeyWatch.vue";

const props = defineProps<{ bucket: string }>();
const section = ref("keys");
const browser = ref<InstanceType<typeof KeyBrowser>>();
const error = ref<unknown>(null);

function putKey() {
  formDialog({
    title: "Put key",
    fields: [{ key: "key", label: "Key", required: true }, { key: "value", label: "Value", type: "textarea" }],
    async onSubmit(v) {
      await invoke("nats/kvPut", { bucket: props.bucket, key: v.key, data: v.value });
      section.value = "keys";
      await browser.value?.reload(String(v.key));
    },
  });
}

async function removeBucket() {
  const ok = await confirmAction({ title: `Delete bucket ${props.bucket}?`, message: "Every key and its history is permanently deleted.",
    confirmLabel: "Delete bucket", danger: true, typeToConfirm: props.bucket });
  if (!ok) return;
  try {
    await invoke("nats/kvBucketDelete", { bucket: props.bucket });
    refreshList("kv");
    closeTab(`kv:${props.bucket}`);
  } catch (err) {
    error.value = err;
  }
}
</script>

<template>
  <PageHeader :title="bucket" subtitle="Key-value bucket">
    <button type="button" class="dbx-btn" @click="openFiles('kv', bucket)">Browse as files</button>
    <button type="button" class="dbx-btn" @click="browser?.reload()">Refresh</button>
    <WriteButton icon="plus" variant="primary" @click="putKey">Put key</WriteButton>
    <WriteButton icon="trash" variant="danger" @click="removeBucket">Delete bucket</WriteButton>
  </PageHeader>
  <ErrorBox :error="error" />
  <SegmentedControl v-model="section" :options="[['keys', 'Keys'], ['watch', 'Watch']]" />
  <!-- v-show keeps an active watch running while the keys are shown -->
  <KeyBrowser v-show="section === 'keys'" ref="browser" :bucket="bucket" />
  <KeyWatch v-show="section === 'watch'" :bucket="bucket" />
</template>
