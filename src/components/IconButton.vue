<script setup lang="ts">
import { session } from "../api/bridge";
import type { IconName } from "../lib/icons";
import AppIcon from "./AppIcon.vue";

defineProps<{ icon: IconName; label: string; danger?: boolean; write?: boolean }>();
defineEmits<{ click: [] }>();
</script>

<template>
  <button type="button" class="icon-btn" :class="{ danger }" :aria-label="label" :title="label"
    :disabled="write && session.readOnly" @click="$emit('click')">
    <AppIcon :name="icon" :size="14" />
  </button>
</template>
