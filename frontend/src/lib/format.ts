import type { Headers, Payload } from "../api/types";

export const num = (n?: number) => Number(n || 0).toLocaleString();

export function bytes(n?: number) {
  let v = Number(n || 0);
  if (v < 1024) return `${v} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let i = -1;
  do {
    v /= 1024;
    i++;
  } while (v >= 1024 && i < units.length - 1);
  return `${v.toFixed(v < 10 ? 2 : 1)} ${units[i]}`;
}

export const time = (t?: string) => (t && !t.startsWith("0001") ? new Date(t).toLocaleString() : "—");

export function payloadText(p: Payload) {
  if (p.encoding === "base64") return `[binary, ${bytes(p.size)}] base64:\n${p.data}`;
  let text = p.data;
  try {
    text = JSON.stringify(JSON.parse(p.data), null, 2);
  } catch {
    // not JSON
  }
  return p.truncated ? `${text}\n… truncated (${bytes(p.size)} total)` : text;
}

export function parseHeaders(text: string): Headers {
  const out: Headers = {};
  for (const line of text.split("\n")) {
    const i = line.indexOf(":");
    if (i > 0) out[line.slice(0, i).trim()] = line.slice(i + 1).trim();
  }
  return out;
}

export const headerLine = (headers?: Headers) =>
  headers ? Object.entries(headers).map(([k, v]) => `${k}: ${v}`).join("  ·  ") : "";

export const errorText = (err: unknown) => (err instanceof Error ? err.message : String((err as { message?: string })?.message ?? err));
