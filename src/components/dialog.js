import { button, errorBox, h, mount } from "../dom.js";

function modal(title, content, footer) {
  const dialog = h("dialog", { class: "modal", "aria-label": title },
    h("h2", { class: "modal-title" }, title), content, h("div", { class: "modal-actions" }, footer));
  document.body.append(dialog);
  dialog.addEventListener("close", () => dialog.remove());
  dialog.showModal();
  return dialog;
}

// confirmAction resolves true when confirmed. typeToConfirm forces typing a name for destructive actions.
export function confirmAction({ title, message, confirmLabel = "Confirm", danger = false, typeToConfirm }) {
  return new Promise((resolve) => {
    const input = typeToConfirm && h("input", { class: "dbx-input", "aria-label": `Type ${typeToConfirm} to confirm`, spellcheck: "false" });
    const ok = button(confirmLabel, () => { resolve(true); dialog.close(); }, { variant: danger ? "dbx-btn--danger" : "dbx-btn--primary", disabled: !!typeToConfirm });
    const cancel = button("Cancel", () => dialog.close());
    const dialog = modal(title, h("div", { class: "modal-body" }, h("p", null, message),
      input && h("label", { class: "field" }, h("span", null, `Type ${typeToConfirm} to confirm`), input)), [cancel, ok]);
    input?.addEventListener("input", () => { ok.disabled = input.value !== typeToConfirm; });
    dialog.addEventListener("close", () => resolve(false));
    (input || cancel).focus();
  });
}

// field: { key, label, type: text|number|select|textarea|checkbox, value, options, hint, required, disabled }
export function formDialog({ title, fields, submitLabel = "Save", onSubmit }) {
  const controls = {};
  const error = h("div");
  const body = h("div", { class: "modal-body form-grid" }, fields.map((f) => {
    const id = `f-${f.key}`;
    let control;
    if (f.type === "select") control = h("select", { id, class: "dbx-select", disabled: f.disabled }, f.options.map(([v, l]) => h("option", { value: v, selected: v === f.value }, l)));
    else if (f.type === "textarea") control = h("textarea", { id, class: "dbx-textarea mono", rows: "4", spellcheck: "false", disabled: f.disabled }, f.value ?? "");
    else if (f.type === "checkbox") control = h("input", { id, type: "checkbox", checked: !!f.value, disabled: f.disabled });
    else control = h("input", { id, class: "dbx-input", type: f.type || "text", value: f.value ?? "", required: f.required, disabled: f.disabled, spellcheck: "false" });
    controls[f.key] = control;
    return h("label", { class: `field ${f.type === "checkbox" ? "inline" : ""}`, for: id }, h("span", null, f.label), control, f.hint && h("span", { class: "hint" }, f.hint));
  }), error);
  const submit = button(submitLabel, async () => {
    const values = {};
    for (const f of fields) {
      const c = controls[f.key];
      values[f.key] = f.type === "checkbox" ? c.checked : f.type === "number" ? Number(c.value || 0) : c.value;
    }
    submit.disabled = true;
    mount(error);
    try {
      await onSubmit(values);
      dialog.close();
    } catch (err) {
      mount(error, errorBox(err));
    } finally {
      submit.disabled = false;
    }
  }, { variant: "dbx-btn--primary" });
  const dialog = modal(title, body, [button("Cancel", () => dialog.close()), submit]);
  Object.values(controls)[0]?.focus();
  return dialog;
}
