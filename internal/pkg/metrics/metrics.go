// Package metrics provides a tiny dependency-free Prometheus text exporter.
// It intentionally only implements counters, gauges and histograms: enough for
// RED/USE metrics without pulling an extra exporter into the demo.
package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var defaultRegistry = NewRegistry()

func Default() *Registry { return defaultRegistry }

func ResetDefault() { defaultRegistry = NewRegistry() }

type Registry struct {
	mu         sync.RWMutex
	counters   map[string]*metricFamily
	gauges     map[string]*metricFamily
	histograms map[string]*histogramFamily
}

func NewRegistry() *Registry {
	return &Registry{
		counters:   make(map[string]*metricFamily),
		gauges:     make(map[string]*metricFamily),
		histograms: make(map[string]*histogramFamily),
	}
}

type metricFamily struct {
	name   string
	help   string
	labels []string
	mu     sync.RWMutex
	values map[string]float64
}

type histogramFamily struct {
	name    string
	help    string
	labels  []string
	buckets []float64
	mu      sync.RWMutex
	series  map[string]*histogramSeries
}

type histogramSeries struct {
	counts []uint64
	sum    float64
	count  uint64
}

// Counter returns (or creates) a counter vector.
func (r *Registry) Counter(name, help string, labels ...string) *metricFamily {
	r.mu.Lock()
	defer r.mu.Unlock()
	if f, ok := r.counters[name]; ok {
		return f
	}
	f := &metricFamily{name: name, help: help, labels: labels, values: make(map[string]float64)}
	r.counters[name] = f
	return f
}

// Gauge returns (or creates) a gauge vector.
func (r *Registry) Gauge(name, help string, labels ...string) *metricFamily {
	r.mu.Lock()
	defer r.mu.Unlock()
	if f, ok := r.gauges[name]; ok {
		return f
	}
	f := &metricFamily{name: name, help: help, labels: labels, values: make(map[string]float64)}
	r.gauges[name] = f
	return f
}

// Histogram returns (or creates) a histogram vector with the given buckets.
func (r *Registry) Histogram(name, help string, buckets []float64, labels ...string) *histogramFamily {
	r.mu.Lock()
	defer r.mu.Unlock()
	if f, ok := r.histograms[name]; ok {
		return f
	}
	if len(buckets) == 0 {
		buckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}
	}
	f := &histogramFamily{name: name, help: help, labels: labels, buckets: buckets, series: make(map[string]*histogramSeries)}
	r.histograms[name] = f
	return f
}

func (f *metricFamily) Inc(labelValues ...string) { f.Add(1, labelValues...) }

func (f *metricFamily) Add(delta float64, labelValues ...string) {
	key := labelKey(labelValues...)
	f.mu.Lock()
	f.values[key] += delta
	f.mu.Unlock()
}

func (f *metricFamily) Set(value float64, labelValues ...string) {
	key := labelKey(labelValues...)
	f.mu.Lock()
	f.values[key] = value
	f.mu.Unlock()
}

func (f *histogramFamily) Observe(seconds float64, labelValues ...string) {
	key := labelKey(labelValues...)
	f.mu.Lock()
	s, ok := f.series[key]
	if !ok {
		s = &histogramSeries{counts: make([]uint64, len(f.buckets))}
		f.series[key] = s
	}
	s.sum += seconds
	s.count++
	for i, b := range f.buckets {
		if seconds <= b {
			s.counts[i]++
		}
	}
	f.mu.Unlock()
}

func labelKey(values ...string) string { return strings.Join(values, "\x00") }

// Handler renders all registered metrics in Prometheus text format.
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		r.mu.RLock()
		defer r.mu.RUnlock()

		names := make([]string, 0, len(r.counters)+len(r.gauges)+len(r.histograms))
		for n := range r.counters {
			names = append(names, n)
		}
		for n := range r.gauges {
			names = append(names, n)
		}
		for n := range r.histograms {
			names = append(names, n)
		}
		sort.Strings(names)

		for _, n := range names {
			if f, ok := r.counters[n]; ok {
				writeFamily(w, f, "counter")
				continue
			}
			if f, ok := r.gauges[n]; ok {
				writeFamily(w, f, "gauge")
				continue
			}
			if f, ok := r.histograms[n]; ok {
				writeHistogram(w, f)
			}
		}
	})
}

func writeFamily(w http.ResponseWriter, f *metricFamily, typ string) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n", f.name, f.help, f.name, typ)
	f.mu.RLock()
	defer f.mu.RUnlock()
	keys := sortedKeys(f.values)
	for _, k := range keys {
		fmt.Fprintf(w, "%s%s %s\n", f.name, formatLabels(f.labels, k), formatFloat(f.values[k]))
	}
}

func writeHistogram(w http.ResponseWriter, f *histogramFamily) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s histogram\n", f.name, f.help, f.name)
	f.mu.RLock()
	defer f.mu.RUnlock()
	keys := make([]string, 0, len(f.series))
	for k := range f.series {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		s := f.series[k]
		for i, b := range f.buckets {
			fmt.Fprintf(w, "%s_bucket%s %d\n", f.name, formatLabelsWith(f.labels, k, "le", formatFloat(b)), s.counts[i])
		}
		fmt.Fprintf(w, "%s_bucket%s %d\n", f.name, formatLabelsWith(f.labels, k, "le", "+Inf"), s.count)
		fmt.Fprintf(w, "%s_sum%s %s\n", f.name, formatLabels(f.labels, k), formatFloat(s.sum))
		fmt.Fprintf(w, "%s_count%s %d\n", f.name, formatLabels(f.labels, k), s.count)
	}
}

func formatLabels(names []string, key string) string {
	return formatLabelsWith(names, key, "", "")
}

func formatLabelsWith(names []string, key, extraName, extraValue string) string {
	values := strings.Split(key, "\x00")
	pairs := make([]string, 0, len(names)+1)
	for i, n := range names {
		v := ""
		if i < len(values) {
			v = values[i]
		}
		pairs = append(pairs, fmt.Sprintf(`%s="%s"`, n, escapeLabel(v)))
	}
	if extraName != "" {
		pairs = append(pairs, fmt.Sprintf(`%s="%s"`, extraName, escapeLabel(extraValue)))
	}
	if len(pairs) == 0 {
		return ""
	}
	return "{" + strings.Join(pairs, ",") + "}"
}

func escapeLabel(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return strings.ReplaceAll(s, `"`, `\"`)
}

func formatFloat(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ObserveDuration is a helper for timing blocks.
func ObserveDuration(f *histogramFamily, start time.Time, labelValues ...string) {
	f.Observe(time.Since(start).Seconds(), labelValues...)
}
