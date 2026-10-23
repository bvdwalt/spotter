# spotter

Lightweight live map of aircraft (and later ships) within a fixed radius of home.
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
| `ADSB_BASE_URL` | `https://api.adsb.lol/v2` | any readsb-style `/point/{lat}/{lon}/{nm}` API |
| `ADSB_POLL_INTERVAL` | `5s` | |
| `AISSTREAM_API_KEY` | empty | ships disabled when empty (not yet implemented) |
| `AIRCRAFT_EXPIRY` | `60s` | drop aircraft not seen for this long |
| `VESSEL_EXPIRY` | `15m` | |
| `TRACK_WINDOW` | `15m` | recent positions kept per contact |
| `TRACK_MIN_INTERVAL` | `10s` | downsample interval for tracks |

## API

| Path | Purpose |
|---|---|
| `GET /health` | liveness, always 200 while the process runs |
| `GET /api/contacts` | current contacts |
| `GET /api/contacts/{id}/track` | recent positions for one contact |
| `GET /api/status` | per-source health and contact counts |
| `GET /api/config` | home point, radius, expiry values for the UI |

## Development

`just test`, `just fmt`, `just lint`, `just build`. Tests use recorded fixtures only
(`internal/source/adsb/testdata/point.json`) and never call the live API.

Map tiles come from [OpenFreeMap](https://openfreemap.org). Aircraft data from
[adsb.lol](https://adsb.lol) (ODbL).
