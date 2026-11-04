package middlewares

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

func OTelMiddleware(serviceName string) echo.MiddlewareFunc {
	tracer := otel.Tracer(serviceName)
	propagator := otel.GetTextMapPropagator()

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			ctx := propagator.Extract(req.Context(), propagation.HeaderCarrier(req.Header))
			spanName := req.Method + " " + c.Path()

			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPMethod(req.Method),
					semconv.HTTPURL(req.URL.String()),
					semconv.NetHostName(req.Host),
					attribute.String("http.route", c.Path()),
				),
			)
			defer span.End()

			c.SetRequest(req.WithContext(ctx))

			err := next(c)

			if res, unwrapErr := echo.UnwrapResponse(c.Response()); unwrapErr == nil {
				status := res.Status
				span.SetAttributes(semconv.HTTPStatusCode(status))
				if status >= http.StatusInternalServerError {
					span.SetStatus(codes.Error, http.StatusText(status))
				}
			}

			return err
		}
	}
}
