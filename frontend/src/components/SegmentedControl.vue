<script setup lang="ts">
defineProps<{ options: [id: string, label: string][] }>();
const model = defineModel<string>({ required: true });
</script>

<template>
  <div class="segmented" role="tablist">
    <button v-for="[id, label] in options" :key="id" type="button" role="tab" :aria-selected="model === id"
      @click="model = id">{{ label }}</button>
  </div>
</template>
