package config

import "testing"

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
