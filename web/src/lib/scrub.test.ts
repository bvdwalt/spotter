import { describe, expect, it } from "vitest";
import type { Leg, Position } from "./api";
import { positionAt } from "./scrub";

const T0 = Date.parse("2026-10-29T08:00:00Z");
const min = 60_000;

function p(minutes: number, lat: number, lon: number, extra: Partial<Position> = {}): Position {
  return { lat, lon, time: new Date(T0 + minutes * min).toISOString(), ...extra };
}

function leg(callsign: string, positions: Position[]): Leg {
  return { callsign, start: positions[0].time, end: positions[positions.length - 1].time, positions };
}

// PLZ to JNB, a ground stop, then JNB to DUR.
const legs = [
  leg("SFR177", [p(0, -34, 25.6, { alt_ft: 0, speed_kt: 150 }), p(10, -32, 26, { alt_ft: 30000, speed_kt: 450 }), p(60, -26.1, 28.2, { on_ground: true })]),
  leg("LNK553C", [p(100, -26.1, 28.2, { on_ground: true }), p(110, -27, 29)]),
];

describe("positionAt", () => {
  it("returns null without a track", () => {
    expect(positionAt([], T0)).toBeNull();
  });

  it("interpolates position, altitude and speed between stored points", () => {
    const s = positionAt(legs, T0 + 5 * min)!;
    expect(s.lat).toBeCloseTo(-33);
    expect(s.lon).toBeCloseTo(25.8);
    expect(s.alt_ft).toBe(15000);
    expect(s.speed_kt).toBe(300);
    expect(s.leg).toBe(0);
    expect(s.time).toBe(T0 + 5 * min);
  });

  it("returns a stored point exactly at its time", () => {
    const s = positionAt(legs, T0 + 10 * min)!;
    expect(s.lat).toBe(-32);
    expect(s.alt_ft).toBe(30000);
  });

  it("takes the heading from the direction of travel", () => {
    const east = [leg("X", [p(0, 0, 0), p(10, 0, 1)])];
    const north = [leg("X", [p(0, 0, 0), p(10, 1, 0)])];
    expect(positionAt(east, T0 + min)!.heading_deg).toBeCloseTo(90);
    expect(positionAt(north, T0 + min)!.heading_deg).toBeCloseTo(0);
  });

  it("keeps a known value when the other side is missing it", () => {
    const s = positionAt(legs, T0 + 30 * min)!;
    expect(s.alt_ft).toBe(30000);
    expect(s.speed_kt).toBe(450);
  });

  it("stays at the touchdown point during a ground stop between legs", () => {
    const s = positionAt(legs, T0 + 80 * min)!;
    expect(s.leg).toBe(0);
    expect(s.lat).toBe(-26.1);
    expect(s.on_ground).toBe(true);
  });

  it("finds a time in a later leg", () => {
    const s = positionAt(legs, T0 + 105 * min)!;
    expect(s.leg).toBe(1);
    expect(s.lat).toBeCloseTo(-26.55);
  });

  it("clamps to the ends of the track", () => {
    expect(positionAt(legs, T0 - 10 * min)!.lat).toBe(-34);
    const after = positionAt(legs, T0 + 500 * min)!;
    expect(after.leg).toBe(1);
    expect(after.lat).toBe(-27);
  });
});
