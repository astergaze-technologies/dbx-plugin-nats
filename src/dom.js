// Server data is untrusted: build DOM with textContent only, never innerHTML.
export function h(tag, attrs, ...children) {
  const el = document.createElement(tag);
  for (const [key, value] of Object.entries(attrs || {})) {
    if (value == null || value === false) continue;
    if (key.startsWith("on")) el.addEventListener(key.slice(2), value);
    else if (key === "class") el.className = value;
    else el.setAttribute(key, value === true ? "" : value);
  }
  append(el, children);
  return el;
}

export function append(el, children) {
  for (const child of children.flat(Infinity)) {
    if (child == null || child === false) continue;
    el.append(child instanceof Node ? child : String(child));
  }
  return el;
}

export const mount = (el, ...children) => {
  el.replaceChildren();
  return append(el, children);
};

export const button = (label, onclick, { variant = "", title, disabled, icon } = {}) =>
  h("button", { type: "button", class: `dbx-btn ${variant}`.trim(), onclick, title, disabled, "aria-label": title }, icon, label);

export const badge = (text, tone = "") => h("span", { class: `dbx-badge ${tone}`.trim() }, text);

export const empty = (text) => h("div", { class: "empty" }, text);

export const errorBox = (err) => h("div", { class: "error", role: "alert" }, err?.message || String(err));

export function onEnter(input, fn) {
  input.addEventListener("keydown", (e) => {
    if (e.key === "Enter") {
      e.preventDefault();
      fn();
    }
  });
  return input;
}

// Runs an async action with the button disabled and errors routed to onError.
export async function busy(btn, action, onError) {
  btn.disabled = true;
  try {
    return await action();
  } catch (err) {
    onError?.(err);
  } finally {
    btn.disabled = false;
  }
}
