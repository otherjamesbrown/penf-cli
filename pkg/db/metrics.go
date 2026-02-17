// Package db provides shared PostgreSQL database utilities for Penfold microservices.
package db

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// PoolStatsCollector collects connection pool statistics as Prometheus metrics.
// It implements prometheus.Collector interface and reads stats directly from the pool
// on each scrape, ensuring up-to-date values.
type PoolStatsCollector struct {
	pool *pgxpool.Pool

	// Metric descriptors
	totalConns    *prometheus.Desc
	idleConns     *prometheus.Desc
	acquiredConns *prometheus.Desc
	maxConns      *prometheus.Desc
}

// NewPoolStatsCollector creates a new collector for the given connection pool.
// The serviceName is used as a label to distinguish between multiple services.
func NewPoolStatsCollector(pool *pgxpool.Pool, namespace, serviceName string) *PoolStatsCollector {
	constLabels := prometheus.Labels{"service": serviceName}

	return &PoolStatsCollector{
		pool: pool,
		totalConns: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "db_pool", "total_conns"),
			"Total number of connections currently open in the pool",
			nil,
			constLabels,
		),
		idleConns: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "db_pool", "idle_conns"),
			"Number of idle connections in the pool",
			nil,
			constLabels,
		),
		acquiredConns: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "db_pool", "acquired_conns"),
			"Number of connections currently acquired from the pool",
			nil,
			constLabels,
		),
		maxConns: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "db_pool", "max_conns"),
			"Maximum number of connections allowed in the pool",
			nil,
			constLabels,
		),
	}
}

// Describe sends all metric descriptors to the channel.
func (c *PoolStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.totalConns
	ch <- c.idleConns
	ch <- c.acquiredConns
	ch <- c.maxConns
}

// Collect gathers current pool statistics and sends them as metrics.
func (c *PoolStatsCollector) Collect(ch chan<- prometheus.Metric) {
	if c.pool == nil {
		return
	}

	stats := c.pool.Stat()

	ch <- prometheus.MustNewConstMetric(
		c.totalConns,
		prometheus.GaugeValue,
		float64(stats.TotalConns()),
	)
	ch <- prometheus.MustNewConstMetric(
		c.idleConns,
		prometheus.GaugeValue,
		float64(stats.IdleConns()),
	)
	ch <- prometheus.MustNewConstMetric(
		c.acquiredConns,
		prometheus.GaugeValue,
		float64(stats.AcquiredConns()),
	)
	ch <- prometheus.MustNewConstMetric(
		c.maxConns,
		prometheus.GaugeValue,
		float64(stats.MaxConns()),
	)
}

// RegisterPoolStatsCollector creates and registers a pool stats collector with the
// default Prometheus registry. Returns the collector for potential unregistration.
func RegisterPoolStatsCollector(pool *pgxpool.Pool, namespace, serviceName string) (*PoolStatsCollector, error) {
	collector := NewPoolStatsCollector(pool, namespace, serviceName)
	if err := prometheus.Register(collector); err != nil {
		// If already registered, that's acceptable
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			return nil, err
		}
	}
	return collector, nil
}

// RegisterPoolStatsCollectorWithRegistry creates and registers a pool stats collector
// with a specific Prometheus registry. Useful for testing or custom registries.
func RegisterPoolStatsCollectorWithRegistry(pool *pgxpool.Pool, namespace, serviceName string, reg *prometheus.Registry) (*PoolStatsCollector, error) {
	collector := NewPoolStatsCollector(pool, namespace, serviceName)
	if err := reg.Register(collector); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			return nil, err
		}
	}
	return collector, nil
}
