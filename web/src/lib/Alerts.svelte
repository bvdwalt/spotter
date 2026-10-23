<script lang="ts">
  import { store } from "./contacts.svelte";
  import { ageText } from "./format";

  let open = $state(true);
  const shown = $derived(store.alerts.slice(0, 8));
</script>

{#if store.alerts.length > 0}
  <section class="alerts">
    <button class="head" onclick={() => (open = !open)}>
      Alerts <span class="count">{store.alerts.length}</span>
      <span class="chev">{open ? "▾" : "▸"}</span>
    </button>
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
    width: 100%;
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.4rem 1rem;
    background: none;
    border: none;
    color: #e0e0e0;
    font: inherit;
    font-weight: 600;
    cursor: pointer;
    text-align: left;
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
    width: 100%;
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
  li.emergency .title {
    color: #ff4757;
    font-weight: 600;
  }
  li.military .title {
    color: #ff9f43;
  }
  li.watchlist .title {
    color: #4ecdc4;
  }
</style>
