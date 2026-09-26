<script setup lang="ts">
defineProps<{ items: [label: string, value: unknown][] }>();
</script>

<template>
  <dl class="props card">
    <template v-for="[label, value] in items" :key="label">
      <dt>{{ label }}</dt>
      <dd>{{ value }}</dd>
    </template>
  </dl>
</template>
