package otelpgx

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "github.com/siabroo/micra/adapters/otelpgx"

type config struct {
	application      string
	routeFromContext func(context.Context) string
	tracerProvider   trace.TracerProvider
}

// Option configures a Pool wrapper via WrapPool.
type Option func(*config)

func defaults() config {
	return config{
		routeFromContext: routeFromBaggage,
		tracerProvider:   nil, // resolved lazily to otel.GetTracerProvider()
	}
}

// WithApplication sets the static "application" field in sqlcommenter
// output. Usually the service name. Omitted if empty.
func WithApplication(name string) Option {
	return func(c *config) { c.application = name }
}

// WithRouteFromContext overrides the default route extractor. The
// default reads OTel baggage key "micra.route".
func WithRouteFromContext(fn func(context.Context) string) Option {
	return func(c *config) {
		if fn != nil {
			c.routeFromContext = fn
		}
	}
}

// WithTracerProvider overrides the global otel.GetTracerProvider() for
// span emission. Useful in tests; production services configure the
// global provider via adapters/otelinit and leave this unset.
func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(c *config) { c.tracerProvider = tp }
}

func (c *config) tracer() trace.Tracer {
	tp := c.tracerProvider
	if tp == nil {
		tp = otel.GetTracerProvider()
	}
	return tp.Tracer(instrumentationName)
}

// before is the shared hot-path entrypoint used by Pool.Query/QueryRow/
// Exec and Tx.Query/QueryRow/Exec. It builds the sqlcommenter prefix,
// prepends it to sql, starts a CLIENT span carrying semconv DB
// attributes, and returns the rewritten sql, span ctx, and an end func.
func (c *config) before(ctx context.Context, sql, spanName string) (string, context.Context, func(error)) {
	fields := commentFields{
		application: c.application,
		route:       c.routeFromContext(ctx),
		traceparent: traceparentFromContext(ctx),
	}
	if prefix := encode(fields); prefix != "" {
		sql = prefix + sql
	}
	ctx, span := c.tracer().Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			attribute.String("db.statement", sql),
		),
	)
	return sql, ctx, func(err error) {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, err.Error())
		}
		span.End()
	}
}
