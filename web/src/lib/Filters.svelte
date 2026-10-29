<script lang="ts">
  import { store, type AirportMode } from "./contacts.svelte";

  let open = $state(false);
  let up = $state(false);
  let root: HTMLDivElement;

  // Open upward when the menu would run off the bottom of the screen (the sheet at peek or half).
  function place(menu: HTMLDivElement) {
    up = root.getBoundingClientRect().bottom + menu.offsetHeight + 8 > window.innerHeight;
  }

  // Count the options that differ from the defaults so the button shows there is something to undo.
  const active = $derived(
    [store.kind !== "all", !store.hideGround, store.airports !== "major", store.planeRange, store.shipArea].filter(Boolean)
      .length,
  );

  const kinds = [
    { value: "all", label: "All" },
    { value: "aircraft", label: "Aircraft" },
    { value: "vessel", label: "Vessels" },
  ] as const;
  const airportModes: { value: AirportMode; label: string }[] = [
    { value: "off", label: "Off" },
    { value: "major", label: "Major" },
    { value: "all", label: "All" },
  ];

  function onWindowClick(e: MouseEvent) {
    if (open && !root.contains(e.target as Node)) open = false;
  }
  function onKey(e: KeyboardEvent) {
    if (e.key === "Escape") open = false;
  }
  function reset() {
    store.kind = "all";
    store.hideGround = true;
    store.airports = "major";
    store.planeRange = false;
    store.shipArea = false;
  }
</script>

<svelte:window onclick={onWindowClick} onkeydown={onKey} />

<div class="filters" bind:this={root}>
  <button class="trigger" class:active={active > 0} aria-expanded={open} aria-controls="filters-menu" onclick={() => (open = !open)}>
    Filters{#if active > 0}<span class="badge">{active}</span>{/if}
  </button>
  {#if open}
    <div class="menu" class:up use:place id="filters-menu" role="group" aria-label="Filters and map layers">
      <div class="group">
        <span class="title">Show</span>
        <div class="segmented" role="radiogroup" aria-label="Contact kind">
          {#each kinds as k}
            <button role="radio" aria-checked={store.kind === k.value} class:on={store.kind === k.value} onclick={() => (store.kind = k.value)}>
              {k.label}
            </button>
          {/each}
        </div>
        <label class="switch">
          <input type="checkbox" role="switch" bind:checked={store.hideGround} />
          <span class="knob"></span>
          Hide aircraft on the ground
        </label>
      </div>
      <div class="group">
        <span class="title">Map layers</span>
        <div class="row">
          <span>Airports</span>
          <div class="segmented" role="radiogroup" aria-label="Airports">
            {#each airportModes as a}
              <button role="radio" aria-checked={store.airports === a.value} class:on={store.airports === a.value} onclick={() => (store.airports = a.value)}>
                {a.label}
              </button>
            {/each}
          </div>
        </div>
        <label class="switch">
          <input type="checkbox" role="switch" bind:checked={store.planeRange} />
          <span class="knob"></span>
          Aircraft coverage
        </label>
        {#if store.config?.ais_bbox}
          <label class="switch">
            <input type="checkbox" role="switch" bind:checked={store.shipArea} />
            <span class="knob"></span>
            Ship coverage
          </label>
        {/if}
      </div>
      <button class="reset" onclick={reset} disabled={active === 0}>Reset to defaults</button>
    </div>
  {/if}
</div>

<style>
  .filters {
    position: relative;
  }
  .trigger {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    min-height: 2.25rem;
    padding: 0 0.75rem;
    background: var(--bg-0);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    color: var(--text);
    cursor: pointer;
  }
  .trigger.active {
    border-color: var(--accent);
  }
  .badge {
    min-width: 1.2rem;
    padding: 0 0.3rem;
    border-radius: 999px;
    background: var(--accent);
    color: var(--bg-0);
    font-size: 0.7rem;
    font-weight: 600;
    text-align: center;
    line-height: 1.2rem;
  }
  .menu {
    position: absolute;
    right: 0;
    top: calc(100% + 0.4rem);
    z-index: 20;
    width: min(20rem, calc(100vw - 2rem));
    padding: 0.75rem;
    background: var(--bg-2);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    box-shadow: 0 12px 32px rgba(0, 0, 0, 0.5);
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    font-size: 0.85rem;
  }
  .menu.up {
    top: auto;
    bottom: calc(100% + 0.4rem);
  }
  .group {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .title {
    color: var(--muted);
    font-size: 0.75rem;
  }
  .row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.5rem;
  }
  .segmented {
    display: inline-flex;
    padding: 2px;
    background: var(--bg-0);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
  }
  .segmented button {
    min-height: 1.9rem;
    padding: 0 0.7rem;
    border: none;
    border-radius: 4px;
    background: none;
    color: var(--muted);
    cursor: pointer;
  }
  .segmented button.on {
    background: var(--bg-2);
    color: var(--text);
  }
  .switch {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    min-height: 2rem;
    cursor: pointer;
  }
  .switch input {
    position: absolute;
    opacity: 0;
    width: 0;
    height: 0;
  }
  .knob {
    position: relative;
    flex: 0 0 auto;
    width: 2rem;
    height: 1.15rem;
    border-radius: 999px;
    background: var(--line-strong);
    transition: background 120ms;
  }
  .knob::after {
    content: "";
    position: absolute;
    top: 2px;
    left: 2px;
    width: calc(1.15rem - 4px);
    height: calc(1.15rem - 4px);
    border-radius: 50%;
    background: var(--text);
    transition: transform 120ms;
  }
  .switch input:checked + .knob {
    background: var(--accent);
  }
  .switch input:checked + .knob::after {
    transform: translateX(0.85rem);
    background: var(--bg-0);
  }
  .switch input:focus-visible + .knob {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .reset {
    align-self: flex-start;
    padding: 0.2rem 0;
    border: none;
    background: none;
    color: var(--accent);
    cursor: pointer;
  }
  .reset:disabled {
    color: var(--muted);
    cursor: default;
  }
  @media (prefers-reduced-motion: reduce) {
    .knob,
    .knob::after {
      transition: none;
    }
  }
</style>
