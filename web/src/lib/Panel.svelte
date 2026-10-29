<script lang="ts">
  import { store } from "./contacts.svelte";
  import { api, type Photo } from "./api";
  import { ageText, clockText, dateTimeText, durationText, fmtNum, navStatusText } from "./format";
  import { iataFlight, operator } from "./airlines";
  import { haversineKm } from "./geo";
  import { legEnds } from "./airports";

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

  /** The leg the slider spans: the one being scrubbed, else the newest. The newest runs up to the live position. */
  const span = $derived.by(() => {
    const legs = store.track;
    if (!c || legs.length === 0) return null;
    const i = store.scrubbed?.leg ?? legs.length - 1;
    const last = i === legs.length - 1;
    const start = Date.parse(legs[i].start);
    const end = last ? Math.max(Date.parse(legs[i].end), Date.parse(c.seen)) : Date.parse(legs[i].end);
    return { i, last, start, end };
  });

  /** Each leg's name, ends and times. The newest is active while the contact is still out flying or sailing it. */
  const legRows = $derived.by(() => {
    if (!c) return [];
    const last = store.track.length - 1;
    return store.track.map((leg, i) => {
      const active = i === last && !c.on_ground;
      const ends = legEnds(leg, store.airportList);
      // An unfinished flight has not landed yet, so its scheduled destination stands in, marked as such.
      const scheduled = active && !ends.to && c.route?.plausible ? c.route.to.iata || c.route.to.icao : undefined;
      const start = Date.parse(leg.start);
      const end = active ? Math.max(Date.parse(leg.end), Date.parse(c.seen)) : Date.parse(leg.end);
      return { name: leg.callsign || `Leg ${i + 1}`, ...ends, scheduled, active, start, end };
    });
  });

  function onScrub(e: Event & { currentTarget: HTMLInputElement }) {
    const t = Number(e.currentTarget.value);
    // The end of the newest leg is now, so dragging all the way right goes back to live.
    store.scrubTo(span?.last && t >= span.end ? null : t);
  }
</script>

{#if c}
  <section class="panel">
    <header>
      <div>
        <h2>{c.label}</h2>
        <span class="sub">{c.kind} · {c.source}</span>
      </div>
      <div class="actions">
        <button class="watch" class:on={store.follow} onclick={() => (store.follow = !store.follow)}>
          {store.follow ? "following" : "follow"}
        </button>
        <button
          class="watch"
          class:on={store.isWatched(c.id)}
          title="Notify whenever this contact appears"
          onclick={() => store.toggleWatch(c.id)}
        >
          {store.isWatched(c.id) ? "alerting" : "alert me"}
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
    {#if span}
      <div class="track">
        {#if store.track.length > 1}
          <ol class="legs">
            {#each legRows as row, i}
              <li>
                <button class:on={i === span.i} onclick={() => store.pickLeg(i)}>
                  <span>
                    {row.name}
                    {#if row.from || row.to || row.scheduled}
                      <span class="muted">
                        {row.from ?? "?"} →
                        {#if row.to}{row.to}{:else if row.scheduled}<span class="dim" title="scheduled">{row.scheduled}</span>{:else}?{/if}
                      </span>
                    {/if}
                    {#if row.active}<span class="now">now</span>{/if}
                  </span>
                  <span class="muted">
                    {clockText(new Date(row.start))}–{row.active ? "now" : clockText(new Date(row.end))} · {durationText(row.end - row.start)}
                  </span>
                </button>
              </li>
            {/each}
          </ol>
        {/if}
        <input
          type="range"
          aria-label="Track time"
          min={span.start}
          max={span.end}
          step={10_000}
          value={store.scrubAt ?? span.end}
          disabled={span.end <= span.start}
          oninput={onScrub}
        />
        <div class="readout">
          {#if store.scrubbed}
            {@const s = store.scrubbed}
            <span>
              {clockText(new Date(s.time))} <span class="muted">{durationText(store.now - s.time)} ago</span>
              · {s.on_ground ? "ground" : fmtNum(s.alt_ft, " ft")} · {fmtNum(s.speed_kt, " kt", 0)}
            </span>
            <button class="watch" onclick={() => store.scrubTo(null)}>live</button>
          {:else}
            <span class="muted">Live · drag to look back</span>
          {/if}
        </div>
      </div>
    {/if}
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
  @media (max-width: 800px) {
    .watch,
    .close {
      min-height: 2.5rem;
      padding: 0 0.75rem;
    }
  }
  .track {
    margin-top: 0.75rem;
    font-size: 0.8rem;
  }
  .legs {
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    margin-bottom: 0.5rem;
  }
  .legs button {
    display: flex;
    justify-content: space-between;
    gap: 0.5rem;
    width: 100%;
    padding: 0.3rem 0.5rem;
    background: none;
    border: 1px solid #2a2a2a;
    border-radius: 4px;
    color: inherit;
    font: inherit;
    cursor: pointer;
    text-align: left;
  }
  .legs button.on {
    border-color: #4ecdc4;
  }
  .now {
    margin-left: 0.3rem;
    padding: 0 0.3rem;
    border-radius: 3px;
    background: #4ecdc4;
    color: #111;
    font-size: 0.65rem;
    font-weight: 600;
    text-transform: uppercase;
  }
  input[type="range"] {
    width: 100%;
    accent-color: #4ecdc4;
  }
  .readout {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.5rem;
    min-height: 1.6rem;
  }
</style>
