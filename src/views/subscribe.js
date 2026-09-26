import { invoke, on } from "../bridge.js";
import { pageHeader } from "../components/page.js";
import { messageCard } from "../components/payload.js";
import { button, empty, errorBox, h, mount, onEnter } from "../dom.js";
import { num } from "../format.js";

const MAX_LIVE = 500;

export function subscribeTab(panel) {
  const feeds = new Map();
  let live = [];
  let dropped = 0;
  let paused = false;
  let queued = false;

  const subject = h("input", { class: "dbx-input grow", placeholder: "orders.> or events.*", spellcheck: "false", "aria-label": "Subject" });
  const queue = h("input", { class: "dbx-input queue", placeholder: "queue group (optional)", spellcheck: "false", "aria-label": "Queue group" });
  const chips = h("div", { class: "chips" });
  const status = h("span", { class: "hint grow", "aria-live": "polite" });
  const log = h("div", { class: "msg-list" });
  const error = h("div");

  function drawChips() {
    if (!feeds.size) return mount(chips, h("span", { class: "hint" }, "No active subscriptions. Wildcards like orders.* and events.> work."));
    mount(chips, [...feeds].map(([id, f]) => h("span", { class: "chip" },
      h("code", null, f.queue ? `${f.subject} (${f.queue})` : f.subject), h("span", { class: "hint" }, num(f.count)),
      h("button", { type: "button", "aria-label": `Stop ${f.subject}`, title: "Stop", onclick: () => stop(id) }, "×"))));
  }

  function drawLog() {
    status.textContent = `${num(live.length)} shown (last ${MAX_LIVE})${dropped ? ` · ${num(dropped)} dropped by the 100 msg/s limit` : ""}${paused ? " · paused" : ""}`;
    if (paused) return;
    mount(log, live.length ? live.map((m) => messageCard(m, m.reply && h("span", { class: "hint mono" }, `reply → ${m.reply}`))) : empty("Messages appear here as they arrive."));
  }

  async function add() {
    mount(error);
    const s = subject.value.trim();
    if (!s) return;
    try {
      const { feedId } = await invoke("nats/subscribe", { subject: s, queue: queue.value.trim() });
      feeds.set(feedId, { subject: s, queue: queue.value.trim(), count: 0 });
      subject.value = "";
      drawChips();
    } catch (err) {
      mount(error, errorBox(err));
    }
  }

  async function stop(id) {
    await invoke("nats/feedStop", { feedId: id }).catch(() => {});
    feeds.delete(id);
    drawChips();
  }

  const offMessage = on("nats/message", (m) => {
    const feed = feeds.get(m.feedId);
    if (!feed) return;
    feed.count++;
    dropped += m.dropped || 0;
    if (paused) return;
    live.unshift(m);
    if (live.length > MAX_LIVE) live.length = MAX_LIVE;
    if (!queued) {
      queued = true;
      requestAnimationFrame(() => { queued = false; drawLog(); drawChips(); });
    }
  });
  const offClosed = on("nats/feedClosed", (f) => { if (feeds.delete(f.feedId)) drawChips(); });

  onEnter(subject, add);
  onEnter(queue, add);
  const pause = button("Pause", () => { paused = !paused; pause.textContent = paused ? "Resume" : "Pause"; drawLog(); });
  mount(panel,
    pageHeader("Subscribe", "Watch subjects live. Each subscription forwards up to 100 messages per second."),
    h("div", { class: "toolbar" }, subject, queue, button("Subscribe", add, { variant: "dbx-btn--primary" })),
    error, chips,
    h("div", { class: "toolbar" }, status, pause, button("Clear", () => { live = []; dropped = 0; drawLog(); })),
    log);
  drawChips();
  drawLog();

  return () => {
    offMessage();
    offClosed();
    for (const id of feeds.keys()) invoke("nats/feedStop", { feedId: id }).catch(() => {});
  };
}
