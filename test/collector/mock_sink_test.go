package collector_test

type mockMetricSink struct {
	metrics map[string]float64
}

func newMockMetricSink() *mockMetricSink {
	return &mockMetricSink{
		metrics: make(map[string]float64),
	}
}

func (m *mockMetricSink) UpdateMetric(key string, _ map[string]string, value float64) {
	m.metrics[key] = value
}
