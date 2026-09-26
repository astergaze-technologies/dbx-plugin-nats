<script setup lang="ts">
import { nextTick, reactive, ref, watch } from "vue";
import { activeDialog, type FormValues } from "../stores/dialogs";
import ErrorBox from "./ErrorBox.vue";

const el = ref<HTMLDialogElement>();
const typed = ref("");
const values = reactive<FormValues>({});
const error = ref<unknown>(null);
const busy = ref(false);

watch(activeDialog, async (d) => {
  if (!d) return;
  typed.value = "";
  error.value = null;
  if (d.kind === "form") {
    for (const key of Object.keys(values)) delete values[key];
    for (const f of d.fields) values[f.key] = f.value ?? "";
  }
  await nextTick();
  el.value?.showModal();
  el.value?.querySelector<HTMLElement>("input, select, textarea, .dbx-btn")?.focus();
});

function close(ok = false) {
  const d = activeDialog.value;
  if (d?.kind === "confirm") d.resolve(ok);
  activeDialog.value = null;
  el.value?.close();
}

async function submit() {
  const d = activeDialog.value;
  if (d?.kind !== "form") return;
  const out: FormValues = {};
  for (const f of d.fields) out[f.key] = f.type === "number" ? Number(values[f.key] || 0) : values[f.key];
  busy.value = true;
  error.value = null;
  try {
    await d.onSubmit(out);
    close();
  } catch (err) {
    error.value = err;
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <dialog ref="el" class="modal" :aria-label="activeDialog?.title" @close="activeDialog && close()">
    <template v-if="activeDialog">
      <h2 class="modal-title">{{ activeDialog.title }}</h2>

      <div v-if="activeDialog.kind === 'confirm'" class="modal-body">
        <p>{{ activeDialog.message }}</p>
        <label v-if="activeDialog.typeToConfirm" class="field">
          <span>Type {{ activeDialog.typeToConfirm }} to confirm</span>
          <input v-model="typed" class="dbx-input" spellcheck="false" :aria-label="`Type ${activeDialog.typeToConfirm} to confirm`" />
        </label>
      </div>

      <div v-else class="modal-body form-grid" @keydown.enter.exact="($event.target as HTMLElement).tagName === 'INPUT' && submit()">
        <label v-for="f in activeDialog.fields" :key="f.key" class="field" :for="`f-${f.key}`">
          <span>{{ f.label }}</span>
          <select v-if="f.type === 'select'" :id="`f-${f.key}`" v-model="values[f.key]" class="dbx-select" :disabled="f.disabled">
            <option v-for="[value, label] in f.options" :key="value" :value="value">{{ label }}</option>
          </select>
          <textarea v-else-if="f.type === 'textarea'" :id="`f-${f.key}`" v-model="values[f.key]" class="dbx-textarea mono"
            rows="4" spellcheck="false" :disabled="f.disabled" />
          <input v-else :id="`f-${f.key}`" v-model="values[f.key]" class="dbx-input" :type="f.type || 'text'"
            :required="f.required" :disabled="f.disabled" spellcheck="false" />
          <span v-if="f.hint" class="hint">{{ f.hint }}</span>
        </label>
        <ErrorBox :error="error" />
      </div>

      <div class="modal-actions">
        <button type="button" class="dbx-btn" @click="close()">Cancel</button>
        <button v-if="activeDialog.kind === 'confirm'" type="button" class="dbx-btn"
          :class="activeDialog.danger ? 'dbx-btn--danger' : 'dbx-btn--primary'"
          :disabled="!!activeDialog.typeToConfirm && typed !== activeDialog.typeToConfirm" @click="close(true)">
          {{ activeDialog.confirmLabel ?? "Confirm" }}
        </button>
        <button v-else type="button" class="dbx-btn dbx-btn--primary" :disabled="busy" @click="submit">
          {{ activeDialog.submitLabel ?? "Save" }}
        </button>
      </div>
    </template>
  </dialog>
</template>
