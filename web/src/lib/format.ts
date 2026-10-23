export function fmtNum(v: number | undefined, unit = "", digits = 0): string {
  if (v === undefined || v === null) return "–";
  return v.toFixed(digits) + unit;
}

export function ageText(seen: string, now: number): string {
  const s = Math.max(0, Math.round((now - Date.parse(seen)) / 1000));
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  return `${m}m ${s % 60}s ago`;
}

const NAV_STATUS: Record<number, string> = {
  0: "under way",
  1: "at anchor",
  2: "not under command",
  3: "restricted manoeuvrability",
  4: "constrained by draught",
  5: "moored",
  6: "aground",
  7: "fishing",
  8: "sailing",
  11: "towing astern",
  12: "towing alongside",
  14: "AIS-SART",
  15: "undefined",
};

export function navStatusText(code: number | undefined): string {
  if (code === undefined) return "–";
  return NAV_STATUS[code] ?? `status ${code}`;
}

export function durationText(ms: number): string {
  const m = Math.round(ms / 60000);
  if (m < 1) return "under a minute";
  if (m < 60) return `${m} min`;
  const h = Math.floor(m / 60);
  return `${h}h ${String(m % 60).padStart(2, "0")}m`;
}

export function clockText(iso: string | Date): string {
  const d = typeof iso === "string" ? new Date(iso) : iso;
  return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

export function dateTimeText(iso: string | Date): string {
  const d = typeof iso === "string" ? new Date(iso) : iso;
  return d.toLocaleString([], { weekday: "short", day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });
}
