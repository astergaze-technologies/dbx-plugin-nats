<script lang="ts">
import type { IconName } from "../lib/icons";

export interface NavItem {
  id: string;
  label: string;
  meta?: string;
  open(): void;
}

export interface NavSection {
  id: string;
  label: string;
  icon: IconName;
  open(): void;
  load?(): Promise<NavItem[]>; // sections with children expand like the DBX sidebar
}
</script>

<script setup lang="ts">
import { reactive, ref, watch } from "vue";
import { errorText } from "../lib/format";
import { workspace } from "../stores/workspace";
import AppIcon from "./AppIcon.vue";

const props = defineProps<{ sections: NavSection[] }>();

interface NodeState { expanded: boolean; loaded: boolean; loading: boolean; items: NavItem[]; error: string }
const nodes = reactive<Record<string, NodeState>>(
  Object.fromEntries(props.sections.map((s) => [s.id, { expanded: false, loaded: false, loading: false, items: [], error: "" }])),
);
const query = ref("");

async function load(section: NavSection) {
  const node = nodes[section.id];
  node.loading = true;
  node.error = "";
  try {
    node.items = await section.load!();
    node.loaded = true;
  } catch (err) {
    node.error = errorText(err);
  } finally {
    node.loading = false;
  }
}

function toggle(section: NavSection, open = !nodes[section.id].expanded) {
  nodes[section.id].expanded = open;
  if (open && !nodes[section.id].loaded) load(section);
}

const matches = (section: NavSection) =>
  nodes[section.id].items.filter((it) => it.label.toLowerCase().includes(query.value.toLowerCase()));

watch(query, (q) => {
  if (q) for (const s of props.sections) if (s.load) toggle(s, true);
});

watch(() => ({ ...workspace.navVersion }), (now, before) => {
  for (const s of props.sections) {
    const node = nodes[s.id];
    if (s.load && now[s.id] !== before[s.id] && (node.loaded || node.expanded)) load(s);
  }
});
</script>

<template>
  <nav class="navigator" aria-label="NATS navigator">
    <input v-model="query" class="dbx-input nav-filter" type="search" placeholder="Filter…" aria-label="Filter navigator" />
    <ul class="nav-tree" role="tree" aria-label="NATS objects">
      <li v-for="s in sections" :key="s.id" role="treeitem" :aria-expanded="s.load ? nodes[s.id].expanded : undefined">
        <div class="nav-row">
          <button v-if="s.load" type="button" class="nav-toggle" :class="{ open: nodes[s.id].expanded }"
            :aria-label="`Expand ${s.label}`" @click="toggle(s)">
            <AppIcon name="chevron" :size="14" />
          </button>
          <span v-else class="nav-toggle" />
          <button type="button" class="nav-label" @click="s.open()">
            <AppIcon :name="s.icon" />
            <span>{{ s.label }}</span>
            <span class="nav-count">{{ nodes[s.id].loaded ? nodes[s.id].items.length : "" }}</span>
          </button>
        </div>
        <ul v-if="nodes[s.id].expanded" role="group">
          <li v-if="nodes[s.id].error" class="nav-hint error-text">{{ nodes[s.id].error }}</li>
          <li v-else-if="!nodes[s.id].loaded" class="nav-hint">Loading…</li>
          <li v-else-if="!matches(s).length" class="nav-hint">{{ query ? "No matches" : "Empty" }}</li>
          <template v-else>
            <li v-for="it in matches(s)" :key="it.id" role="treeitem">
              <button type="button" class="nav-leaf" :title="it.label" @click="it.open()">
                <AppIcon :name="s.icon" :size="14" />
                <span class="nav-leaf-label">{{ it.label }}</span>
                <span v-if="it.meta" class="nav-count">{{ it.meta }}</span>
              </button>
            </li>
          </template>
        </ul>
      </li>
    </ul>
  </nav>
</template>
