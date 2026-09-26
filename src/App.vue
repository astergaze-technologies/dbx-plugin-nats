<script setup lang="ts">
import { ref } from "vue";
import AppIcon from "./components/AppIcon.vue";
import DialogHost from "./components/DialogHost.vue";
import NavigatorTree, { type NavSection } from "./components/NavigatorTree.vue";
import { invoke, session } from "./api/bridge";
import type { Bucket, ObjectStore, StreamSummary } from "./api/types";
import { openBucket, openStore, openStream } from "./stores/navigation";
import { closeTab, openTab, workspace } from "./stores/workspace";
import BucketsView from "./views/kv/BucketsView.vue";
import PublishView from "./views/messaging/PublishView.vue";
import SubscribeView from "./views/messaging/SubscribeView.vue";
import StoresView from "./views/objects/StoresView.vue";
import ServicesView from "./views/services/ServicesView.vue";
import StreamsView from "./views/streams/StreamsView.vue";

defineProps<{ inDbx: boolean }>();

const navOpen = ref(true);

const count = (n: number) => (n ? n.toLocaleString() : "");
const byLabel = <T extends { label: string }>(items: T[]) => items.sort((a, b) => a.label.localeCompare(b.label));

const sections: NavSection[] = [
  { id: "server", label: "Server", icon: "server", open: () => (workspace.active = "server") },
  {
    id: "streams", label: "Streams", icon: "streams",
    open: () => openTab({ id: "streams", title: "Streams", icon: "streams", component: StreamsView }),
    load: async () => byLabel((await invoke<{ streams: StreamSummary[] }>("nats/streams")).streams
      .filter((s) => !s.system)
      .map((s) => ({ id: s.name, label: s.name, meta: count(s.messages), open: () => openStream(s.name) }))),
  },
  {
    id: "kv", label: "Key-Value", icon: "kv",
    open: () => openTab({ id: "kv", title: "Key-Value", icon: "kv", component: BucketsView }),
    load: async () => byLabel((await invoke<{ buckets: Bucket[] }>("nats/kvBuckets")).buckets
      .map((b) => ({ id: b.bucket, label: b.bucket, meta: count(b.values), open: () => openBucket(b.bucket) }))),
  },
  {
    id: "objects", label: "Object Store", icon: "objects",
    open: () => openTab({ id: "objects", title: "Object Store", icon: "objects", component: StoresView }),
    load: async () => byLabel((await invoke<{ stores: ObjectStore[] }>("nats/objectStores")).stores
      .map((s) => ({ id: s.store, label: s.store, open: () => openStore(s.store) }))),
  },
  { id: "services", label: "Services", icon: "services", open: () => openTab({ id: "services", title: "Services", icon: "services", component: ServicesView }) },
  { id: "publish", label: "Publish", icon: "send", open: () => openTab({ id: "publish", title: "Publish", icon: "send", component: PublishView }) },
  { id: "subscribe", label: "Subscribe", icon: "radio", open: () => openTab({ id: "subscribe", title: "Subscribe", icon: "radio", component: SubscribeView }) },
];
</script>

<template>
  <div v-if="!inDbx" class="empty">This page must be opened inside DBX.</div>
  <div v-else-if="!session.connectionId" class="empty">Open a saved NATS connection from the sidebar to use this workbench.</div>
  <div v-else :key="session.connectionId" class="layout" :class="{ 'nav-hidden': !navOpen }">
    <NavigatorTree v-show="navOpen" id="nats-navigator" :sections="sections" />
    <main class="workspace">
      <div class="tab-bar" role="tablist">
        <button type="button" class="nav-collapse" :aria-label="navOpen ? 'Hide navigator' : 'Show navigator'"
          :title="navOpen ? 'Hide navigator' : 'Show navigator'" :aria-expanded="navOpen" aria-controls="nats-navigator" @click="navOpen = !navOpen">
          <AppIcon :name="navOpen ? 'panelClose' : 'panelOpen'" />
        </button>
        <div v-for="tab in workspace.tabs" :key="tab.key" class="tab-wrap">
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
  </div>
  <DialogHost />
</template>
