import { ref, shallowRef } from "vue";

// useLoader runs fn now and on every load(), tracking its result and error.
export function useLoader<T>(fn: () => Promise<T>) {
  const data = shallowRef<T>();
  const error = ref<unknown>(null);
  const loading = ref(false);
  async function load() {
    loading.value = true;
    error.value = null;
    try {
      data.value = await fn();
    } catch (err) {
      error.value = err;
    } finally {
      loading.value = false;
    }
  }
  load();
  return { data, error, loading, load };
}
