package stress

import (
	"encoding/json"
	"io"
	"sort"
	"time"
)

type Result struct {
	Code      uint16
	Timestamp time.Time
	Latency   time.Duration
	BytesOut  uint64
	BytesIn   uint64
	Error     string
}

type Results []Result

func (r Results) Encode(out io.Writer) error {
	return json.NewEncoder(out).Encode(r)
}

func (r *Results) Decode(in io.Reader) error {
	return json.NewDecoder(in).Decode(r)
}

func (r Results) Sort() Results {
	sort.Sort(r)
	return r
}

func (r Results) Len() int           { return len(r) }
func (r Results) Less(i, j int) bool { return r[i].Timestamp.Before(r[j].Timestamp) }
func (r Results) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
