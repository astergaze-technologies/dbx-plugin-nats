import BucketView from "../views/kv/BucketView.vue";
import StoreView from "../views/objects/StoreView.vue";
import StreamView from "../views/streams/StreamView.vue";
import { openTab } from "./workspace";

export const openStream = (name: string, reopen = false) =>
  openTab({ id: `stream:${name}`, title: name, icon: "streams", component: StreamView, props: { name } }, reopen);

export const openBucket = (bucket: string) =>
  openTab({ id: `kv:${bucket}`, title: bucket, icon: "kv", component: BucketView, props: { bucket } });

export const openStore = (store: string) =>
  openTab({ id: `objects:${store}`, title: store, icon: "objects", component: StoreView, props: { store } });
