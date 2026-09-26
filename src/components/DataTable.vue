<script lang="ts">
export interface Column<T> {
  key: string;
  label: string;
  num?: boolean;
  wrap?: boolean;
  value?: (row: T) => unknown; // sort/filter value when it differs from row[key]
}
</script>

<script setup lang="ts" generic="T extends object">
import { computed, ref } from "vue";

// Cells render row[key] unless a slot named after the column key is given; #actions adds a trailing column.
const props = withDefaults(defineProps<{
  columns: Column<T>[];
  rows: T[];
  label: string;
  emptyText?: string;
  filterable?: boolean;
  onOpen?: (row: T) => void;
}>(), { emptyText: "No rows", filterable: true });

const query = ref("");
const sort = ref({ key: props.columns[0].key, desc: false });

const valueOf = (col: Column<T>, row: T) => (col.value ? col.value(row) : (row as Record<string, unknown>)[col.key]);

const visible = computed(() => {
  const q = query.value.toLowerCase();
  const col = props.columns.find((c) => c.key === sort.value.key)!;
  return props.rows
    .filter((row) => !q || props.columns.some((c) => String(valueOf(c, row) ?? "").toLowerCase().includes(q)))
    .sort((a, b) => {
      const [x, y] = [valueOf(col, a), valueOf(col, b)];
      const cmp = typeof x === "number" && typeof y === "number" ? x - y : String(x ?? "").localeCompare(String(y ?? ""));
      return sort.value.desc ? -cmp : cmp;
    });
});

function sortBy(col: Column<T>) {
  const on = sort.value.key === col.key;
  sort.value = { key: col.key, desc: on ? !sort.value.desc : !!col.num };
}

const ariaSort = (col: Column<T>) =>
  sort.value.key !== col.key ? "none" : sort.value.desc ? "descending" : "ascending";
</script>

<template>
  <div class="table-block">
    <div v-if="filterable" class="toolbar">
      <input v-model="query" class="dbx-input" type="search" placeholder="Filter…" :aria-label="`Filter ${label}`" />
      <span class="hint">{{ query ? `${visible.length} of ${rows.length}` : rows.length }}</span>
    </div>
    <div class="scroll">
      <table class="dbx-table">
        <thead>
          <tr>
            <th v-for="col in columns" :key="col.key" scope="col" :class="{ num: col.num }" :aria-sort="ariaSort(col)">
              <button type="button" class="th-sort" @click="sortBy(col)">
                {{ col.label }}{{ sort.key === col.key ? (sort.desc ? " ↓" : " ↑") : "" }}
              </button>
            </th>
            <th v-if="$slots.actions" scope="col"><span class="sr-only">Actions</span></th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!visible.length">
            <td class="empty" :colspan="columns.length + ($slots.actions ? 1 : 0)">{{ query ? "No matches" : emptyText }}</td>
          </tr>
          <tr v-for="(row, i) in visible" :key="i" :tabindex="onOpen ? 0 : undefined" :class="{ clickable: onOpen }"
            @click="onOpen?.(row)" @keydown.enter="onOpen?.(row)">
            <td v-for="col in columns" :key="col.key" :class="{ num: col.num, wrap: col.wrap }">
              <slot :name="col.key" :row="row">{{ valueOf(col, row) }}</slot>
            </td>
            <td v-if="$slots.actions" class="row-actions" @click.stop @keydown.enter.stop>
              <slot name="actions" :row="row" />
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
