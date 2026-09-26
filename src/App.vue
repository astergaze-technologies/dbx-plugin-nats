<script setup lang="ts">
import AppIcon from "./components/AppIcon.vue";
import DialogHost from "./components/DialogHost.vue";
import { session } from "./api/bridge";
import type { IconName } from "./lib/icons";
import { closeTab, openTab, workspace } from "./stores/workspace";
import BucketsView from "./views/kv/BucketsView.vue";
import PublishView from "./views/messaging/PublishView.vue";
import SubscribeView from "./views/messaging/SubscribeView.vue";
import StoresView from "./views/objects/StoresView.vue";
import ServicesView from "./views/services/ServicesView.vue";
import StreamsView from "./views/streams/StreamsView.vue";
import ServerView from "./views/server/ServerView.vue";
import { computed, type Component } from "vue";

defineProps<{ inDbx: boolean }>();

// Sections open (or focus) one tab each; their pages list the streams, buckets and stores to open.
const sections: { id: string; label: string; icon: IconName; component: Component }[] = [
  { id: "server", label: "Server", icon: "server", component: ServerView },
  { id: "streams", label: "Streams", icon: "streams", component: StreamsView },
  { id: "kv", label: "Key-Value", icon: "kv", component: BucketsView },
  { id: "objects", label: "Object Store", icon: "objects", component: StoresView },
  { id: "services", label: "Services", icon: "services", component: ServicesView },
  { id: "publish", label: "Publish", icon: "send", component: PublishView },
  { id: "subscribe", label: "Subscribe", icon: "radio", component: SubscribeView },
];

const openSection = (s: (typeof sections)[number]) =>
  openTab({ id: s.id, title: s.label, icon: s.icon, component: s.component, section: true });

const itemTabs = computed(() => workspace.tabs.filter((t) => !t.section));

// A section is highlighted while its page or one of its items (stream:ORDERS, kv:config…) is active.
const isActive = (id: string) => workspace.active === id || workspace.active.startsWith(`${id === "streams" ? "stream" : id}:`);
</script>

<template>
  <div v-if="!inDbx" class="empty">This page must be opened inside DBX.</div>
  <div v-else-if="!session.connectionId" class="empty">Open a saved NATS connection from the sidebar to use this workbench.</div>
  <main v-else :key="session.connectionId" class="workspace">
    <nav class="section-bar" aria-label="NATS sections">
      <button v-for="s in sections" :key="s.id" type="button" class="section" :aria-current="isActive(s.id) ? 'page' : undefined"
        @click="openSection(s)">
        <AppIcon :name="s.icon" :size="14" />
        <span>{{ s.label }}</span>
      </button>
    </nav>
    <div v-if="itemTabs.length" class="tab-bar" role="tablist" aria-label="Open items">
      <div v-for="tab in itemTabs" :key="tab.key" class="tab-wrap">
        <button type="button" role="tab" class="tab" :aria-selected="workspace.active === tab.id"
          :tabindex="workspace.active === tab.id ? 0 : -1" @click="workspace.active = tab.id">
          <AppIcon :name="tab.icon" :size="14" />
          <span class="tab-title">{{ tab.title }}</span>
        </button>
        <button v-if="tab.closable" type="button" class="tab-close" :aria-label="`Close ${tab.title}`" @click="closeTab(tab.id)">
          <AppIcon name="close" :size="12" />
        </button>
      </div>
    </div>
    <div class="tab-body-host">
      <!-- panels stay mounted while hidden so live views keep their state -->
      <section v-for="tab in workspace.tabs" :key="tab.key" :hidden="workspace.active !== tab.id"
        class="tab-panel" role="tabpanel" :aria-label="tab.title">
        <component :is="tab.component" v-bind="tab.props" />
      </section>
    </div>
  </main>
  <DialogHost />
</template>
