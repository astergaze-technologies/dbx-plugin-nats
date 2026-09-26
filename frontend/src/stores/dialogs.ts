import { shallowRef } from "vue";

export interface FormField {
  key: string;
  label: string;
  type?: "text" | "number" | "select" | "textarea";
  value?: string | number;
  options?: [value: string, label: string][];
  hint?: string;
  required?: boolean;
  disabled?: boolean;
}

export type FormValues = Record<string, string | number>;

export interface ConfirmOptions {
  title: string;
  message: string;
  confirmLabel?: string;
  danger?: boolean;
  typeToConfirm?: string; // forces typing a name before a destructive action
}

export interface FormOptions {
  title: string;
  fields: FormField[];
  submitLabel?: string;
  onSubmit(values: FormValues): Promise<void>;
}

export type ActiveDialog =
  | ({ kind: "confirm"; resolve(ok: boolean): void } & ConfirmOptions)
  | ({ kind: "form" } & FormOptions);

export const activeDialog = shallowRef<ActiveDialog | null>(null);

export function confirmAction(options: ConfirmOptions): Promise<boolean> {
  return new Promise((resolve) => {
    activeDialog.value = { kind: "confirm", ...options, resolve };
  });
}

export function formDialog(options: FormOptions) {
  activeDialog.value = { kind: "form", ...options };
}
