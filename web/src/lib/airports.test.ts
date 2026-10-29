import { describe, expect, it } from "vitest";
import type { Leg, Position } from "./api";
import { type AirportPoint, legEnds, nearestAirport } from "./airports";

const airports: AirportPoint[] = [
  { code: "BFN", lat: -29.0927, lon: 26.3024 },
  { code: "JNB", lat: -26.1392, lon: 28.246 },
  { code: "", lat: -29.0, lon: 26.31 }, // an airstrip without a code
];

function p(lat: number, lon: number, extra: Partial<Position> = {}): Position {
  return { lat, lon, time: "2026-10-29T08:00:00Z", ...extra };
}

function leg(...positions: Position[]): Leg {
  return { start: positions[0].time, end: positions[positions.length - 1].time, positions };
}

describe("nearestAirport", () => {
  it("finds the closest coded airport within range", () => {
    expect(nearestAirport(-29.1, 26.31, airports, 10)?.code).toBe("BFN");
  });

  it("returns nothing when no airport is close enough", () => {
    expect(nearestAirport(-27.5, 27.2, airports, 10)).toBeUndefined();
  });
});

describe("legEnds", () => {
  it("names a leg from its ground departure to its touchdown", () => {
    const l = leg(p(-29.093, 26.303, { on_ground: true }), p(-27.5, 27.2, { alt_ft: 30000 }), p(-26.14, 28.245, { on_ground: true }));
    expect(legEnds(l, airports)).toEqual({ from: "BFN", to: "JNB" });
  });

  it("accepts a first point seen just after takeoff", () => {
    const l = leg(p(-29.05, 26.33, { alt_ft: 2500 }), p(-27.5, 27.2, { alt_ft: 30000 }));
    expect(legEnds(l, airports).from).toBe("BFN");
  });

  it("does not name an end seen at cruise height over an airport", () => {
    const l = leg(p(-29.093, 26.303, { alt_ft: 30000 }), p(-26.14, 28.245, { on_ground: true }));
    expect(legEnds(l, airports)).toEqual({ from: undefined, to: "JNB" });
  });

  it("does not name an end without a known altitude", () => {
    const l = leg(p(-29.093, 26.303), p(-27.5, 27.2, { alt_ft: 30000 }));
    expect(legEnds(l, airports).from).toBeUndefined();
  });
});
