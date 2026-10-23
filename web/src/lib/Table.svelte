<script lang="ts">
  import type { Contact } from "./api";
  import { store } from "./contacts.svelte";
  import { ageText, fmtNum } from "./format";

  type Key = "label" | "type" | "alt_ft" | "speed_kt" | "heading_deg" | "distance_km" | "seen";
  let sortKey = $state<Key>("distance_km");
  let sortAsc = $state(true);

  const columns: { key: Key; title: string }[] = [
    { key: "label", title: "Name" },
    { key: "type", title: "Type" },
    { key: "alt_ft", title: "Alt ft" },
    { key: "speed_kt", title: "Kt" },
    { key: "heading_deg", title: "Hdg" },
    { key: "distance_km", title: "Km" },
    { key: "seen", title: "Seen" },
  ];

  function sortBy(k: Key) {
    if (sortKey === k) sortAsc = !sortAsc;
    else {
      sortKey = k;
      sortAsc = true;
    }
  }

  const rows = $derived.by(() => {
    const dir = sortAsc ? 1 : -1;
    const val = (c: Contact) => (sortKey === "seen" ? -Date.parse(c.seen) : (c[sortKey] ?? ""));
    return [...store.visible].sort((a, b) => {
      const va = val(a), vb = val(b);
      if (va === vb) return a.id < b.id ? -1 : 1;
      if (va === "" || va === undefined) return 1;
      if (vb === "" || vb === undefined) return -1;
      return (va < vb ? -1 : 1) * dir;
    });
  });
</script>

<table>
  <thead>
    <tr>
      {#each columns as col}
        <th onclick={() => sortBy(col.key)} class:active={sortKey === col.key}>
          {col.title}{sortKey === col.key ? (sortAsc ? " ▲" : " ▼") : ""}
        </th>
      {/each}
    </tr>
  </thead>
  <tbody>
    {#each rows as c (c.id)}
      <tr class:selected={c.id === store.selectedId} class:military={c.military} class:emergency={!!c.emergency} onclick={() => store.select(c.id)}>
        <td>{c.kind === "vessel" ? "⛴ " : ""}{c.label}</td>
        <td>{c.type ?? ""}</td>
        <td class="num">{c.kind === "vessel" ? "" : c.on_ground ? "gnd" : fmtNum(c.alt_ft)}</td>
        <td class="num">{fmtNum(c.speed_kt)}</td>
        <td class="num">{fmtNum(c.heading_deg)}</td>
        <td class="num">{c.distance_km.toFixed(0)}</td>
        <td class="num">{ageText(c.seen, store.now)}</td>
      </tr>
    {/each}
    {#if rows.length === 0}
      <tr><td colspan="7" class="empty">No contacts</td></tr>
    {/if}
  </tbody>
</table>

<style>
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.8rem;
    font-variant-numeric: tabular-nums;
  }
  th,
  td {
    padding: 0.3rem 0.5rem;
    text-align: left;
    white-space: nowrap;
  }
  th {
    position: sticky;
    top: 0;
    background: #1b1b1b;
    color: #9a9a9a;
    font-weight: 500;
    cursor: pointer;
    user-select: none;
  }
  th.active {
    color: #e0e0e0;
  }
  tbody tr {
    cursor: pointer;
    border-top: 1px solid #222;
  }
  tbody tr:hover {
    background: #222;
  }
  tr.selected {
    background: #163b38;
  }
  tr.military td:first-child {
    color: #ff9f43;
  }
  tr.emergency td:first-child {
    color: #ff4757;
    font-weight: 600;
  }
  .num {
    text-align: right;
  }
  .empty {
    color: #9a9a9a;
    text-align: center;
    padding: 1rem;
  }
</style>
