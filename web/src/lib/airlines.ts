/** IATA to ICAO airline prefixes, for carriers commonly seen over southern Africa. */
export const IATA_TO_ICAO: Record<string, string> = {
  FA: "SFR", // FlySafair
  SA: "SAA", // South African Airways
  "4Z": "LNK", // Airlink
  "5Z": "KEM", // CemAir
  MN: "CAW", // Comair / kulula
  TM: "LAM", // LAM Mozambique
  WB: "RWD", // RwandAir
  ET: "ETH", // Ethiopian
  KQ: "KQA", // Kenya Airways
  MS: "MSR", // EgyptAir
  EK: "UAE", // Emirates
  QR: "QTR", // Qatar Airways
  EY: "ETD", // Etihad
  SV: "SVA", // Saudia
  TK: "THY", // Turkish
  BA: "BAW", // British Airways
  VS: "VIR", // Virgin Atlantic
  LH: "DLH", // Lufthansa
  LX: "SWR", // Swiss
  KL: "KLM",
  AF: "AFR", // Air France
  DE: "CFG", // Condor
  SQ: "SIA", // Singapore
  CX: "CPA", // Cathay Pacific
  QF: "QFA", // Qantas
  DL: "DAL", // Delta
  UA: "UAL", // United
};

const ICAO_TO_IATA: Record<string, string> = Object.fromEntries(
  Object.entries(IATA_TO_ICAO).map(([iata, icao]) => [icao, iata]),
);

const flightRe = /^([A-Z0-9]{2})(\d{0,4}[A-Z]?)$/;

/** Expands "fa105" to ["FA105", "SFR105"] so either form matches a callsign. */
export function searchTerms(query: string): string[] {
  const q = query.trim().toUpperCase();
  if (!q) return [];
  const m = flightRe.exec(q);
  const icao = m && IATA_TO_ICAO[m[1]];
  return icao ? [q, icao + m[2]] : [q];
}

/** Returns the IATA flight number for an ICAO callsign, e.g. "SFR105" to "FA105". */
export function iataFlight(callsign: string): string | null {
  const m = /^([A-Z]{3})(\d{1,4}[A-Z]?)$/.exec(callsign.toUpperCase());
  const iata = m && ICAO_TO_IATA[m[1]];
  return iata ? iata + m[2] : null;
}
