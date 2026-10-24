<script lang="ts">
  import Map from "./lib/Map.svelte";
  import Panel from "./lib/Panel.svelte";
  import Alerts from "./lib/Alerts.svelte";
  import Table from "./lib/Table.svelte";
  import Filters from "./lib/Filters.svelte";
  import { store } from "./lib/contacts.svelte";
  import { ageText } from "./lib/format";
  import type { SourceStatus } from "./lib/api";

  $effect(() => {
    store.start();
  });

  const sources = $derived(store.status?.sources ?? []);
  const sourceOk = (s: SourceStatus) =>
    !!s.last_success && !(s.last_error_at && s.last_success && s.last_error_at > s.last_success);
  const anyBad = $derived(sources.some((s) => !sourceOk(s)));
  let showStatus = $state(false);
  const statusText = $derived.by(() => {
    if (store.error) return `Backend unreachable, retrying (${store.error})`;
    if (sources.length === 0) return "Connecting to the backend";
    return sources
      .map((s) => {
        const state = sourceOk(s) ? "ok" : (s.last_error ?? "waiting");
        return `${s.name}: ${state}${s.last_success ? ` (${ageText(s.last_success, store.now)})` : ""}`;
      })
      .join(" · ");
  });
</script>

<div class="layout">
  <div class="map-wrap">
    <Map />
  </div>
  <aside>
    <header class="toolbar">
      <div class="brand">
        <strong>spotter</strong>
        <button
          class="dot"
          class:bad={anyBad || !!store.error}
          title={statusText}
          aria-label="Data sources: {statusText}"
          onclick={() => (showStatus = !showStatus)}
        ></button>
        <span class="count">{store.visible.length} of {store.contacts.length}</span>
      </div>
      <div class="controls">
        <input type="search" placeholder="Callsign, flight, reg, type" bind:value={store.query} aria-label="Search contacts" />
        <Filters />
      </div>
      {#if showStatus}
        <p class="status" class:bad={anyBad || !!store.error}>{statusText}</p>
      {/if}
    </header>
    <Alerts />
    <Panel />
    <div class="table-wrap">
      <Table />
    </div>
  </aside>
</div>

<style>
  .layout {
    display: grid;
    grid-template-columns: 1fr 420px;
    height: 100vh;
  }
  .map-wrap {
    position: relative;
  }
  aside {
    display: flex;
    flex-direction: column;
    background: var(--bg-1);
    border-left: 1px solid var(--line);
    min-height: 0;
  }
  .toolbar {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    padding: 0.6rem 1rem;
    border-bottom: 1px solid var(--line);
    font-size: 0.85rem;
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .count {
    color: var(--muted);
  }
  .dot {
    width: 0.6rem;
    height: 0.6rem;
    padding: 0;
    border: none;
    border-radius: 50%;
    background: var(--ok);
    cursor: pointer;
  }
  .dot.bad {
    background: var(--warn);
  }
  .controls {
    display: flex;
    gap: 0.5rem;
  }
  input[type="search"] {
    flex: 1;
    min-width: 0;
    min-height: 2.25rem;
    padding: 0 0.6rem;
    background: var(--bg-0);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    color: inherit;
  }
  .status {
    font-size: 0.75rem;
    color: var(--ok);
  }
  .status.bad {
    color: var(--warn);
  }
  .table-wrap {
    flex: 1;
    overflow: auto;
  }
  @media (max-width: 800px) {
    .layout {
      grid-template-columns: 1fr;
      grid-template-rows: 55vh 45vh;
    }
    aside {
      border-left: none;
      border-top: 1px solid var(--line);
    }
  }
</style>
