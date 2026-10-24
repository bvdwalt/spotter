<script lang="ts">
  import type { Contact } from "./api";
  import { store } from "./contacts.svelte";
  import { ageText, fmtNum } from "./format";
  import { operator } from "./airlines";

  type SortKey = "nearest" | "highest" | "fastest" | "newest" | "name";
  const sorts: { key: SortKey; label: string }[] = [
    { key: "nearest", label: "Nearest" },
    { key: "highest", label: "Highest" },
    { key: "fastest", label: "Fastest" },
    { key: "newest", label: "Newest" },
    { key: "name", label: "Name" },
  ];
  let sortKey = $state<SortKey>("nearest");

  const rows = $derived.by(() => {
    const val = (c: Contact): number | string => {
      switch (sortKey) {
        case "nearest":
          return c.distance_km;
        case "highest":
          return -(c.alt_ft ?? -1);
        case "fastest":
          return -(c.speed_kt ?? -1);
        case "newest":
          return -Date.parse(c.seen);
        case "name":
          return c.label;
      }
    };
    return [...store.visible].sort((a, b) => {
      const va = val(a), vb = val(b);
      if (va === vb) return a.id < b.id ? -1 : 1;
      return va < vb ? -1 : 1;
    });
  });

  /** Who or what it is: airline for a flight, ship type for a vessel. */
  function who(c: Contact): string {
    if (c.kind === "vessel") return c.type ?? "";
    return operator(c.label)?.name ?? c.type ?? "";
  }

  /** Where it is going, when known. */
  function where(c: Contact): string {
    if (c.kind === "vessel") return c.destination ?? "";
    if (!c.route) return "";
    const code = (a: { iata?: string; icao?: string }) => a.iata || a.icao || "?";
    return `${code(c.route.from)} → ${code(c.route.to)}`;
  }

  function motion(c: Contact): string {
    const parts: string[] = [];
    if (c.kind === "aircraft") parts.push(c.on_ground ? "ground" : fmtNum(c.alt_ft, " ft"));
    if (c.speed_kt !== undefined) parts.push(fmtNum(c.speed_kt, " kt"));
    if (c.heading_deg !== undefined) parts.push(fmtNum(c.heading_deg, "°"));
    return parts.join(" · ");
  }

  let list = $state<HTMLUListElement>();
  $effect(() => {
    const id = store.selectedId;
    if (!id || !list) return;
    list.querySelector<HTMLElement>(`[data-id="${CSS.escape(id)}"]`)?.scrollIntoView({ block: "nearest" });
  });
</script>

<div class="head">
  <span class="label">Sort by</span>
  <div class="segmented" role="radiogroup" aria-label="Sort contacts">
    {#each sorts as s}
      <button role="radio" aria-checked={sortKey === s.key} class:on={sortKey === s.key} onclick={() => (sortKey = s.key)}>
        {s.label}
      </button>
    {/each}
  </div>
</div>

{#if rows.length === 0}
  <div class="empty">
    {#if store.error}
      <p>Backend unreachable, retrying.</p>
    {:else if store.contacts.length === 0}
      <p>No aircraft or vessels in range yet.</p>
    {:else}
      <p>Nothing matches the current search and filters.</p>
      {#if store.query}<button class="link" onclick={() => (store.query = "")}>Clear search</button>{/if}
    {/if}
  </div>
{:else}
  <ul bind:this={list}>
    {#each rows as c (c.id)}
      <li
        data-id={c.id}
        class:selected={c.id === store.selectedId}
        class:military={c.military}
        class:emergency={!!c.emergency}
        class:watched={store.isWatched(c.id)}
      >
        <button onclick={() => store.select(c.id)}>
          <span class="glyph" aria-hidden="true">{c.kind === "vessel" ? "⛴" : "✈"}</span>
          <span class="main">
            <span class="line">
              <span class="name">{c.label}</span>
              <span class="who">{who(c)}</span>
            </span>
            <span class="line sub">
              {#if where(c)}<span class="where">{where(c)}</span>{/if}
              <span>{motion(c)}</span>
            </span>
          </span>
          <span class="side">
            <span class="km">{c.distance_km.toFixed(0)} km</span>
            <span class="age">{ageText(c.seen, store.now)}</span>
          </span>
        </button>
      </li>
    {/each}
  </ul>
{/if}

<style>
  .head {
    white-space: nowrap;
    position: sticky;
    top: 0;
    z-index: 1;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.4rem 1rem;
    background: var(--bg-1);
    border-bottom: 1px solid var(--line);
    font-size: 0.75rem;
    color: var(--muted);
  }
  .segmented {
    display: inline-flex;
    padding: 2px;
    background: var(--bg-0);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    overflow-x: auto;
  }
  .segmented button {
    min-height: 1.6rem;
    padding: 0 0.55rem;
    border: none;
    border-radius: 4px;
    background: none;
    color: var(--muted);
    font-size: 0.75rem;
    cursor: pointer;
    white-space: nowrap;
  }
  .segmented button.on {
    background: var(--bg-2);
    color: var(--text);
  }
  ul {
    list-style: none;
    font-size: 0.85rem;
    font-variant-numeric: tabular-nums;
  }
  li {
    border-bottom: 1px solid var(--line);
    border-left: 3px solid transparent;
  }
  li.watched {
    border-left-color: var(--accent);
  }
  li.military {
    border-left-color: var(--warn);
  }
  li.emergency {
    border-left-color: var(--danger);
  }
  li.selected {
    background: var(--accent-dim);
  }
  li > button {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    width: 100%;
    min-height: 3rem;
    padding: 0.45rem 1rem 0.45rem 0.75rem;
    background: none;
    border: none;
    color: inherit;
    text-align: left;
    cursor: pointer;
  }
  li > button:hover {
    background: var(--bg-2);
  }
  li.selected > button:hover {
    background: var(--accent-dim);
  }
  .glyph {
    flex: 0 0 1.2em;
    color: var(--muted);
  }
  li.military .glyph {
    color: var(--warn);
  }
  li.emergency .glyph {
    color: var(--danger);
  }
  .main {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
  }
  .line {
    display: flex;
    gap: 0.5rem;
    min-width: 0;
  }
  .line > * {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .name {
    flex: 0 0 auto;
    font-weight: 600;
  }
  li.emergency .name {
    color: var(--danger);
  }
  .who {
    color: var(--muted);
  }
  .sub {
    font-size: 0.75rem;
    color: var(--muted);
  }
  .where {
    flex: 0 0 auto;
    color: var(--text);
  }
  .side {
    flex: 0 0 auto;
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 0.1rem;
  }
  .km {
    font-weight: 500;
  }
  .age {
    font-size: 0.7rem;
    color: var(--muted);
  }
  .empty {
    padding: 2rem 1rem;
    text-align: center;
    color: var(--muted);
  }
  @media (max-width: 800px) {
    .segmented button {
      min-height: 2.1rem;
    }
    li > button {
      min-height: 3.4rem;
    }
  }
  .link {
    margin-top: 0.5rem;
    border: none;
    background: none;
    color: var(--accent);
    cursor: pointer;
  }
</style>
