import type { Leg, Position } from "./api";

/** Where a contact was at a past time, and which leg that time falls in. */
export interface Scrubbed {
  lat: number;
  lon: number;
  alt_ft?: number;
  speed_kt?: number;
  heading_deg?: number;
  on_ground: boolean;
  time: number;
  leg: number;
}

/**
 * The contact's position at time t, interpolated between stored points. That is exact enough because
 * history only drops points that lie on the straight line between the ones it keeps. During a stop
 * between legs it stays at the touchdown point; outside the track it clamps to the first or last point.
 */
export function positionAt(legs: Leg[], t: number): Scrubbed | null {
  if (legs.length === 0) return null;
  let leg = legs.findIndex((l) => t <= Date.parse(l.end));
  if (leg === -1) leg = legs.length - 1;
  else if (leg > 0 && t < Date.parse(legs[leg].start)) leg -= 1;

  const ps = legs[leg].positions;
  const k = ps.findIndex((p) => Date.parse(p.time) >= t);
  if (k === -1) return at(ps[ps.length - 1], t, leg);
  if (k === 0 || Date.parse(ps[k].time) === t) return at(ps[k], t, leg);

  const a = ps[k - 1];
  const b = ps[k];
  const ta = Date.parse(a.time);
  const f = (t - ta) / (Date.parse(b.time) - ta);
  const moved = a.lat !== b.lat || a.lon !== b.lon;
  return {
    lat: a.lat + f * (b.lat - a.lat),
    lon: a.lon + f * (b.lon - a.lon),
    alt_ft: lerp(a.alt_ft, b.alt_ft, f),
    speed_kt: lerp(a.speed_kt, b.speed_kt, f),
    heading_deg: moved ? bearingDeg(a, b) : (a.heading_deg ?? b.heading_deg),
    on_ground: !!a.on_ground,
    time: t,
    leg,
  };
}

function at(p: Position, t: number, leg: number): Scrubbed {
  return { lat: p.lat, lon: p.lon, alt_ft: p.alt_ft, speed_kt: p.speed_kt, heading_deg: p.heading_deg, on_ground: !!p.on_ground, time: t, leg };
}

function lerp(a: number | undefined, b: number | undefined, f: number): number | undefined {
  if (a === undefined || b === undefined) return a ?? b;
  return Math.round(a + f * (b - a));
}

function bearingDeg(a: Position, b: Position): number {
  const rad = Math.PI / 180;
  const dLon = (b.lon - a.lon) * rad;
  const y = Math.sin(dLon) * Math.cos(b.lat * rad);
  const x = Math.cos(a.lat * rad) * Math.sin(b.lat * rad) - Math.sin(a.lat * rad) * Math.cos(b.lat * rad) * Math.cos(dLon);
  return (Math.atan2(y, x) / rad + 360) % 360;
}
