// Package metrics manages metric stores and Prometheus metric generation.
package metrics

import (
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"ubnt-exporter/internal/definitions"
)

type metricRef struct {
	metricKey string
	compKey   string
}

// MetricSink defines the interface for emitting metrics during device collection.
type MetricSink interface {
	UpdateMetric(key string, labels map[string]string, value float64)
}

// MetricEntry represents an individual metric data point before commitment.
type MetricEntry struct {
	Key    string
	Labels map[string]string
	Value  float64
}

// Buffer collects metrics in memory for a single device scrape to minimize lock contention.
type Buffer struct {
	entries []MetricEntry
}

// NewBuffer constructs a new MetricBuffer preallocated for typical device metrics.
func NewBuffer() *Buffer {
	return &Buffer{
		entries: make([]MetricEntry, 0, 128),
	}
}

// UpdateMetric buffers a metric update in memory without acquiring global locks.
func (b *Buffer) UpdateMetric(key string, labels map[string]string, value float64) {
	b.entries = append(b.entries, MetricEntry{
		Key:    key,
		Labels: labels,
		Value:  value,
	})
}

// Commit flushes the buffer to the store in a single atomic lock operation.
func (b *Buffer) Commit(store *Store, devName, devIP string) {
	if store == nil {
		return
	}
	store.ReplaceDeviceMetrics(devName, devIP, b.entries)
}

// Store holds current metric values in memory in a thread-safe manner.
type Store struct {
	mu             sync.RWMutex
	defs           map[string]definitions.MetricDefinition
	descs          map[string]*prometheus.Desc
	data           map[string]map[string]float64
	labelsMap      map[string]map[string][]string
	deviceRefs     map[string][]metricRef
	ScrapeDuration *prometheus.HistogramVec
}

var globalStore *Store
var storeOnce sync.Once

// GetStore returns the singleton metric store.
func GetStore() *Store {
	storeOnce.Do(func() {
		defsCopy := make(map[string]definitions.MetricDefinition, len(definitions.MetricsDef))
		descsMap := make(map[string]*prometheus.Desc, len(definitions.MetricsDef))
		for k, v := range definitions.MetricsDef {
			lbls := make([]string, len(v.Labels))
			copy(lbls, v.Labels)
			def := definitions.MetricDefinition{
				Type:   v.Type,
				Name:   v.Name,
				Help:   v.Help,
				Labels: lbls,
			}
			defsCopy[k] = def
			descsMap[k] = prometheus.NewDesc(def.Name, def.Help, def.Labels, nil)
		}

		hist := prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "ubnt_scrape_duration_seconds",
				Help:    "Time taken to scrape the device (latency distribution)",
				Buckets: []float64{0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
			},
			[]string{"name", "ip"},
		)

		globalStore = &Store{
			defs:           defsCopy,
			descs:          descsMap,
			data:           make(map[string]map[string]float64),
			labelsMap:      make(map[string]map[string][]string),
			deviceRefs:     make(map[string][]metricRef),
			ScrapeDuration: hist,
		}
	})
	return globalStore
}

// RegisterDynamicLabels appends dynamic label keys to device_info definition.
func (s *Store) RegisterDynamicLabels(dynamicKeys []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	def, ok := s.defs["device_info"]
	if !ok {
		return
	}

	existing := make(map[string]bool)
	for _, l := range def.Labels {
		existing[l] = true
	}

	for _, k := range dynamicKeys {
		if !existing[k] {
			def.Labels = append(def.Labels, k)
			existing[k] = true
		}
	}
	s.defs["device_info"] = def
	s.descs["device_info"] = prometheus.NewDesc(def.Name, def.Help, def.Labels, nil)
}

func getDeviceKey(labels map[string]string) (string, string) {
	name := labels["name"]
	if name == "" {
		name = labels["ap_name"]
	}
	ip := labels["ip"]
	if ip == "" {
		ip = labels["ap_ip"]
	}
	return name, ip
}

