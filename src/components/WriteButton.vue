<script setup lang="ts">
import { session } from "../api/bridge";
import type { IconName } from "../lib/icons";
import AppIcon from "./AppIcon.vue";

// A button for mutating actions: disabled, with an explanation, on read-only connections.
defineProps<{ icon?: IconName; variant?: "primary" | "danger" }>();
defineEmits<{ click: [] }>();
</script>

<template>
  <button type="button" class="dbx-btn"
    :class="{ 'dbx-btn--primary': variant === 'primary', 'dbx-btn--danger-ghost': variant === 'danger' }"
    :disabled="session.readOnly" :title="session.readOnly ? 'Read-only connection' : undefined" @click="$emit('click')">
    <AppIcon v-if="icon" :name="icon" :size="14" />
    <slot />
  </button>
</template>
