package collector_test

type recordedMetric struct {
	labels map[string]string
	value  float64
}

type mockMetricSink struct {
	metrics map[string]float64
	records map[string][]recordedMetric
}

func newMockMetricSink() *mockMetricSink {
	return &mockMetricSink{
		metrics: make(map[string]float64),
		records: make(map[string][]recordedMetric),
	}
}

func (m *mockMetricSink) UpdateMetric(key string, labels map[string]string, value float64) {
	m.metrics[key] = value
	copied := make(map[string]string, len(labels))
	for k, v := range labels {
		copied[k] = v
	}
	m.records[key] = append(m.records[key], recordedMetric{
		labels: copied,
		value:  value,
	})
}
