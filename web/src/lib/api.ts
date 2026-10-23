export type Kind = "aircraft" | "vessel";

export interface Contact {
  id: string;
  kind: Kind;
  label: string;
  lat: number;
  lon: number;
  alt_ft?: number;
  speed_kt?: number;
  heading_deg?: number;
  vs_fpm?: number;
  on_ground: boolean;
  type?: string;
  reg?: string;
  squawk?: string;
  emergency?: string;
  category?: string;
  nav_status?: number;
  destination?: string;
  military: boolean;
  source: string;
  seen: string;
  distance_km: number;
}

export interface Position {
  lat: number;
  lon: number;
  alt_ft?: number;
  speed_kt?: number;
  heading_deg?: number;
  time: string;
}

export interface ClientConfig {
  home_lat: number;
  home_lon: number;
  radius_nm: number;
  aircraft_expiry_s: number;
  vessel_expiry_s: number;
  track_window_s: number;
  poll_interval_s: number;
}

export interface SourceStatus {
  name: string;
  enabled: boolean;
  last_success?: string;
  last_error?: string;
  last_error_at?: string;
  polls: number;
  errors: number;
  last_count: number;
}

export interface Status {
  version: string;
  now: string;
  sources: SourceStatus[];
  contacts: Record<Kind, number>;
}

export interface Alert {
  rule: "emergency" | "military" | "watchlist";
  contact_id: string;
  label: string;
  kind: Kind;
  title: string;
  message: string;
  time: string;
}

async function getJSON<T>(path: string): Promise<T> {
  const r = await fetch(path);
  if (!r.ok) throw new Error(`${path}: HTTP ${r.status}`);
  return r.json() as Promise<T>;
}

export const api = {
  config: () => getJSON<ClientConfig>("/api/config"),
  contacts: () => getJSON<{ now: string; contacts: Contact[] }>("/api/contacts"),
  track: (id: string) => getJSON<{ positions: Position[] }>(`/api/contacts/${encodeURIComponent(id)}/track`),
  status: () => getJSON<Status>("/api/status"),
  alerts: () => getJSON<{ alerts: Alert[] }>("/api/alerts"),
};
