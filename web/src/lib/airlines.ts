interface Airline {
  iata: string;
  icao: string;
  name: string;
}

/** Carriers commonly seen over southern Africa, keyed by ICAO callsign prefix. */
const AIRLINES: Airline[] = [
  { iata: "FA", icao: "SFR", name: "FlySafair" },
  { iata: "SA", icao: "SAA", name: "South African Airways" },
  { iata: "4Z", icao: "LNK", name: "Airlink" },
  { iata: "5Z", icao: "KEM", name: "CemAir" },
  { iata: "MN", icao: "CAW", name: "Comair / kulula" },
  { iata: "TM", icao: "LAM", name: "LAM Mozambique" },
  { iata: "WB", icao: "RWD", name: "RwandAir" },
  { iata: "ET", icao: "ETH", name: "Ethiopian Airlines" },
  { iata: "KQ", icao: "KQA", name: "Kenya Airways" },
  { iata: "MS", icao: "MSR", name: "EgyptAir" },
  { iata: "EK", icao: "UAE", name: "Emirates" },
  { iata: "QR", icao: "QTR", name: "Qatar Airways" },
  { iata: "EY", icao: "ETD", name: "Etihad" },
  { iata: "SV", icao: "SVA", name: "Saudia" },
  { iata: "TK", icao: "THY", name: "Turkish Airlines" },
  { iata: "BA", icao: "BAW", name: "British Airways" },
  { iata: "VS", icao: "VIR", name: "Virgin Atlantic" },
  { iata: "LH", icao: "DLH", name: "Lufthansa" },
  { iata: "LX", icao: "SWR", name: "Swiss" },
  { iata: "KL", icao: "KLM", name: "KLM" },
  { iata: "AF", icao: "AFR", name: "Air France" },
  { iata: "DE", icao: "CFG", name: "Condor" },
  { iata: "SQ", icao: "SIA", name: "Singapore Airlines" },
  { iata: "CX", icao: "CPA", name: "Cathay Pacific" },
  { iata: "QF", icao: "QFA", name: "Qantas" },
  { iata: "DL", icao: "DAL", name: "Delta" },
  { iata: "UA", icao: "UAL", name: "United" },
  { iata: "UB", icao: "UBT", name: "LIFT" },
];

export const IATA_TO_ICAO: Record<string, string> = Object.fromEntries(AIRLINES.map((a) => [a.iata, a.icao]));
const BY_ICAO: Record<string, Airline> = Object.fromEntries(AIRLINES.map((a) => [a.icao, a]));
const ICAO_TO_IATA: Record<string, string> = Object.fromEntries(AIRLINES.map((a) => [a.icao, a.iata]));

const callsignRe = /^([A-Z]{3})(\d{1,4}[A-Z]?)$/;

/** Returns the operator for an ICAO callsign such as "SFR806", or null when unknown. */
export function operator(callsign: string): Airline | null {
  const m = callsignRe.exec(callsign.toUpperCase());
  return (m && BY_ICAO[m[1]]) || null;
}

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
  const m = callsignRe.exec(callsign.toUpperCase());
  const iata = m && ICAO_TO_IATA[m[1]];
  return iata ? iata + m[2] : null;
}
