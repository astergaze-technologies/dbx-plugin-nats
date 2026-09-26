import { createApp } from "vue";
import App from "./App.vue";
import { plugin, session, startEvents } from "./api/bridge";
import ServerView from "./views/server/ServerView.vue";
import { openTab, resetTabs } from "./stores/workspace";
import "./styles.css";

function start(connectionId: string) {
  if (connectionId === session.connectionId) return;
  resetTabs();
  session.connectionId = connectionId;
  session.readOnly = false;
  if (connectionId) openTab({ id: "server", title: "Server", icon: "server", component: ServerView, closable: false });
}

createApp(App, { inDbx: !!plugin }).mount("#app");

const host = plugin;
if (host) {
  startEvents();
  host.ready.then((context) => {
    start((context || host.context)?.connectionId || "");
    host.onContext?.((c) => start(c?.connectionId || ""));
  });
}
