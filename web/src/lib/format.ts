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
