package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func setupPrometheus(ctx context.Context) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		ErrorCounter,
		EvaluationLatencyHistogram,
		FliptRequestsCounter,
		FliptResultsCounter,
	)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	lc := net.ListenConfig{}
	ln, err := lc.Listen(ctx, "tcp", ":9090")
	if err != nil {
		log.Printf("Failed to create metrics listener: %v", err)
		return
	}

	server := &http.Server{
		Handler: mux,
	}

	go func() {
		log.Println("Prometheus metrics server starting on :9090")
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("Prometheus server error: %v", err)
		}
		log.Println("Prometheus initialized successfully")
	}()

	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Prometheus shutdown error: %v", err)
		}
	}()
}

var ErrorCounter = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "chaosmonkey_errors_total",
	Help: "Total number of chaosmonkey errors",
})

var EvaluationLatencyHistogram = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "flipt_evaluations_latency_milliseconds",
		Help:    "Evaluation latency in milliseconds",
		Buckets: prometheus.DefBuckets,
		ConstLabels: prometheus.Labels{
			"unit": "ms",
		},
	},
	[]string{"flipt_flag", "flipt_environment", "flipt_namespace", "instance"},
)

var FliptRequestsCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "flipt_evaluations_requests_total",
		Help: "Total number of Flipt evaluation requests",
	},
	[]string{"flipt_flag", "flipt_environment", "flipt_namespace", "instance"},
)

var FliptResultsCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "flipt_evaluations_results_total",
		Help: "Total number of Flipt evaluation results",
	},
	[]string{"flipt_flag", "flipt_environment", "flipt_namespace", "flipt_value", "flipt_reason", "flipt_flag_type", "instance"},
)
