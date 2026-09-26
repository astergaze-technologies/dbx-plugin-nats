import { invoke } from "../../api/bridge";
import type { StreamSpec } from "../../api/types";
import { formDialog, type FormField } from "../../stores/dialogs";
import { openStream } from "../../stores/navigation";
import { refreshNav } from "../../stores/workspace";

const fields = (spec: Partial<StreamSpec> = {}, editing = false): FormField[] => [
  { key: "name", label: "Name", value: spec.name, required: true, disabled: editing },
  { key: "subjects", label: "Subjects", value: (spec.subjects || []).join(", "), hint: "Comma separated; wildcards allowed (orders.>)" },
  { key: "description", label: "Description", value: spec.description },
  { key: "storage", label: "Storage", type: "select", value: spec.storage || "file", options: [["file", "File"], ["memory", "Memory"]], disabled: editing },
  { key: "retention", label: "Retention", type: "select", value: spec.retention || "limits", disabled: editing,
    options: [["limits", "Limits"], ["interest", "Interest"], ["workqueue", "Work queue"]] },
  { key: "discard", label: "When full, discard", type: "select", value: spec.discard || "old", options: [["old", "Old messages"], ["new", "New messages"]] },
  { key: "replicas", label: "Replicas", type: "number", value: spec.replicas || 1 },
  { key: "maxMsgs", label: "Max messages", type: "number", value: spec.maxMsgs! > 0 ? spec.maxMsgs : "", hint: "Empty = unlimited" },
  { key: "maxBytes", label: "Max bytes", type: "number", value: spec.maxBytes! > 0 ? spec.maxBytes : "", hint: "Empty = unlimited" },
  { key: "maxAgeSecs", label: "Max age (seconds)", type: "number", value: spec.maxAgeSecs || "", hint: "Empty = keep forever" },
];

// streamForm creates a stream, or edits one when existing is given.
export function streamForm(existing?: StreamSpec) {
  formDialog({
    title: existing ? `Edit stream ${existing.name}` : "Create stream",
    fields: fields(existing, !!existing),
    submitLabel: existing ? "Save" : "Create",
    async onSubmit(v) {
      const subjects = String(v.subjects).split(",").map((s) => s.trim()).filter(Boolean);
      const streamSpec = { ...existing, ...v, subjects } as StreamSpec;
      await invoke("nats/streamSave", { streamSpec, update: !!existing });
      refreshNav("streams");
      openStream(streamSpec.name, true);
    },
  });
}
