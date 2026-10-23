<script lang="ts">
  import Map from "./lib/Map.svelte";
  import Panel from "./lib/Panel.svelte";
  import Table from "./lib/Table.svelte";
  import { store } from "./lib/contacts.svelte";
  import { ageText } from "./lib/format";

  $effect(() => {
    store.start();
  });

  const source = $derived(store.status?.sources[0]);
  const sourceOk = $derived(!!source?.last_success && !(source.last_error_at && source.last_success && source.last_error_at > source.last_success));
</script>

<div class="layout">
  <div class="map-wrap">
    <Map />
  </div>
  <aside>
    <div class="toolbar">
      <strong>spotter</strong>
      <span class="count">{store.visible.length} / {store.contacts.length}</span>
      <input type="search" placeholder="Search callsign, reg, type" bind:value={store.query} />
      <label><input type="checkbox" bind:checked={store.hideGround} /> hide ground</label>
    </div>
    <div class="status" class:bad={!sourceOk || !!store.error}>
      {#if store.error}
        backend: {store.error}
      {:else if source}
        {source.name}: {sourceOk ? "ok" : (source.last_error ?? "waiting")}
        {#if source.last_success}· polled {ageText(source.last_success, store.now)}{/if}
      {:else}
        connecting…
      {/if}
    </div>
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
    background: #1b1b1b;
    border-left: 1px solid #2a2a2a;
    min-height: 0;
  }
  .toolbar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.5rem;
    padding: 0.6rem 1rem;
    border-bottom: 1px solid #2a2a2a;
    font-size: 0.85rem;
  }
  .count {
    color: #9a9a9a;
  }
  input[type="search"] {
    flex: 1 1 100%;
    padding: 0.35rem 0.5rem;
    background: #121212;
    border: 1px solid #333;
    border-radius: 4px;
    color: inherit;
  }
  label {
    display: flex;
    gap: 0.3rem;
    align-items: center;
    color: #9a9a9a;
  }
  .status {
    padding: 0.3rem 1rem;
    font-size: 0.75rem;
    color: #7cd992;
    border-bottom: 1px solid #2a2a2a;
  }
  .status.bad {
    color: #ff9f43;
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
      border-top: 1px solid #2a2a2a;
    }
  }
</style>
