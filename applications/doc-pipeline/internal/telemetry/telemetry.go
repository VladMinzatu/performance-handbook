package telemetry

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func InitMetrics() (*sdkmetric.MeterProvider, error) {
	promExporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(promExporter))
	otel.SetMeterProvider(mp)
	return mp, nil
}

func GetMeter(name string) metric.Meter {
	return otel.GetMeterProvider().Meter(name)
}
