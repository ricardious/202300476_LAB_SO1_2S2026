package store

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/redis/go-redis/v9"
	"net/http"
	"so1/proyecto3/internal/report"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed apply.lua
var script string

const Prefix = "so1:p3:"

func Apply(ctx context.Context, c *redis.Client, r report.Report) (bool, error) {
	t, e := time.Parse(time.RFC3339Nano, r.Timestamp)
	if e != nil {
		return false, e
	}
	n, e := c.Eval(ctx, script, []string{Prefix}, r.ID, r.Country, r.Planes, r.Ships, r.Timestamp, t.UnixNano()/1e6).Int()
	return n == 1, e
}
func mode(hist map[string]string) (int, []string) {
	max := int64(-1)
	selected := 0
	ties := []string{}
	for s, count := range hist {
		n, _ := strconv.Atoi(s)
		v, _ := strconv.ParseInt(count, 10, 64)
		if v > max {
			max = v
			selected = n
			ties = []string{s}
		} else if v == max {
			ties = append(ties, s)
			if n < selected {
				selected = n
			}
		}
	}
	sort.Slice(ties, func(i, j int) bool { a, _ := strconv.Atoi(ties[i]); b, _ := strconv.Atoi(ties[j]); return a < b })
	return selected, ties
}
func Metrics(c *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		stats, e := c.HGetAll(ctx, Prefix+"stats").Result()
		if e != nil {
			http.Error(w, "storage unavailable", 503)
			return
		}
		var out strings.Builder
		fmt.Fprintln(&out, "# TYPE so1_reports_total counter")
		total := stats["total"]
		if total == "" {
			total = "0"
		}
		fmt.Fprintln(&out, "so1_reports_total", total)
		if total != "0" {
			for _, name := range []string{"planes_min", "planes_max", "ships_min", "ships_max"} {
				fmt.Fprintf(&out, "so1_%s %s\n", name, stats[name])
			}
			for _, kind := range []string{"planes", "ships"} {
				hist, err := c.HGetAll(ctx, Prefix+kind+":hist").Result()
				if err != nil {
					http.Error(w, "storage unavailable", 503)
					return
				}
				v, ties := mode(hist)
				fmt.Fprintf(&out, "so1_%s_mode{ties=%q} %d\n", kind, strings.Join(ties, ","), v)
				rank, err := c.ZRangeWithScores(ctx, Prefix+kind+":rank", 0, -1).Result()
				if err != nil {
					http.Error(w, "storage unavailable", 503)
					return
				}
				for _, x := range rank {
					fmt.Fprintf(&out, "so1_%s_sum{country=%q} %g\n", kind, x.Member, x.Score)
				}
			}
		}
		for _, country := range []string{"USA", "RUS", "CHN", "ESP", "GTM"} {
			n := stats["country:"+country]
			if n == "" {
				n = "0"
			}
			fmt.Fprintf(&out, "so1_country_reports_total{country=%q} %s\n", country, n)
			latest, err := c.HGetAll(ctx, Prefix+"latest:"+country).Result()
			if err != nil {
				http.Error(w, "storage unavailable", 503)
				return
			}
			for _, kind := range []string{"planes", "ships"} {
				if v, ok := latest[kind]; ok {
					fmt.Fprintf(&out, "so1_%s_latest{country=%q} %s\n", kind, country, v)
				}
			}
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprint(w, out.String())
	}
}
