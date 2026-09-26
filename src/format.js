export const num = (n) => Number(n || 0).toLocaleString();

export function bytes(n) {
  n = Number(n || 0);
  if (n < 1024) return `${n} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let i = -1;
  do {
    n /= 1024;
    i++;
  } while (n >= 1024 && i < units.length - 1);
  return `${n.toFixed(n < 10 ? 2 : 1)} ${units[i]}`;
}

export const time = (t) => (t && !String(t).startsWith("0001") ? new Date(t).toLocaleString() : "—");

export function payloadText(p) {
  if (p.encoding === "base64") return `[binary, ${bytes(p.size)}] base64:\n${p.data}`;
  let text = p.data;
  try {
    text = JSON.stringify(JSON.parse(p.data), null, 2);
  } catch {
    // not JSON
  }
  return p.truncated ? `${text}\n… truncated (${bytes(p.size)} total)` : text;
}

export function parseHeaders(text) {
  const out = {};
  for (const line of text.split("\n")) {
    const i = line.indexOf(":");
    if (i > 0) out[line.slice(0, i).trim()] = line.slice(i + 1).trim();
  }
  return out;
}

export const headerLine = (headers) =>
  headers && Object.keys(headers).length ? Object.entries(headers).map(([k, v]) => `${k}: ${v}`).join("  ·  ") : "";
