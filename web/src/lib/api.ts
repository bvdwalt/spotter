export type Kind = "aircraft" | "vessel";

export interface Airport {
  iata: string;
  icao: string;
  name: string;
  lat: number;
  lon: number;
}

export interface Route {
  from: Airport;
  to: Airport;
  plausible: boolean;
}

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
  eta?: string;
  military: boolean;
  route?: Route;
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
  basemap_url?: string;
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

export interface Photo {
  thumbnail: string;
  large: string;
  link: string;
  photographer: string;
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

async function del(path: string): Promise<void> {
  const r = await fetch(path, { method: "DELETE" });
  if (!r.ok && r.status !== 404) throw new Error(`${path}: HTTP ${r.status}`);
}

async function put(path: string): Promise<void> {
  const r = await fetch(path, { method: "PUT" });
  if (!r.ok) throw new Error(`${path}: HTTP ${r.status}`);
}

export const api = {
  watchList: () => getJSON<{ watch: string[] }>("/api/watch"),
  watch: (id: string) => put(`/api/watch/${encodeURIComponent(id)}`),
  unwatch: (id: string) => del(`/api/watch/${encodeURIComponent(id)}`),
  clearAlerts: () => del("/api/alerts"),
  dismissAlert: (a: Alert) =>
    del(`/api/alerts/${a.rule}/${encodeURIComponent(a.contact_id)}?at=${encodeURIComponent(a.time)}`),
  config: () => getJSON<ClientConfig>("/api/config"),
  contacts: () => getJSON<{ now: string; contacts: Contact[] }>("/api/contacts"),
  track: (id: string) => getJSON<{ positions: Position[] }>(`/api/contacts/${encodeURIComponent(id)}/track`),
  status: () => getJSON<Status>("/api/status"),
  alerts: () => getJSON<{ alerts: Alert[] }>("/api/alerts"),
  photo: (id: string) => getJSON<{ photo: Photo }>(`/api/contacts/${encodeURIComponent(id)}/photo`),
};
