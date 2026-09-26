import { markRaw, reactive, type Component } from "vue";
import type { IconName } from "../lib/icons";

export interface Tab {
  id: string;
  key: number; // changes when a tab is reopened so its view remounts
  title: string;
  icon: IconName;
  component: Component;
  props: Record<string, unknown>;
  closable: boolean;
  section: boolean; // section pages are switched from the section bar, not shown as tabs
}

type TabInput = Omit<Tab, "key" | "closable" | "props" | "section"> & { props?: Record<string, unknown>; closable?: boolean; section?: boolean };

let nextKey = 0;

export const workspace = reactive({
  tabs: [] as Tab[],
  active: "",
  listVersion: {} as Record<string, number>,
});

export function openTab(tab: TabInput, reopen = false) {
  if (reopen) closeTab(tab.id);
  if (!workspace.tabs.some((t) => t.id === tab.id)) {
    workspace.tabs.push({ closable: true, section: false, props: {}, ...tab, key: nextKey++, component: markRaw(tab.component) });
  }
  workspace.active = tab.id;
}

export function closeTab(id: string) {
  const i = workspace.tabs.findIndex((t) => t.id === id && t.closable);
  if (i < 0) return;
  workspace.tabs.splice(i, 1);
  if (workspace.active === id) workspace.active = workspace.tabs.findLast((t) => !t.section)?.id ?? "server";
}

export function resetTabs() {
  workspace.tabs = [];
  workspace.active = "";
}

// refreshList tells an open list page (streams, kv, objects) to reload, e.g. after a delete.
export function refreshList(section: string) {
  workspace.listVersion[section] = (workspace.listVersion[section] || 0) + 1;
}
