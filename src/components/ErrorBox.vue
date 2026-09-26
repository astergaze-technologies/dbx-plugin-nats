<script setup lang="ts">
import { errorText } from "../lib/format";

defineProps<{ error: unknown }>();
</script>

<template>
  <div v-if="error" class="error" role="alert">{{ errorText(error) }}</div>
</template>
