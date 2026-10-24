package config

import (
	"testing"

	"github.com/bvdwalt/spotter/internal/model"
)

func TestBBoxDerivedFromHome(t *testing.T) {
	box, err := bboxEnv("UNSET_BBOX_VAR", -26.2, 28.05, 150)
	if err != nil {
		t.Fatal(err)
	}
	if box[0][0] >= -26.2 || box[1][0] <= -26.2 || box[0][1] >= 28.05 || box[1][1] <= 28.05 {
		t.Errorf("home not inside box: %v", box)
	}
	if d := box[1][0] - box[0][0]; d < 4.9 || d > 5.1 {
		t.Errorf("150 nm should span about 5 degrees of latitude, got %v", d)
	}
}

func TestBBoxOverride(t *testing.T) {
	t.Setenv("AIS_BBOX", "-31.2, 29.5, -28.5, 33")
	box, err := bboxEnv("AIS_BBOX", 0, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if box != [2][2]float64{{-31.2, 29.5}, {-28.5, 33}} {
		t.Errorf("box = %v", box)
	}
	t.Setenv("AIS_BBOX", "-28.5,29.5,-31.2,33")
	if _, err := bboxEnv("AIS_BBOX", 0, 0, 1); err == nil {
		t.Error("inverted corners should fail")
	}
}

func TestPointsDefaultsToHome(t *testing.T) {
	home := model.Circle{Lat: -33.93, Lon: 18.45, RadiusNM: 150}
	for _, v := range []string{"", " ", ";;"} {
		t.Setenv("ADSB_POINTS", v)
		got, err := pointsEnv("ADSB_POINTS", home)
		if err != nil || len(got) != 1 || got[0] != home {
			t.Errorf("ADSB_POINTS=%q: got %v %v, want the home circle", v, got, err)
		}
	}
}

func TestPointsParse(t *testing.T) {
	t.Setenv("ADSB_POINTS", " -32.08, 21.16, 200 ; -28.16,25.97,200; ")
	got, err := pointsEnv("ADSB_POINTS", model.Circle{})
	if err != nil {
		t.Fatal(err)
	}
	want := []model.Circle{{Lat: -32.08, Lon: 21.16, RadiusNM: 200}, {Lat: -28.16, Lon: 25.97, RadiusNM: 200}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPointsRejects(t *testing.T) {
	for _, v := range []string{
		"1,2",                                 // missing radius
		"a,2,3",                               // not a number
		"-33,18,300",                          // radius over the API limit
		"-33,18,0",                            // zero radius
		"95,18,100",                           // latitude out of range
		"1,1,1;2,2,1;3,3,1;4,4,1;5,5,1;6,6,1", // more than five
	} {
		t.Setenv("ADSB_POINTS", v)
		if _, err := pointsEnv("ADSB_POINTS", model.Circle{}); err == nil {
			t.Errorf("ADSB_POINTS=%q should fail", v)
		}
	}
}
