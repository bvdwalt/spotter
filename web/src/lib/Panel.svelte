<script lang="ts">
  import { store } from "./contacts.svelte";
  import { api, type Photo } from "./api";
  import { ageText, clockText, dateTimeText, durationText, fmtNum, navStatusText } from "./format";
  import { iataFlight, operator } from "./airlines";
  import { haversineKm } from "./geo";

  const c = $derived(store.selected);

  let photo = $state<Photo | null>(null);
  $effect(() => {
    const id = store.selectedId;
    photo = null;
    if (!id || !id.startsWith("a:")) return;
    let live = true;
    api
      .photo(id)
      .then((r) => live && (photo = r.photo))
      .catch(() => undefined);
    return () => {
      live = false;
    };
  });

  /** Aircraft only: distance left along the route and the time it takes at the current ground speed. */
  const progress = $derived.by(() => {
    if (!c || !c.route || !c.route.plausible) return null;
    const toGoKm = haversineKm(c.lat, c.lon, c.route.to.lat, c.route.to.lon);
    const totalKm = haversineKm(c.route.from.lat, c.route.from.lon, c.route.to.lat, c.route.to.lon);
    const speedKmh = (c.speed_kt ?? 0) * 1.852;
    const remainingMs = speedKmh > 90 ? (toGoKm / speedKmh) * 3600_000 : null;
    return { toGoKm, totalKm, remainingMs, eta: remainingMs === null ? null : new Date(store.now + remainingMs) };
  });
</script>

{#if c}
  <section class="panel">
    <header>
      <div>
        <h2>{c.label}</h2>
        <span class="sub">{c.kind} · {c.source}</span>
      </div>
      <div class="actions">
        <button class="watch" class:on={store.isWatched(c.id)} onclick={() => store.toggleWatch(c.id)}>
          {store.isWatched(c.id) ? "watching" : "watch"}
        </button>
        <button class="close" onclick={() => store.select(null)} aria-label="Close">×</button>
      </div>
    </header>
    {#if photo}
      <a class="photo" href={photo.link} target="_blank" rel="noopener">
        <img src={photo.large} alt="{c.label} photo" loading="lazy" />
        <span class="credit">© {photo.photographer} · planespotters.net</span>
      </a>
    {/if}
    <dl>
      {#if c.kind === "aircraft"}
        {#if iataFlight(c.label)}<dt>Flight</dt><dd>{iataFlight(c.label)}</dd>{/if}
        {#if operator(c.label)}
          <dt>Operator</dt>
          <dd>{operator(c.label)?.name} <span class="muted">{operator(c.label)?.iata} / {operator(c.label)?.icao}</span></dd>
        {/if}
        {#if c.route}
          <dt>Route</dt>
          <dd class:dim={!c.route.plausible} title={c.route.plausible ? "" : "position does not fit this route"}>
            {c.route.from.iata || c.route.from.icao} → {c.route.to.iata || c.route.to.icao}
            <span class="sub">{c.route.from.name} → {c.route.to.name}</span>
          </dd>
          {#if progress}
            <dt>To go</dt>
            <dd>{progress.toGoKm.toFixed(0)} km of {progress.totalKm.toFixed(0)} km</dd>
            {#if progress.remainingMs !== null && progress.eta}
              <dt>Remaining</dt>
              <dd>{durationText(progress.remainingMs)} <span class="muted">at current speed</span></dd>
              <dt>ETA</dt>
              <dd>{clockText(progress.eta)} <span class="muted">estimated, local time</span></dd>
            {/if}
          {/if}
        {/if}
        <dt>Registration</dt><dd>{c.reg ?? "–"}</dd>
        <dt>Type</dt><dd>{c.type ?? "–"}{c.category ? ` (${c.category})` : ""}</dd>
        <dt>Altitude</dt><dd>{c.on_ground ? "ground" : fmtNum(c.alt_ft, " ft")}</dd>
        <dt>Vertical speed</dt><dd>{fmtNum(c.vs_fpm, " fpm")}</dd>
        <dt>Squawk</dt><dd class:alert={!!c.emergency}>{c.squawk ?? "–"}{c.emergency ? ` ${c.emergency}` : ""}</dd>
      {:else}
        <dt>MMSI</dt><dd>{c.id.slice(2)}</dd>
        <dt>Type</dt><dd>{c.type ?? "–"}</dd>
        <dt>Destination</dt><dd>{c.destination ?? "–"}</dd>
        {#if c.eta}
          <dt>ETA</dt>
          <dd>
            {dateTimeText(c.eta)} <span class="muted">reported by ship</span>
            {#if Date.parse(c.eta) > store.now}
              <span class="sub">{durationText(Date.parse(c.eta) - store.now)} from now</span>
            {/if}
          </dd>
        {/if}
        <dt>Status</dt><dd>{navStatusText(c.nav_status)}</dd>
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
  .actions {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .watch {
    background: none;
    border: 1px solid #444;
    border-radius: 4px;
    color: #9a9a9a;
    font: inherit;
    font-size: 0.75rem;
    padding: 0.15rem 0.5rem;
    cursor: pointer;
  }
  .watch.on {
    border-color: #4ecdc4;
    color: #4ecdc4;
  }
  .muted {
    color: #9a9a9a;
    font-size: 0.75rem;
  }
  .close {
    background: none;
    border: none;
    color: #9a9a9a;
    font-size: 1.4rem;
    cursor: pointer;
    line-height: 1;
  }
  .photo {
    display: block;
    position: relative;
    margin: 0.25rem 0 0.6rem;
    border-radius: 4px;
    overflow: hidden;
    background: #111;
  }
  .photo img {
    display: block;
    width: 100%;
    max-height: 220px;
    object-fit: cover;
  }
  .credit {
    position: absolute;
    right: 0;
    bottom: 0;
    padding: 0.1rem 0.4rem;
    font-size: 0.65rem;
    color: #ddd;
    background: rgba(0, 0, 0, 0.55);
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
  .dim {
    color: #9a9a9a;
  }
  dd .sub {
    display: block;
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
