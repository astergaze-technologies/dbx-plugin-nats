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
}

type TabInput = Omit<Tab, "key" | "closable" | "props"> & { props?: Record<string, unknown>; closable?: boolean };

let nextKey = 0;

export const workspace = reactive({
  tabs: [] as Tab[],
  active: "",
  navVersion: {} as Record<string, number>,
});

export function openTab(tab: TabInput, reopen = false) {
  if (reopen) closeTab(tab.id);
  if (!workspace.tabs.some((t) => t.id === tab.id)) {
    workspace.tabs.push({ closable: true, props: {}, ...tab, key: nextKey++, component: markRaw(tab.component) });
  }
  workspace.active = tab.id;
}

export function closeTab(id: string) {
  const i = workspace.tabs.findIndex((t) => t.id === id && t.closable);
  if (i < 0) return;
  workspace.tabs.splice(i, 1);
  if (workspace.active === id) workspace.active = workspace.tabs.at(-1)?.id ?? "";
}

export function resetTabs() {
  workspace.tabs = [];
  workspace.active = "";
}

// refreshNav reloads one navigator section, e.g. after creating a stream.
export function refreshNav(section: string) {
  workspace.navVersion[section] = (workspace.navVersion[section] || 0) + 1;
}
