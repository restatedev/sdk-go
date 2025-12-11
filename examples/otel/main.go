package main

import (
	"context"
	"fmt"
	"log"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/server"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Greeter struct{}

func (Greeter) Greet(ctx restate.Context, message string) (string, error) {
	_, span := otel.Tracer("example-tracer").Start(ctx, "Greet")
	defer span.End()

	return fmt.Sprintf("%s!", message), nil
}

func main() {
	exporter, err := otlptrace.New(
		context.Background(),
		otlptracegrpc.NewClient(
			otlptracegrpc.WithInsecure(),
			otlptracegrpc.WithEndpoint("localhost:4317"),
		),
	)

	if err != nil {
		log.Fatalf("Could not set exporter: %v", err)
	}

	resources, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", "restate-sdk-go-otel-example-greeter"),
		),
	)
	if err != nil {
		log.Fatalf("Could not set resources: %v", err)
	}

	otel.SetTracerProvider(
		sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
			sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(exporter)),
			sdktrace.WithResource(resources),
		),
	)

	if err := server.NewRestate().
		Bind(restate.Reflect(Greeter{})).
		Start(context.Background(), ":9080"); err != nil {
		log.Fatal(err)
	}
}
