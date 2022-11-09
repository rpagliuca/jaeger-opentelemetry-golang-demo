package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	tp, tpErr := JaegerTraceProvider()
	if tpErr != nil {
		log.Fatal(tpErr)
	}
	otel.SetTracerProvider(tp)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, span := otel.Tracer("").Start(context.Background(), "HandleFuncRoot")
		defer span.End()

		// Calling app2
		req, _ := http.NewRequest("GET", "http://app2:8080", nil)
		addSpanContextToHeader(span.SpanContext(), req.Header)
		client := http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}

		// Print something
		fmt.Fprintf(w, "Response code after app1 -> app2: %d", resp.StatusCode)
	})
	http.HandleFunc("/other", func(w http.ResponseWriter, r *http.Request) {
		_, span := otel.Tracer("").Start(addSpanContextToContext(context.Background(), r.Header), "HandleFuncOther")
		defer span.End()
	})
	log.Println("Starting app1 on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func JaegerTraceProvider() (*sdktrace.TracerProvider, error) {
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint("http://jaeger:14268/api/traces")))
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("app1"),
			semconv.DeploymentEnvironmentKey.String("dev"),
		)),
	)
	return tp, nil
}

func addSpanContextToHeader(spanContext trace.SpanContext, header http.Header) {
	header.Add("SPAN_ID", spanContext.SpanID().String())
	header.Add("TRACE_ID", spanContext.TraceID().String())
	header.Add("TRACE_FLAGS", spanContext.TraceFlags().String())
}

func addSpanContextToContext(ctx context.Context, header http.Header) context.Context {
	traceId, _ := trace.TraceIDFromHex(header.Get("TRACE_ID"))
	spanId, _ := trace.SpanIDFromHex(header.Get("SPAN_ID"))
	traceFlags := header.Get("TRACE_FLAGS")
	decodedTraceFlags, err := hex.DecodeString(traceFlags)
	if err != nil {
		panic(err)
	}

	var spanContextConfig trace.SpanContextConfig
	spanContextConfig.TraceID = traceId
	spanContextConfig.SpanID = spanId
	spanContextConfig.TraceFlags = trace.TraceFlags(decodedTraceFlags[0])
	spanContext := trace.NewSpanContext(spanContextConfig)
	return trace.ContextWithSpanContext(ctx, spanContext)
}
