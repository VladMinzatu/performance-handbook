package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

type TelemetryMetrics struct {
	// Load Generator metrics
	numDocumentsCounter metric.Int64Counter
	textSizeHistogram   metric.Int64Histogram

	// Stage metrics
	processingLatencyHistogram metric.Int64Histogram
}

func (t *TelemetryMetrics) IncDataLoadingRequests(ctx context.Context, n int64) {
	t.numDocumentsCounter.Add(ctx, n)
}

func (t *TelemetryMetrics) RecordDataLoadingRequestTextSize(ctx context.Context, size int64) {
	t.textSizeHistogram.Record(ctx, int64(size))
}

func (t *TelemetryMetrics) RecordProcessingLatency(ctx context.Context, latency time.Duration) {
	t.processingLatencyHistogram.Record(ctx, latency.Milliseconds())
}

func InitMetrics() (*TelemetryMetrics, error) {
	promExporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(promExporter))
	otel.SetMeterProvider(mp)

	meter := otel.GetMeterProvider().Meter("load_generator")
	numDocumentsCounter, err := meter.Int64Counter("num_documents",
		metric.WithDescription("Number of total documents requested"),
	)
	if err != nil {
		return nil, err
	}

	textSizeHistogram, err := meter.Int64Histogram("text_size",
		metric.WithDescription("Histogram of text sizes requested"),
	)
	if err != nil {
		return nil, err
	}

	processingLatencyHistogram, err := meter.Int64Histogram("processing_latency",
		metric.WithDescription("Histogram of processing latencies"),
	)
	if err != nil {
		return nil, err
	}

	return &TelemetryMetrics{
		numDocumentsCounter:        numDocumentsCounter,
		textSizeHistogram:          textSizeHistogram,
		processingLatencyHistogram: processingLatencyHistogram,
	}, nil
}