func (s *Store) updateMetricLocked(key string, labels map[string]string, value float64) {
	def, exists := s.defs[key]
	if !exists {
		return
	}

	labelValues := make([]string, len(def.Labels))
	for i, lk := range def.Labels {
		labelValues[i] = labels[lk]
	}
	compKey := strings.Join(labelValues, "\x00")

	if s.data[key] == nil {
		s.data[key] = make(map[string]float64)
		s.labelsMap[key] = make(map[string][]string)
	}

	s.data[key][compKey] = value
	s.labelsMap[key][compKey] = labelValues

	devName, devIP := getDeviceKey(labels)
	if devName != "" && devIP != "" {
		refKey := devName + ":" + devIP
		s.deviceRefs[refKey] = append(s.deviceRefs[refKey], metricRef{
			metricKey: key,
			compKey:   compKey,
		})
	}
}

// UpdateMetric sets or updates a metric value for the given key and label set.
func (s *Store) UpdateMetric(key string, labels map[string]string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateMetricLocked(key, labels, value)
}

// IncrementMetric adds amount to a counter metric.
func (s *Store) IncrementMetric(key string, labels map[string]string, amount float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	def, exists := s.defs[key]
	if !exists {
		return
	}

	labelValues := make([]string, len(def.Labels))
	for i, lk := range def.Labels {
		labelValues[i] = labels[lk]
	}
	compKey := strings.Join(labelValues, "\x00")

	if s.data[key] == nil {
		s.data[key] = make(map[string]float64)
		s.labelsMap[key] = make(map[string][]string)
	}

	current := s.data[key][compKey]
	s.data[key][compKey] = current + amount
	s.labelsMap[key][compKey] = labelValues

	devName, devIP := getDeviceKey(labels)
	if devName != "" && devIP != "" {
		refKey := devName + ":" + devIP
		s.deviceRefs[refKey] = append(s.deviceRefs[refKey], metricRef{
			metricKey: key,
			compKey:   compKey,
		})
	}
}

func (s *Store) removeDeviceMetricsLocked(name, ip string) {
	refKey := name + ":" + ip
	refs, ok := s.deviceRefs[refKey]
	if !ok {
		return
	}

	for _, ref := range refs {
		if dataMap, ok := s.data[ref.metricKey]; ok {
			delete(dataMap, ref.compKey)
			if len(dataMap) == 0 {
				delete(s.data, ref.metricKey)
			}
		}
		if lblMap, ok := s.labelsMap[ref.metricKey]; ok {
			delete(lblMap, ref.compKey)
			if len(lblMap) == 0 {
				delete(s.labelsMap, ref.metricKey)
			}
		}
	}

	delete(s.deviceRefs, refKey)
}

// RemoveDeviceMetrics removes all metrics belonging to the specified device.
func (s *Store) RemoveDeviceMetrics(name, ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeDeviceMetricsLocked(name, ip)
}

// ReplaceDeviceMetrics atomically clears previous device metrics and writes new ones in a single lock.
func (s *Store) ReplaceDeviceMetrics(devName, devIP string, entries []MetricEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.removeDeviceMetricsLocked(devName, devIP)
	for _, e := range entries {
		s.updateMetricLocked(e.Key, e.Labels, e.Value)
	}
}

// ClearAllMetrics flushes all in-memory metrics.
func (s *Store) ClearAllMetrics() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data = make(map[string]map[string]float64)
	s.labelsMap = make(map[string]map[string][]string)
	s.deviceRefs = make(map[string][]metricRef)
}

// GetDeviceMetric retrieves the latest recorded value of a metric for a specific device.
func (s *Store) GetDeviceMetric(devName, devIP, metricKey string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metricMap, ok := s.data[metricKey]
	if !ok {
		return 0, false
	}
	labelsSub, ok := s.labelsMap[metricKey]
	if !ok {
		return 0, false
	}
	def, ok := s.defs[metricKey]
	if !ok {
		return 0, false
	}

	nameIdx := -1
	ipIdx := -1
	for i, l := range def.Labels {
		if l == "name" || l == "ap_name" {
			nameIdx = i
		}
		if l == "ip" || l == "ap_ip" {
			ipIdx = i
		}
	}

	for compKey, val := range metricMap {
		lVals := labelsSub[compKey]
		matchesName := (nameIdx == -1) || (nameIdx < len(lVals) && lVals[nameIdx] == devName)
		matchesIP := (ipIdx == -1) || (ipIdx < len(lVals) && lVals[ipIdx] == devIP)
		if matchesName && matchesIP {
			return val, true
		}
	}
	return 0, false
}

// CountMetricDataPoints returns the count of data points stored for the specified metric key.
func (s *Store) CountMetricDataPoints(metricKey string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data[metricKey])
}
