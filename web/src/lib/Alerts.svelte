<script lang="ts">
  import { store } from "./contacts.svelte";
  import { ageText } from "./format";

  let open = $state(true);
  const shown = $derived(store.alerts.slice(0, 8));
</script>

{#if store.alerts.length > 0}
  <section class="alerts">
    <div class="head">
      <button class="toggle" onclick={() => (open = !open)}>
        Alerts <span class="count">{store.alerts.length}</span>
        <span class="chev">{open ? "▾" : "▸"}</span>
      </button>
      <button class="clear" onclick={() => store.clearAlerts()}>clear</button>
    </div>
    {#if open}
      <ul>
        {#each shown as a (a.rule + a.contact_id + a.time)}
          <li class={a.rule}>
            <button
              class="row"
              disabled={!store.contacts.some((c) => c.id === a.contact_id)}
              onclick={() => store.select(a.contact_id)}
              title={a.message}
            >
              <span class="title">{a.title}</span>
              <span class="when">{ageText(a.time, store.now)}</span>
            </button>
            <button class="dismiss" onclick={() => store.dismissAlert(a)} aria-label="Dismiss">×</button>
          </li>
        {/each}
      </ul>
    {/if}
  </section>
{/if}

<style>
  .alerts {
    border-bottom: 1px solid #2a2a2a;
    font-size: 0.8rem;
  }
  .head {
    display: flex;
    align-items: center;
    padding: 0.2rem 1rem 0.2rem 0;
  }
  .toggle {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.3rem 1rem;
    background: none;
    border: none;
    color: #e0e0e0;
    font: inherit;
    font-weight: 600;
    cursor: pointer;
    text-align: left;
  }
  .clear,
  .dismiss {
    background: none;
    border: none;
    color: #9a9a9a;
    font: inherit;
    cursor: pointer;
  }
  .clear:hover,
  .dismiss:hover {
    color: #e0e0e0;
  }
  li {
    display: flex;
    align-items: center;
  }
  .dismiss {
    padding: 0 0.8rem 0 0.4rem;
    font-size: 1.1rem;
    line-height: 1;
  }
  .count {
    color: #9a9a9a;
    font-weight: 400;
  }
  .chev {
    margin-left: auto;
    color: #9a9a9a;
  }
  ul {
    list-style: none;
    padding-bottom: 0.3rem;
  }
  .row {
    flex: 1;
    min-width: 0;
    display: flex;
    justify-content: space-between;
    gap: 0.5rem;
    padding: 0.2rem 1rem;
    background: none;
    border: none;
    color: inherit;
    font: inherit;
    cursor: pointer;
    text-align: left;
  }
  .row:disabled {
    color: #777;
    cursor: default;
  }
  .row:not(:disabled):hover {
    background: #222;
  }
  .when {
    color: #9a9a9a;
    white-space: nowrap;
  }
  li.watchlist .title {
    color: #4ecdc4;
  }
</style>
