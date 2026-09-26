<script setup lang="ts">
import { icons, type IconName } from "../lib/icons";

withDefaults(defineProps<{ name: IconName; size?: number }>(), { size: 16 });
</script>

<template>
  <svg :width="size" :height="size" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
    stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path :d="icons[name]" />
  </svg>
</template>
