package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

type TelemetryMetrics struct {
	// Load Generator metrics
	numDocumentsCounter metric.Int64Counter
	textSizeHistogram   metric.Int64Histogram

	// Generic stage metrics
	processingLatencyHistogram      metric.Float64Histogram
	stageTotalProcessedItemsCounter metric.Int64Counter
	stageErrorsCounter              metric.Int64Counter

	// Stage-specific metrics

	// Indexing and deduplication metrics
	deduplicationThreshold             metric.Float64Gauge
	totalProcessedDocumentsForIndexing metric.Int64Counter
	totalDuplicateDocuments            metric.Int64Counter
}

func (t *TelemetryMetrics) IncDataLoadingRequests(ctx context.Context, n int64) {
	t.numDocumentsCounter.Add(ctx, n)
}

func (t *TelemetryMetrics) RecordDataLoadingRequestTextSize(ctx context.Context, size int64) {
	t.textSizeHistogram.Record(ctx, int64(size))
}

func (t *TelemetryMetrics) RecordProcessingLatency(ctx context.Context, latency time.Duration, stageName string) {
	t.processingLatencyHistogram.Record(ctx, float64(latency.Nanoseconds())/1000_000.0, metric.WithAttributes(attribute.String("stage_name", stageName)))
}

func (t *TelemetryMetrics) IncStageTotalProcessedItems(ctx context.Context, stageName string) {
	t.stageTotalProcessedItemsCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("stage_name", stageName)))
}

func (t *TelemetryMetrics) IncStageErrors(ctx context.Context, stageName string) {
	t.stageErrorsCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("stage_name", stageName)))
}

func (t *TelemetryMetrics) SetDeduplicationThreshold(ctx context.Context, threshold float64) {
	t.deduplicationThreshold.Record(ctx, threshold)
}

func (t *TelemetryMetrics) IncTotalProcessedDocumentsForIndexing(ctx context.Context) {
	t.totalProcessedDocumentsForIndexing.Add(ctx, 1)
}

func (t *TelemetryMetrics) IncTotalDuplicateDocuments(ctx context.Context) {
	t.totalDuplicateDocuments.Add(ctx, 1)
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

	processingLatencyHistogram, err := meter.Float64Histogram("processing_latency",
		metric.WithDescription("Histogram of processing latencies"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(
			0.005, // 5us
			0.01,  // 10us
			0.025, // 25us
			0.05,  // 50us
			0.1,   // 100us
			0.25,
			0.5,
			1.0,
			2.0,
			5.0,
			10.0,
			25.0,
			50.0,
			100.0,
			250.0,
			500.0,
		),
	)
	if err != nil {
		return nil, err
	}

	stageTotalProcessedItemsCounter, err := meter.Int64Counter("stage_total_processed_items",
		metric.WithDescription("Number of total items processed by stage"),
	)
	if err != nil {
		return nil, err
	}

	stageErrorsCounter, err := meter.Int64Counter("stage_errors",
		metric.WithDescription("Number of errors encountered by stage"),
	)
	if err != nil {
		return nil, err
	}

	deduplicationThreshold, err := meter.Float64Gauge("deduplication_threshold",
		metric.WithDescription("Deduplication threshold"),
	)
	if err != nil {
		return nil, err
	}

	totalProcessedDocumentsForIndexing, err := meter.Int64Counter("total_processed_documents_for_indexing",
		metric.WithDescription("Number of total documents processed for indexing"),
	)
	if err != nil {
		return nil, err
	}

	totalDuplicateDocuments, err := meter.Int64Counter("total_duplicate_documents",
		metric.WithDescription("Number of duplicate documents"),
	)
	if err != nil {
		return nil, err
	}

	return &TelemetryMetrics{
		numDocumentsCounter:                numDocumentsCounter,
		textSizeHistogram:                  textSizeHistogram,
		processingLatencyHistogram:         processingLatencyHistogram,
		stageTotalProcessedItemsCounter:    stageTotalProcessedItemsCounter,
		stageErrorsCounter:                 stageErrorsCounter,
		deduplicationThreshold:             deduplicationThreshold,
		totalProcessedDocumentsForIndexing: totalProcessedDocumentsForIndexing,
		totalDuplicateDocuments:            totalDuplicateDocuments,
	}, nil
}
