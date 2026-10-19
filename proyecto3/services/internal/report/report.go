package report

import (
	"encoding/json"
	"errors"
	"io"
	"time"
)

type Report struct {
	Country   string `json:"country"`
	Planes    int32  `json:"warplanes_in_air"`
	Ships     int32  `json:"warships_in_water"`
	Timestamp string `json:"timestamp"`
	ID        string `json:"report_id"`
}

func (r Report) Validate() error {
	switch r.Country {
	case "USA", "RUS", "CHN", "ESP", "GTM":
	default:
		return errors.New("invalid country")
	}
	if r.Planes < 0 || r.Planes > 50 || r.Ships < 0 || r.Ships > 30 {
		return errors.New("invalid range")
	}
	if _, e := time.Parse(time.RFC3339Nano, r.Timestamp); e != nil {
		return errors.New("invalid timestamp")
	}
	if len(r.ID) == 0 || len(r.ID) > 128 {
		return errors.New("invalid report_id")
	}
	return nil
}
func Decode(src io.Reader) (Report, error) {
	var r Report
	d := json.NewDecoder(io.LimitReader(src, 4097))
	d.DisallowUnknownFields()
	if e := d.Decode(&r); e != nil {
		return r, e
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return r, errors.New("trailing JSON")
	}
	return r, r.Validate()
}
