package metrics

import (
	"sort"
	"time"

	"github.com/berkantsoytas/velox/internal/requester"
)

type Report struct {
	TotalRequests int
	SuccessCount  int
	ErrorCount    int
	TotalData     int64 // read total byte
	StatusCodes   map[int]int
	Latencies     []time.Duration

	Min  time.Duration
	Max  time.Duration
	Mean time.Duration
	P50  time.Duration // medyan
	P90  time.Duration
	P95  time.Duration
	P99  time.Duration
}

func (r *Report) Add(res requester.Result) {
	r.TotalRequests++
	r.Latencies = append(r.Latencies, res.Duration)

	if res.Error != nil {
		r.ErrorCount++
		return
	}

	r.StatusCodes[res.StatusCode]++
	r.TotalData += res.BytesRead

	if res.StatusCode >= 200 && res.StatusCode < 400 {
		r.SuccessCount++
	} else {
		r.ErrorCount++
	}
}

func Collect(results <-chan requester.Result, totalRequests int) *Report {
	report := &Report{
		StatusCodes: make(map[int]int),
		Latencies:   make([]time.Duration, 0, totalRequests),
	}

	for res := range results {
		report.Add(res)
	}

	report.Finalize()

	return report
}

func (r *Report) Finalize() {
	if len(r.Latencies) == 0 {
		return
	}

	sort.Slice(r.Latencies, func(i, j int) bool {
		return r.Latencies[i] < r.Latencies[j]
	})

	var total time.Duration
	for _, l := range r.Latencies {
		total += l
	}

	r.Min = r.Latencies[0]
	r.Max = r.Latencies[len(r.Latencies)-1]
	r.Mean = total / time.Duration(len(r.Latencies))

	r.P50 = r.Latencies[int(float64(len(r.Latencies))*0.50)]
	r.P90 = r.Latencies[int(float64(len(r.Latencies))*0.90)]
	r.P95 = r.Latencies[int(float64(len(r.Latencies))*0.95)]

	idx99 := int(float64(len(r.Latencies)) * 0.99)
	if idx99 >= len(r.Latencies) {
		idx99 = len(r.Latencies) - 1
	}
	r.P99 = r.Latencies[idx99]
}

func calculateStats(r *Report) {
	if len(r.Latencies) == 0 {
		return
	}

	sort.Slice(r.Latencies, func(i, j int) bool {
		return r.Latencies[i] < r.Latencies[j]
	})

	var total time.Duration

	for _, l := range r.Latencies {
		total += l
	}

	r.Min = r.Latencies[0]
	r.Max = r.Latencies[len(r.Latencies)-1]
	r.Mean = total / time.Duration(len(r.Latencies))

	r.P50 = r.Latencies[int(float64(len(r.Latencies))*0.50)]
	r.P90 = r.Latencies[int(float64(len(r.Latencies))*0.90)]
	r.P95 = r.Latencies[int(float64(len(r.Latencies))*0.95)]

	idx99 := int(float64(len(r.Latencies)) * 0.99)
	if idx99 >= len(r.Latencies) {
		idx99 = len(r.Latencies) - 1
	}

	r.P99 = r.Latencies[idx99]
}
