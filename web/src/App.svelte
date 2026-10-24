<script lang="ts">
  import Map from "./lib/Map.svelte";
  import Panel from "./lib/Panel.svelte";
  import Alerts from "./lib/Alerts.svelte";
  import ContactList from "./lib/ContactList.svelte";
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

  // Narrow screens show the panel as a bottom sheet over the map.
  type Snap = "peek" | "half" | "full";
  const snaps: Snap[] = ["peek", "half", "full"];
  let snap = $state<Snap>("half");
  let dragStartY: number | null = null;
  const step = (dir: 1 | -1) => (snap = snaps[Math.max(0, Math.min(2, snaps.indexOf(snap) + dir))]);
  function onHandleDown(e: PointerEvent) {
    dragStartY = e.clientY;
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
  }
  function onHandleUp(e: PointerEvent) {
    if (dragStartY === null) return;
    const dy = e.clientY - dragStartY;
    dragStartY = null;
    if (dy < -40) step(1);
    else if (dy > 40) step(-1);
    else snap = snap === "full" ? "peek" : "full";
  }
  function onHandleKey(e: KeyboardEvent) {
    if (e.key === "ArrowUp") step(1);
    else if (e.key === "ArrowDown") step(-1);
    else return;
    e.preventDefault();
  }
  // A contact picked on the map should be readable without a second gesture.
  $effect(() => {
    if (store.selectedId && snap === "peek") snap = "half";
  });
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
  <aside class="sheet-{snap}">
    <button
      class="handle"
      aria-label={snap === "full" ? "Collapse the list" : "Expand the list"}
      onpointerdown={onHandleDown}
      onpointerup={onHandleUp}
      onpointercancel={() => (dragStartY = null)}
      onkeydown={onHandleKey}
    ><span></span></button>
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
      <ContactList />
    </div>
  </aside>
</div>

<style>
  .layout {
    display: grid;
    grid-template-columns: 1fr 420px;
    height: 100vh;
    height: 100dvh;
  }
  .handle {
    display: none;
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
      grid-template-rows: 1fr;
    }
    aside {
      position: fixed;
      left: 0;
      right: 0;
      bottom: 0;
      z-index: 10;
      height: var(--shown);
      max-height: calc(100vh - 2rem);
      max-height: calc(100dvh - 2rem);
      padding-bottom: env(safe-area-inset-bottom);
      border-left: none;
      border-top: 1px solid var(--line-strong);
      border-radius: 14px 14px 0 0;
      box-shadow: 0 -8px 24px rgba(0, 0, 0, 0.45);
      transition: height 220ms ease-out;
    }
    aside.sheet-peek {
      --shown: 7.6rem;
    }
    aside.sheet-half {
      --shown: 50vh;
      --shown: 50dvh;
    }
    aside.sheet-full {
      --shown: 100vh;
      --shown: 100dvh;
    }
    .handle {
      display: flex;
      justify-content: center;
      align-items: center;
      flex: 0 0 auto;
      width: 100%;
      height: 1.6rem;
      border: none;
      background: none;
      cursor: grab;
      touch-action: none;
    }
    .handle span {
      width: 2.5rem;
      height: 4px;
      border-radius: 2px;
      background: var(--line-strong);
    }
    .toolbar {
      padding-top: 0;
    }
    input[type="search"] {
      min-height: 2.75rem;
    }
    .toolbar :global(.trigger) {
      min-height: 2.75rem;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    aside {
      transition: none;
    }
  }
</style>
