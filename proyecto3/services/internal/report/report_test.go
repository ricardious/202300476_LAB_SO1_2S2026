package report

import (
	"strings"
	"testing"
)

func TestDecode(t *testing.T) {
	good := `{"country":"ESP","warplanes_in_air":0,"warships_in_water":30,"timestamp":"2026-10-19T12:00:00Z","report_id":"a"}`
	if _, e := Decode(strings.NewReader(good)); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{strings.Replace(good, `"ESP"`, `"GMT"`, 1), strings.Replace(good, `"warships_in_water":30`, `"warships_in_water":31`, 1), strings.Replace(good, `"warplanes_in_air":0`, `"warplanes_in_air":-1`, 1), strings.Replace(good, `"a"`, `""`, 1), good + ` {}`, strings.Replace(good, `"report_id":`, `"unexpected":1,"report_id":`, 1)} {
		if _, e := Decode(strings.NewReader(bad)); e == nil {
			t.Fatalf("accepted invalid report %s", bad)
		}
	}
}
