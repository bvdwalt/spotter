import type { Leg, Position } from "./api";
import { haversineKm } from "./geo";

export interface AirportPoint {
  code: string;
  lat: number;
  lon: number;
}

// An end of a leg this close to an airport, and on the ground or this low, is taken to be at it.
const NEAR_KM = 10;
const LOW_FT = 5000;

/** The coded airports from airports.json, the same file the map draws. */
export async function loadAirports(): Promise<AirportPoint[]> {
  const r = await fetch("/airports.json");
  if (!r.ok) throw new Error(`/airports.json: HTTP ${r.status}`);
  const fc = (await r.json()) as GeoJSON.FeatureCollection<GeoJSON.Point, { code: string }>;
  return fc.features
    .filter((f) => f.properties.code)
    .map((f) => ({ code: f.properties.code, lon: f.geometry.coordinates[0], lat: f.geometry.coordinates[1] }));
}

export function nearestAirport(lat: number, lon: number, airports: AirportPoint[], maxKm: number): AirportPoint | undefined {
  let best: AirportPoint | undefined;
  let bestKm = maxKm;
  for (const a of airports) {
    if (!a.code) continue;
    const km = haversineKm(lat, lon, a.lat, a.lon);
    if (km <= bestKm) {
      best = a;
      bestKm = km;
    }
  }
  return best;
}

/**
 * Where a leg departed and landed. ADS-B often misses aircraft on the ground, so a first point seen
 * low after takeoff still counts; an end at cruise height is the edge of coverage, not an airport.
 */
export function legEnds(leg: Leg, airports: AirportPoint[]): { from?: string; to?: string } {
  const ps = leg.positions;
  return { from: airportAt(ps[0], airports), to: airportAt(ps[ps.length - 1], airports) };
}

function airportAt(p: Position | undefined, airports: AirportPoint[]): string | undefined {
  if (!p) return undefined;
  const low = p.on_ground || (p.alt_ft !== undefined && p.alt_ft < LOW_FT);
  return low ? nearestAirport(p.lat, p.lon, airports, NEAR_KM)?.code : undefined;
}
