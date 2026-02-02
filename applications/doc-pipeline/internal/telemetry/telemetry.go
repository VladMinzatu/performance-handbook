package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

type TelemetryMetrics struct {
	numDocumentsCounter metric.Int64Counter
}

func (t *TelemetryMetrics) IncDataLoadingRequests(n int64) {
	t.numDocumentsCounter.Add(context.Background(), n)
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

	return &TelemetryMetrics{
		numDocumentsCounter: numDocumentsCounter,
	}, nil
}
