<script lang="ts">
  import maplibregl, { type GeoJSONSource } from "maplibre-gl";
  import "maplibre-gl/dist/maplibre-gl.css";
  import type { Contact } from "./api";
  import { ICONS, loadIcon } from "./icons";
  import { store } from "./contacts.svelte";

  const STYLE = "https://tiles.openfreemap.org/styles/positron";
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

  function trackGeoJSON(): GeoJSON.FeatureCollection {
    const coords = store.track.map((p) => [p.lon, p.lat]);
    const sel = store.selected;
    if (sel && coords.length) coords.push([sel.lon, sel.lat]);
    return {
      type: "FeatureCollection",
      features: coords.length > 1 ? [{ type: "Feature", geometry: { type: "LineString", coordinates: coords }, properties: {} }] : [],
    };
  }

  $effect(() => {
    const cfg = store.config;
    if (!cfg || map) return;
    map = new maplibregl.Map({
      container,
      style: STYLE,
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
      m.addSource("track", { type: "geojson", data: trackGeoJSON() });
      m.addLayer({
        id: "track",
        type: "line",
        source: "track",
        paint: { "line-color": "#4ecdc4", "line-width": 2.5, "line-opacity": 0.9 },
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
</script>

<div class="map" bind:this={container}></div>

<style>
  .map {
    position: absolute;
    inset: 0;
  }
</style>
