// Command airports writes a GeoJSON file of airports inside a bounding box, from the OurAirports dataset.
package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const sourceURL = "https://davidmegginson.github.io/ourairports-data/airports.csv"

type bbox struct{ lonW, latS, lonE, latN float64 }

func (b bbox) contains(lat, lon float64) bool {
	return lon >= b.lonW && lon <= b.lonE && lat >= b.latS && lat <= b.latN
}

type feature struct {
	Type       string     `json:"type"`
	Geometry   geometry   `json:"geometry"`
	Properties properties `json:"properties"`
}

type geometry struct {
	Type        string     `json:"type"`
	Coordinates [2]float64 `json:"coordinates"`
}

type properties struct {
	Code  string `json:"code"` // IATA, then ICAO, then GPS code; empty for strips with none
	Name  string `json:"name"`
	Major bool   `json:"major"`
}

func main() {
	box := flag.String("bbox", "15,-36,34,-21", "lonW,latS,lonE,latN")
	out := flag.String("out", "web/public/airports.json", "output GeoJSON path")
	flag.Parse()

	b, err := parseBBox(*box)
	if err != nil {
		log.Fatal(err)
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(sourceURL)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("download: HTTP %d", resp.StatusCode)
	}
	features, err := extract(resp.Body, b)
	if err != nil {
		log.Fatal(err)
	}
	data, err := json.Marshal(map[string]any{"type": "FeatureCollection", "features": features})
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, append(data, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
	major := 0
	for _, f := range features {
		if f.Properties.Major {
			major++
		}
	}
	fmt.Printf("wrote %s: %d airports, %d major, %d KB\n", *out, len(features), major, len(data)/1024)
}

func parseBBox(s string) (bbox, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return bbox{}, fmt.Errorf("bbox must be lonW,latS,lonE,latN")
	}
	var v [4]float64
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return bbox{}, fmt.Errorf("bbox: %w", err)
		}
		v[i] = f
	}
	return bbox{v[0], v[1], v[2], v[3]}, nil
}

// extract keeps large, medium, and small airports inside the box, sorted by ident for stable diffs.
// Major means a large airport, or a medium one named International; "large" alone would miss Maputo-style
// naming and a name match alone would catch desert strips like Tankwa International.
func extract(r io.Reader, b bbox) ([]feature, error) {
	cr := csv.NewReader(r)
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	col := map[string]int{}
	for i, h := range header {
		col[h] = i
	}
	for _, need := range []string{"ident", "type", "name", "latitude_deg", "longitude_deg", "iata_code", "icao_code", "gps_code"} {
		if _, ok := col[need]; !ok {
			return nil, fmt.Errorf("missing column %q", need)
		}
	}
	type row struct {
		ident string
		f     feature
	}
	var rows []row
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}
		kind := rec[col["type"]]
		if kind != "large_airport" && kind != "medium_airport" && kind != "small_airport" {
			continue
		}
		lat, err1 := strconv.ParseFloat(rec[col["latitude_deg"]], 64)
		lon, err2 := strconv.ParseFloat(rec[col["longitude_deg"]], 64)
		if err1 != nil || err2 != nil || !b.contains(lat, lon) {
			continue
		}
		name := rec[col["name"]]
		code := firstCode(rec[col["iata_code"]], rec[col["icao_code"]], rec[col["gps_code"]])
		major := kind == "large_airport" || (kind == "medium_airport" && strings.Contains(strings.ToLower(name), "international"))
		rows = append(rows, row{rec[col["ident"]], feature{
			Type:       "Feature",
			Geometry:   geometry{Type: "Point", Coordinates: [2]float64{round4(lon), round4(lat)}},
			Properties: properties{Code: code, Name: name, Major: major},
		}})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ident < rows[j].ident })
	out := make([]feature, len(rows))
	for i, r := range rows {
		out[i] = r.f
	}
	return out, nil
}

func round4(f float64) float64 { return math.Round(f*1e4) / 1e4 }

// firstCode returns the first non-empty code; OurAirports' own idents like ZA-0120 are never used as labels.
func firstCode(codes ...string) string {
	for _, c := range codes {
		if c = strings.TrimSpace(c); c != "" {
			return c
		}
	}
	return ""
}
