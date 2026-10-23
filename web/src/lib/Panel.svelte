<script lang="ts">
  import { store } from "./contacts.svelte";
  import { ageText, fmtNum } from "./format";
  import { iataFlight } from "./airlines";

  const c = $derived(store.selected);
</script>

{#if c}
  <section class="panel">
    <header>
      <div>
        <h2>{c.label}</h2>
        <span class="sub">{c.kind} · {c.source}</span>
      </div>
      <button class="close" onclick={() => store.select(null)} aria-label="Close">×</button>
    </header>
    <dl>
      {#if c.kind === "aircraft"}
        {#if iataFlight(c.label)}<dt>Flight</dt><dd>{iataFlight(c.label)}</dd>{/if}
        <dt>Registration</dt><dd>{c.reg ?? "–"}</dd>
        <dt>Type</dt><dd>{c.type ?? "–"}{c.category ? ` (${c.category})` : ""}</dd>
        <dt>Altitude</dt><dd>{c.on_ground ? "ground" : fmtNum(c.alt_ft, " ft")}</dd>
        <dt>Vertical speed</dt><dd>{fmtNum(c.vs_fpm, " fpm")}</dd>
        <dt>Squawk</dt><dd class:alert={!!c.emergency}>{c.squawk ?? "–"}{c.emergency ? ` ${c.emergency}` : ""}</dd>
      {:else}
        <dt>MMSI</dt><dd>{c.id.slice(2)}</dd>
        <dt>Type</dt><dd>{c.type ?? "–"}</dd>
        <dt>Destination</dt><dd>{c.destination ?? "–"}</dd>
        <dt>Nav status</dt><dd>{c.nav_status ?? "–"}</dd>
      {/if}
      <dt>Speed</dt><dd>{fmtNum(c.speed_kt, " kt", 0)}</dd>
      <dt>Heading</dt><dd>{fmtNum(c.heading_deg, "°", 0)}</dd>
      <dt>Distance</dt><dd>{c.distance_km.toFixed(1)} km</dd>
      <dt>Seen</dt><dd>{ageText(c.seen, store.now)}</dd>
      {#if c.military}<dt>Flags</dt><dd class="alert">military</dd>{/if}
    </dl>
    <p class="track">Track: {store.track.length} points</p>
  </section>
{/if}

<style>
  .panel {
    padding: 0.75rem 1rem;
    border-bottom: 1px solid #2a2a2a;
  }
  header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 0.5rem;
  }
  h2 {
    font-size: 1.1rem;
    font-weight: 600;
  }
  .sub {
    font-size: 0.75rem;
    color: #9a9a9a;
  }
  .close {
    background: none;
    border: none;
    color: #9a9a9a;
    font-size: 1.4rem;
    cursor: pointer;
    line-height: 1;
  }
  dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 0.15rem 0.75rem;
    font-size: 0.85rem;
  }
  dt {
    color: #9a9a9a;
  }
  .alert {
    color: #ff9f43;
    font-weight: 600;
  }
  .track {
    margin-top: 0.5rem;
    font-size: 0.75rem;
    color: #9a9a9a;
  }
</style>
