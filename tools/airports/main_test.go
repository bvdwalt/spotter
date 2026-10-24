package main

import (
	"strings"
	"testing"
)

const sample = `"id","ident","type","name","latitude_deg","longitude_deg","elevation_ft","continent","iso_country","iso_region","municipality","scheduled_service","icao_code","iata_code","gps_code"
1,"FACT","large_airport","Cape Town International Airport",-33.96480179,18.60169983,151,"AF","ZA","ZA-WC","Cape Town","yes","FACT","CPT",""
2,"FAGG","large_airport","George Airport",-34.0056,22.3789,648,"AF","ZA","ZA-WC","George","yes","FAGG","GRJ",""
3,"FAPN","medium_airport","Pilanesberg International Airport",-25.33,27.17,3412,"AF","ZA","ZA-NW","","yes","FAPN","NTY",""
4,"ZA-0218","small_airport","Tankwa International Airport",-32.32,19.75,1200,"AF","ZA","ZA-NC","","no","","",""
5,"FAHS","medium_airport","Hoedspruit Air Force Base",-24.37,31.05,1743,"AF","ZA","ZA-LP","","no","FAHS","HDS",""
8,"ZA-0120","small_airport","Morningstar Airfield",-33.76,18.55,200,"AF","ZA","ZA-WC","","no","","",""
9,"FADX","small_airport","Delta 200 Airstrip",-33.65,18.47,90,"AF","ZA","ZA-WC","","no","","","FADX"
6,"ZA-H1","heliport","Some Heliport",-33.9,18.4,0,"AF","ZA","ZA-WC","","no","","",""
7,"EGLL","large_airport","London Heathrow Airport",51.47,-0.46,83,"EU","GB","GB-ENG","London","yes","EGLL","LHR",""
`

func TestExtract(t *testing.T) {
	b, err := parseBBox("15,-36,34,-21")
	if err != nil {
		t.Fatal(err)
	}
	fs, err := extract(strings.NewReader(sample), b)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]properties{}
	for _, f := range fs {
		got[f.Properties.Code] = f.Properties
	}
	if len(fs) != 7 {
		t.Fatalf("want 7 airports (heliport and London excluded), got %d: %v", len(fs), got)
	}
	cases := map[string]bool{"CPT": true, "GRJ": true, "NTY": true, "": false, "HDS": false, "FADX": false}
	for code, major := range cases {
		p, ok := got[code]
		if !ok {
			t.Errorf("%s missing", code)
			continue
		}
		if p.Major != major {
			t.Errorf("%s major = %v, want %v", code, p.Major, major)
		}
	}
	if c := fs[0].Geometry.Coordinates; c != [2]float64{18.6017, -33.9648} {
		t.Errorf("first feature should be FACT with rounded coords, got %v", c)
	}
	for _, f := range fs {
		if strings.HasPrefix(f.Properties.Code, "ZA-") {
			t.Errorf("OurAirports ident leaked into a label: %+v", f.Properties)
		}
	}
}

func TestParseBBoxErrors(t *testing.T) {
	for _, s := range []string{"1,2,3", "a,b,c,d"} {
		if _, err := parseBBox(s); err == nil {
			t.Errorf("parseBBox(%q) should fail", s)
		}
	}
}
