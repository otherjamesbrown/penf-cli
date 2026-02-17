package db

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNewPoolStatsCollector(t *testing.T) {
	collector := NewPoolStatsCollector(nil, "test", "test-service")

	if collector == nil {
		t.Fatal("expected collector to be created")
	}

	if collector.totalConns == nil {
		t.Error("totalConns descriptor should not be nil")
	}
	if collector.idleConns == nil {
		t.Error("idleConns descriptor should not be nil")
	}
	if collector.acquiredConns == nil {
		t.Error("acquiredConns descriptor should not be nil")
	}
	if collector.maxConns == nil {
		t.Error("maxConns descriptor should not be nil")
	}
}

func TestPoolStatsCollector_Describe(t *testing.T) {
	collector := NewPoolStatsCollector(nil, "test", "test-service")

	ch := make(chan *prometheus.Desc, 10)
	go func() {
		collector.Describe(ch)
		close(ch)
	}()

	var descs []*prometheus.Desc
	for desc := range ch {
		descs = append(descs, desc)
	}

	if len(descs) != 4 {
		t.Errorf("expected 4 descriptors, got %d", len(descs))
	}

	// Verify metric names
	expectedNames := []string{
		"test_db_pool_total_conns",
		"test_db_pool_idle_conns",
		"test_db_pool_acquired_conns",
		"test_db_pool_max_conns",
	}

	for i, desc := range descs {
		descStr := desc.String()
		if !strings.Contains(descStr, expectedNames[i]) {
			t.Errorf("expected descriptor to contain %s, got %s", expectedNames[i], descStr)
		}
	}
}

func TestPoolStatsCollector_Collect_NilPool(t *testing.T) {
	collector := NewPoolStatsCollector(nil, "test", "test-service")

	ch := make(chan prometheus.Metric, 10)
	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	var metrics []prometheus.Metric
	for m := range ch {
		metrics = append(metrics, m)
	}

	// Should return no metrics when pool is nil
	if len(metrics) != 0 {
		t.Errorf("expected 0 metrics for nil pool, got %d", len(metrics))
	}
}

func TestRegisterPoolStatsCollectorWithRegistry(t *testing.T) {
	reg := prometheus.NewRegistry()

	collector, err := RegisterPoolStatsCollectorWithRegistry(nil, "test", "test-service", reg)
	if err != nil {
		t.Fatalf("RegisterPoolStatsCollectorWithRegistry failed: %v", err)
	}

	if collector == nil {
		t.Fatal("expected collector to be returned")
	}

	// Verify registration by gathering
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}

	// With nil pool, Collect returns no metrics, so families may be empty
	// This is expected behavior - we just verify no error on gather
	_ = families
}

func TestRegisterPoolStatsCollectorWithRegistry_DoubleRegister(t *testing.T) {
	reg := prometheus.NewRegistry()

	_, err := RegisterPoolStatsCollectorWithRegistry(nil, "test", "test-service", reg)
	if err != nil {
		t.Fatalf("First registration failed: %v", err)
	}

	// Second registration should not return an error (already registered is ignored)
	_, err = RegisterPoolStatsCollectorWithRegistry(nil, "test", "test-service", reg)
	if err != nil {
		t.Fatalf("Second registration should not error: %v", err)
	}
}

func TestPoolStatsCollector_MetricLabels(t *testing.T) {
	collector := NewPoolStatsCollector(nil, "penfold", "gateway")

	ch := make(chan *prometheus.Desc, 10)
	go func() {
		collector.Describe(ch)
		close(ch)
	}()

	for desc := range ch {
		descStr := desc.String()
		// Verify service label is present
		if !strings.Contains(descStr, "service=\"gateway\"") {
			t.Errorf("expected service label 'gateway' in descriptor, got %s", descStr)
		}
		// Verify namespace prefix
		if !strings.Contains(descStr, "fqName: \"penfold_db_pool_") {
			t.Errorf("expected 'penfold_db_pool_' prefix in descriptor, got %s", descStr)
		}
	}
}

// TestPoolStatsCollector_WithLintCheck verifies the collector passes prometheus lint checks
func TestPoolStatsCollector_WithLintCheck(t *testing.T) {
	collector := NewPoolStatsCollector(nil, "test", "test-service")

	// Use testutil.CollectAndLint to validate the collector
	problems, err := testutil.CollectAndLint(collector)
	if err != nil {
		t.Fatalf("CollectAndLint failed: %v", err)
	}

	for _, p := range problems {
		t.Errorf("lint problem: %s", p.Text)
	}
}
