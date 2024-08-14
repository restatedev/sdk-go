# Distributed tracing example

To test out distributed tracing, you can run Jaeger locally:
```shell
docker run -d --name jaeger \
    -e COLLECTOR_OTLP_ENABLED=true \
    -p 4317:4317 -p 16686:16686 \
    jaegertracing/all-in-one:1.46
```

And start the Restate server configured to send traces to Jaeger:
```shell
npx @restatedev/restate-server --tracing-endpoint http://localhost:4317
```

Finally start this example service and register it with the Restate server:
```shell
go run ./examples/otel
restate dep register http://localhost:9080
```

And you can now make invocations with `curl localhost:8080/Greeter/Greet --json '"hello"'`,
and they should appear in the [Jaeger UI](http://localhost:16686) with spans from both the
Restate server and the Go service.
