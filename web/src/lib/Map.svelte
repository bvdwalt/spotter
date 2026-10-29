<script lang="ts">
  import * as maplibregl from "maplibre-gl";
  import type { GeoJSONSource, StyleSpecification } from "maplibre-gl";
  import "maplibre-gl/dist/maplibre-gl.css";
  import workerUrl from "maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url";
  import { Protocol } from "pmtiles";
  import { layers as basemapLayers, namedFlavor } from "@protomaps/basemaps";
  import type { ClientConfig, Contact } from "./api";
  import { ICONS, loadIcon } from "./icons";
  import { store } from "./contacts.svelte";
  import { circleRing, greatCircle } from "./geo";

  // MapLibre 6 cannot find its worker inside a bundle, so Vite emits it and we point MapLibre at it.
  maplibregl.setWorkerUrl(workerUrl);

  const OPENFREEMAP_STYLE = "https://tiles.openfreemap.org/styles/positron";

  /** Local PMTiles basemap with fonts and sprites served from this origin, so nothing leaves the LAN. */
  function localStyle(basemapUrl: string): StyleSpecification {
    return {
      version: 8,
      glyphs: `${location.origin}/basemap/fonts/{fontstack}/{range}.pbf`,
      sprite: `${location.origin}/basemap/sprites/v4/light`,
      sources: {
        protomaps: {
          type: "vector",
          url: `pmtiles://${location.origin}${basemapUrl}`,
          attribution: '<a href="https://protomaps.com">Protomaps</a> © <a href="https://openstreetmap.org">OpenStreetMap</a>',
        },
      },
      layers: basemapLayers("protomaps", namedFlavor("light"), { lang: "en" }),
    };
  }

  let protocolRegistered = false;
  let container: HTMLDivElement;
  let map: maplibregl.Map | null = null;
  let ready = $state(false);

  function iconFor(c: Contact): string {
    const base = c.kind === "vessel" ? "ship" : "plane";
    if (c.id === store.selectedId) return `${base}-selected`;
    if (c.kind === "aircraft") {
      if (c.emergency) return "plane-emergency";
      if (c.military) return "plane-military";
      if (c.on_ground) return "plane-ground";
    }
    return base;
  }

  function contactsGeoJSON(list: Contact[]): GeoJSON.FeatureCollection {
    return {
      type: "FeatureCollection",
      features: list.map((c) => ({
        type: "Feature",
        id: c.id,
        geometry: { type: "Point", coordinates: [c.lon, c.lat] },
        properties: {
          id: c.id,
          label: c.label,
          icon: iconFor(c),
          heading: c.heading_deg ?? 0,
          opacity: 0.25 + 0.75 * store.freshness(c),
        },
      })),
    };
  }

  /** One line per leg, the newest marked current. */
  function trackGeoJSON(): GeoJSON.FeatureCollection {
    const sel = store.selected;
    const features: GeoJSON.Feature[] = [];
    store.track.forEach((leg, i) => {
      const current = i === store.track.length - 1;
      const coords = leg.positions.map((p) => [p.lon, p.lat]);
      if (current && sel) coords.push([sel.lon, sel.lat]);
      if (coords.length > 1) {
        features.push({
          type: "Feature",
          geometry: { type: "LineString", coordinates: coords },
          properties: { current, callsign: leg.callsign ?? "" },
        });
      }
    });
    return { type: "FeatureCollection", features };
  }

  function routeGeoJSON(): GeoJSON.FeatureCollection {
    const sel = store.selected;
    const r = sel?.route;
    if (!sel || !r || !r.plausible) return { type: "FeatureCollection", features: [] };
    const here: [number, number] = [sel.lon, sel.lat];
    const line = (coords: [number, number][]) => ({
      type: "Feature" as const,
      geometry: { type: "LineString" as const, coordinates: coords },
      properties: {},
    });
    const point = (a: { lon: number; lat: number; iata: string; icao: string }) => ({
      type: "Feature" as const,
      geometry: { type: "Point" as const, coordinates: [a.lon, a.lat] },
      properties: { code: a.iata || a.icao },
    });
    return {
      type: "FeatureCollection",
      features: [
        line(greatCircle([r.from.lon, r.from.lat], here)),
        line(greatCircle(here, [r.to.lon, r.to.lat])),
        point(r.from),
        point(r.to),
      ],
    };
  }

  function planeRangeGeoJSON(cfg: ClientConfig): GeoJSON.FeatureCollection {
    return {
      type: "FeatureCollection",
      features: cfg.adsb_circles.map((c) => ({
        type: "Feature",
        geometry: { type: "Polygon", coordinates: [circleRing(c.lat, c.lon, c.radius_nm)] },
        properties: {},
      })),
    };
  }

  function shipAreaGeoJSON(cfg: ClientConfig): GeoJSON.FeatureCollection {
    if (!cfg.ais_bbox) return { type: "FeatureCollection", features: [] };
    const [[s, w], [n, e]] = cfg.ais_bbox;
    return {
      type: "FeatureCollection",
      features: [
        {
          type: "Feature",
          geometry: { type: "Polygon", coordinates: [[[w, s], [e, s], [e, n], [w, n], [w, s]]] },
          properties: {},
        },
      ],
    };
  }

  $effect(() => {
    const cfg = store.config;
    if (!cfg || map) return;
    if (cfg.basemap_url && !protocolRegistered) {
      maplibregl.addProtocol("pmtiles", new Protocol().tile);
      protocolRegistered = true;
    }
    map = new maplibregl.Map({
      container,
      style: cfg.basemap_url ? localStyle(cfg.basemap_url) : OPENFREEMAP_STYLE,
      center: [cfg.home_lon, cfg.home_lat],
      zoom: 7,
      attributionControl: { compact: true },
    });
    map.addControl(new maplibregl.NavigationControl({ showCompass: false }), "top-left");
    // MapLibre's own resize tracking misses some pane resizes; observe the container ourselves.
    const observer = new ResizeObserver(() => map?.resize());
    observer.observe(container);
    map.on("load", async () => {
      const m = map!;
      await Promise.all(
        Object.entries(ICONS).map(async ([name, svg]) => m.addImage(name, await loadIcon(svg))),
      );
      m.addSource("airports", { type: "geojson", data: "/airports.json" });
      const airportLayers = [
        { id: "airports-major", major: true, minzoom: 4, radius: 4.5, text: 11 },
        { id: "airports-minor", major: false, minzoom: 8, radius: 3, text: 10 },
      ];
      for (const a of airportLayers) {
        m.addLayer({
          id: `${a.id}-dot`,
          type: "circle",
          source: "airports",
          minzoom: a.minzoom,
          filter: ["==", ["get", "major"], a.major],
          paint: {
            "circle-radius": a.radius,
            "circle-color": "#5a6b7d",
            "circle-stroke-color": "#fff",
            "circle-stroke-width": 1.2,
          },
        });
        m.addLayer({
          id: `${a.id}-label`,
          type: "symbol",
          source: "airports",
          minzoom: a.minzoom,
          filter: ["==", ["get", "major"], a.major],
          layout: {
            "text-field": ["get", "code"],
            "text-size": a.text,
            "text-font": ["Noto Sans Regular"],
            "text-offset": [0, 0.9],
            "text-anchor": "top",
            "text-optional": true,
          },
          paint: { "text-color": "#445566", "text-halo-color": "#fff", "text-halo-width": 1.2 },
        });
      }
      m.addSource("plane-range", { type: "geojson", data: planeRangeGeoJSON(cfg) });
      m.addLayer({
        id: "plane-range-fill",
        type: "fill",
        source: "plane-range",
        layout: { visibility: store.planeRange ? "visible" : "none" },
        paint: { "fill-color": "#4ecdc4", "fill-opacity": 0.07 },
      });
      m.addLayer({
        id: "plane-range-line",
        type: "line",
        source: "plane-range",
        layout: { visibility: store.planeRange ? "visible" : "none" },
        paint: { "line-color": "#2a9d8f", "line-width": 1, "line-opacity": 0.5, "line-dasharray": [3, 3] },
      });
      m.addSource("ship-area", { type: "geojson", data: shipAreaGeoJSON(cfg) });
      m.addLayer({
        id: "ship-area-fill",
        type: "fill",
        source: "ship-area",
        layout: { visibility: store.shipArea ? "visible" : "none" },
        paint: { "fill-color": "#3a86ff", "fill-opacity": 0.06 },
      });
      m.addLayer({
        id: "ship-area-line",
        type: "line",
        source: "ship-area",
        layout: { visibility: store.shipArea ? "visible" : "none" },
        paint: { "line-color": "#3a86ff", "line-width": 1, "line-opacity": 0.5, "line-dasharray": [3, 3] },
      });
      m.addSource("home", {
        type: "geojson",
        data: { type: "Feature", geometry: { type: "Point", coordinates: [cfg.home_lon, cfg.home_lat] }, properties: {} },
      });
      m.addLayer({
        id: "home",
        type: "circle",
        source: "home",
        paint: { "circle-radius": 5, "circle-color": "#4ecdc4", "circle-stroke-color": "#111", "circle-stroke-width": 1.5 },
      });
      m.addSource("route", { type: "geojson", data: routeGeoJSON() });
      m.addLayer({
        id: "route",
        type: "line",
        source: "route",
        filter: ["==", ["geometry-type"], "LineString"],
        paint: { "line-color": "#6c5ce7", "line-width": 1.5, "line-dasharray": [2, 3], "line-opacity": 0.8 },
      });
      m.addLayer({
        id: "route-airports",
        type: "symbol",
        source: "route",
        filter: ["==", ["geometry-type"], "Point"],
        layout: {
          "text-field": ["get", "code"],
          "text-size": 11,
          "text-font": ["Noto Sans Regular"],
          "text-offset": [0, -1],
        },
        paint: { "text-color": "#6c5ce7", "text-halo-color": "#fff", "text-halo-width": 1.2 },
      });
      m.addSource("track", { type: "geojson", data: trackGeoJSON() });
      m.addLayer({
        id: "track",
        type: "line",
        source: "track",
        filter: ["all", ["==", ["geometry-type"], "LineString"], ["get", "current"]],
        paint: { "line-color": "#4ecdc4", "line-width": 2.5, "line-opacity": 0.9 },
      });
      // Earlier legs are dotted so they read as this track's history rather than a basemap road.
      m.addLayer({
        id: "track-past",
        type: "line",
        source: "track",
        filter: ["all", ["==", ["geometry-type"], "LineString"], ["!", ["get", "current"]]],
        paint: { "line-color": "#2a9d8f", "line-width": 2, "line-opacity": 0.6, "line-dasharray": [1, 2] },
      });
      // The label sits midway along its own leg so it cannot be read as the next leg's callsign.
      m.addLayer({
        id: "track-labels",
        type: "symbol",
        source: "track",
        filter: ["!", ["get", "current"]],
        layout: {
          "symbol-placement": "line-center",
          "text-field": ["get", "callsign"],
          "text-size": 10,
          "text-font": ["Noto Sans Regular"],
          "text-offset": [0, -0.8],
          "text-optional": true,
        },
        paint: { "text-color": "#2a9d8f", "text-halo-color": "#fff", "text-halo-width": 1.2 },
      });
      m.addSource("contacts", { type: "geojson", data: contactsGeoJSON([]) });
      m.addLayer({
        id: "contacts",
        type: "symbol",
        source: "contacts",
        layout: {
          "icon-image": ["get", "icon"],
          "icon-size": 0.42,
          "icon-rotate": ["get", "heading"],
          "icon-rotation-alignment": "map",
          "icon-allow-overlap": true,
          "icon-ignore-placement": true,
          "text-field": ["get", "label"],
          "text-size": 11,
          "text-offset": [0, 1.4],
          "text-anchor": "top",
          "text-optional": true,
          "text-font": ["Noto Sans Regular"],
        },
        paint: {
          "icon-opacity": ["get", "opacity"],
          "text-color": "#222",
          "text-halo-color": "#fff",
          "text-halo-width": 1.2,
        },
      });
      m.on("click", "contacts", (e) => {
        const id = e.features?.[0]?.properties?.id as string | undefined;
        if (id) store.select(id === store.selectedId ? null : id);
      });
      m.on("mouseenter", "contacts", () => (m.getCanvas().style.cursor = "pointer"));
      m.on("mouseleave", "contacts", () => (m.getCanvas().style.cursor = ""));
      ready = true;
    });
    return () => {
      observer.disconnect();
      map?.remove();
      map = null;
    };
  });

  $effect(() => {
    if (!ready || !map) return;
    const data = contactsGeoJSON(store.visible);
    (map.getSource("contacts") as GeoJSONSource | undefined)?.setData(data);
  });

  $effect(() => {
    if (!ready || !map) return;
    (map.getSource("track") as GeoJSONSource | undefined)?.setData(trackGeoJSON());
  });

  $effect(() => {
    if (!ready || !map) return;
    (map.getSource("route") as GeoJSONSource | undefined)?.setData(routeGeoJSON());
  });

  $effect(() => {
    if (!ready || !map) return;
    const mode = store.airports;
    const show = (id: string, on: boolean) => {
      for (const part of ["dot", "label"]) {
        map?.setLayoutProperty(`${id}-${part}`, "visibility", on ? "visible" : "none");
      }
    };
    show("airports-major", mode !== "off");
    show("airports-minor", mode === "all");
  });

  $effect(() => {
    if (!ready || !map) return;
    const toggle = (id: string, on: boolean) => {
      for (const part of ["fill", "line"]) {
        map?.setLayoutProperty(`${id}-${part}`, "visibility", on ? "visible" : "none");
      }
    };
    toggle("plane-range", store.planeRange);
    toggle("ship-area", store.shipArea);
  });

  // Follow: re-centre on the selected contact whenever its position changes.
  $effect(() => {
    const sel = store.selected;
    if (!ready || !map || !store.follow || !sel) return;
    map.easeTo({ center: [sel.lon, sel.lat], duration: 800 });
  });

  // Dragging the map is the natural way to stop following.
  $effect(() => {
    if (!ready || !map) return;
    const m = map;
    const stop = () => (store.follow = false);
    m.on("dragstart", stop);
    return () => {
      m.off("dragstart", stop);
    };
  });
</script>

<div class="map" bind:this={container}></div>

<style>
  .map {
    position: absolute;
    inset: 0;
  }
</style>
