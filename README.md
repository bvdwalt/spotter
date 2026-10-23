# spotter

Lightweight live map of aircraft and ships within a fixed radius of home.
One Go binary with the Svelte frontend embedded, no database, no external services
beyond the ADS-B aggregator and the browser's map tiles.

## Run

```bash
cp .env.example .env   # set HOME_LAT / HOME_LON
just dev               # Go backend on :8080
just web-dev           # Vite on :5173, proxies /api to :8080
```

Or with Docker:

```bash
docker build -t spotter .
docker run -p 8080:8080 -e HOME_LAT=-26.2 -e HOME_LON=28.05 spotter
```

## Configuration

| Var | Default | Notes |
|---|---|---|
| `PORT` | `8080` | |
| `HOME_LAT`, `HOME_LON` | required | centre of the region |
| `RADIUS_NM` | `150` | aircraft query radius, max 250 |
| `ADSB_BASE_URL` | `https://opendata.adsb.fi/api/v2` | `https://api.adsb.lol/v2` also works, but caps at 500 requests a day |
| `ADSB_POLL_INTERVAL` | `5s` | adsb.fi allows one request per second |
| `AISSTREAM_API_KEY` | empty | from [aisstream.io](https://aisstream.io); ships disabled when empty |
| `AIS_BBOX` | derived from home and radius | `latSW,lonSW,latNE,lonNE`, for example a coastal box when home is inland |
| `AIRCRAFT_EXPIRY` | `60s` | drop aircraft not seen for this long |
| `VESSEL_EXPIRY` | `15m` | |
| `TRACK_WINDOW` | `15m` | recent positions kept per contact |
| `TRACK_MIN_INTERVAL` | `10s` | downsample interval for tracks |
| `ROUTE_API_URL` | `https://adsb.im/api/0/routeset` | route lookup by callsign; empty disables |
| `PHOTO_API_URL` | `https://api.planespotters.net/pub/photos/hex` | aircraft photos; empty disables |
| `NTFY_URL` | empty | ntfy topic URL for alerts; empty keeps alerts in the UI only |
| `NTFY_TOKEN` | empty | bearer token for ntfy, if the topic needs one |
| `WATCH_LIST` | empty | comma-separated ICAO hex codes and MMSIs to alert on |
| `ALERT_COOLDOWN` | `1h` | minimum gap between repeat alerts for one contact and rule |

## API

| Path | Purpose |
|---|---|
| `GET /health` | liveness, always 200 while the process runs |
| `GET /api/contacts` | current contacts |
| `GET /api/contacts/{id}/track` | recent positions for one contact |
| `GET /api/contacts/{id}/photo` | first planespotters.net photo for an aircraft |
| `PUT`/`DELETE /api/watch/{id}` | add or remove a contact from the watch list |
| `DELETE /api/alerts` | clear recent alerts |
| `GET /api/status` | per-source health and contact counts |
| `GET /api/alerts` | recent alerts, newest first |
| `GET /api/config` | home point, radius, expiry values for the UI |

## Alerts

Three rules, evaluated on every update: emergency squawks (7500, 7600, 7700),
aircraft flagged military by the aggregator, and contacts on `WATCH_LIST`. Each
alert shows in the sidebar and, when `NTFY_URL` is set, is posted to ntfy with a
title, tags, and high priority for emergencies.

## Development

`just test`, `just fmt`, `just lint`, `just build`. Tests use recorded fixtures only
(`internal/source/adsb/testdata/point.json`) and never call the live API.

Map tiles come from [OpenFreeMap](https://openfreemap.org). Aircraft data from
[adsb.fi](https://adsb.fi) (non-commercial, attribution required), ship data from [AISStream](https://aisstream.io),
routes from the [adsb.im](https://adsb.im) route database, aircraft photos from
[planespotters.net](https://www.planespotters.net/photo/api).
