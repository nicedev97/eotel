package eotel

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"os"
	"sort"
	"time"
)

type loggerCtxKey struct{}

type Exporter interface {
	Send(level string, msg string, traceID string, spanID string)
	CaptureError(err error, tags map[string]string, extras map[string]any)
}

type Logger interface {
	Info(msg string)
	Error(msg string)
	Debug(msg string)
	Warn(msg string)
	Fatal(msg string)

	WithField(key string, value any) Logger
	WithFields(map[string]any) Logger
	WithError(err error) Logger
	WithTracer(name string, fn func(ctx context.Context))
	SpanEvent(name string, attrs ...attribute.KeyValue)
	SetSpanAttr(key string, value any)
	SetSpanError(err error)
	Child(name string) Logger
	Ctx() context.Context
	Start(name string) Timer

	Inject(ctx context.Context, logger Logger) context.Context
	FromContext(ctx context.Context, name string) Logger
	FromGin(c *gin.Context, name string) Logger
	InjectToGin(c *gin.Context, logger Logger)
	RecoverPanic(c *gin.Context) func()
}

type Timer interface {
	Stop()
}

type Eotel struct {
	ctx          context.Context
	logger       *zap.Logger
	tracer       trace.Tracer
	meter        metric.Meter
	span         trace.Span
	logCounter   metric.Int64Counter
	durationHist metric.Float64Histogram
	fields       []zap.Field
	attrs        []attribute.KeyValue
	err          error
	name         string
	start        time.Time
	exporter     Exporter
}

func New(ctx context.Context, name string) Logger {
	meter := otel.Meter(globalCfg.ServiceName)
	logCounter, durationHist := initMetrics(meter)
	return &Eotel{
		ctx:          ctx,
		logger:       zap.L(),
		tracer:       otel.Tracer(globalCfg.ServiceName),
		meter:        meter,
		logCounter:   logCounter,
		durationHist: durationHist,
		start:        time.Now(),
		exporter:     defaultExporter{},
		name:         name,
	}
}

func (l *Eotel) Inject(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey{}, logger)
}

func (l *Eotel) FromContext(ctx context.Context, name string) Logger {
	if val := ctx.Value(loggerCtxKey{}); val != nil {
		if lg, ok := val.(*Eotel); ok {
			return lg
		}
	}
	return New(ctx, name)
}

func (l *Eotel) FromGin(c *gin.Context, name string) Logger {
	return l.FromContext(c.Request.Context(), name)
}

func (l *Eotel) InjectToGin(c *gin.Context, logger Logger) {
	ctx := l.Inject(c.Request.Context(), logger)
	c.Request = c.Request.WithContext(ctx)
}

func (l *Eotel) RecoverPanic(c *gin.Context) func() {
	return func() {
		if rec := recover(); rec != nil {
			err := fmt.Errorf("panic: %v", rec)

			log := l.FromGin(c, "panic")
			if log == nil {
				log = l
			}

			log.WithError(err).Error("unhandled panic")
			c.AbortWithStatus(500)
		}
	}
}

func (l *Eotel) Info(msg string)  { l.log("info", msg) }
func (l *Eotel) Error(msg string) { l.log("error", msg) }
func (l *Eotel) Debug(msg string) { l.log("debug", msg) }
func (l *Eotel) Warn(msg string)  { l.log("warn", msg) }
func (l *Eotel) Fatal(msg string) {
	l.log("fatal", msg)
	if l.span != nil {
		l.span.End()
	}
	os.Exit(1)
}

func (l *Eotel) log(level, msg string) {
	l.startSpanIfNeeded()
	sc := l.span.SpanContext()
	traceID := sc.TraceID().String()

	fields := append([]zap.Field{
		zap.String("trace_id", traceID),
		zap.String("span_id", sc.SpanID().String()),
		zap.String("job", globalCfg.JobName),
		zap.String("service", globalCfg.ServiceName),
		zap.String("level", level),
	}, l.fields...)

	switch level {
	case "info":
		l.logger.Info(msg, fields...)
	case "error":
		l.logger.Error(msg, fields...)
	case "debug":
		l.logger.Debug(msg, fields...)
	case "warn":
		l.logger.Warn(msg, fields...)
	case "fatal":
		l.logger.Fatal(msg, fields...)
	}

	if globalCfg.EnableLoki {
		l.exporter.Send(level, msg, traceID, sc.SpanID().String())
	}

	l.endSpan(msg, level)
}

func (l *Eotel) WithField(key string, value any) Logger {
	l.fields = append(l.fields, zap.Any(key, value))
	l.attrs = append(l.attrs, attribute.String(key, fmt.Sprintf("%v", value)))
	return l
}

func (l *Eotel) WithFields(m map[string]any) Logger {
	for k, v := range m {
		l.WithField(k, v)
	}
	return l
}

func (l *Eotel) WithError(err error) Logger {
	if err != nil {
		l.err = err
		l.fields = append(l.fields, zap.Error(err))
		l.attrs = append(l.attrs, attribute.String("error", err.Error()))
		l.exporter.CaptureError(err, map[string]string{}, map[string]any{"error": err.Error()})
	}
	return l
}

func (l *Eotel) WithTracer(name string, fn func(ctx context.Context)) {
	ctx, span := l.tracer.Start(l.ctx, name)
	defer span.End()
	fn(ctx)
}

func (l *Eotel) SpanEvent(name string, attrs ...attribute.KeyValue) {
	if l.span != nil {
		l.span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

func (l *Eotel) SetSpanAttr(key string, value any) {
	if l.span != nil {
		l.span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", value)))
	}
}

func (l *Eotel) SetSpanError(err error) {
	if err != nil && l.span != nil {
		l.span.RecordError(err)
	}
}

func (l *Eotel) Child(name string) Logger {
	ctx, span := l.tracer.Start(l.ctx, name)
	return &Eotel{
		ctx:      ctx,
		span:     span,
		logger:   l.logger,
		tracer:   l.tracer,
		meter:    l.meter,
		start:    time.Now(),
		exporter: l.exporter,
	}
}

func (l *Eotel) Ctx() context.Context {
	return l.ctx
}

func (l *Eotel) Start(name string) Timer {
	start := time.Now()
	return &eotelTimer{name: name, logger: l, start: start}
}

type eotelTimer struct {
	name   string
	logger Logger
	start  time.Time
}

func (t *eotelTimer) Stop() {
	duration := time.Since(t.start).Seconds() * 1000
	t.logger.SpanEvent(t.name, attribute.Float64("custom.duration_ms", duration))
}

func (l *Eotel) startSpanIfNeeded() {
	if l.span == nil {
		l.ctx, l.span = l.tracer.Start(l.ctx, l.name)
	}
}

func (l *Eotel) endSpan(msg, level string) {
	durationMs := time.Since(l.start).Seconds() * 1000
	l.attrs = append(l.attrs,
		attribute.String("log.message", msg),
		attribute.String("log.level", level),
		attribute.Float64("duration_ms", durationMs),
	)
	sort.SliceStable(l.attrs, func(i, j int) bool {
		return string(l.attrs[i].Key) < string(l.attrs[j].Key)
	})

	if l.span != nil {
		l.span.SetAttributes(l.attrs...)
		if l.err != nil {
			l.span.RecordError(l.err)
		}
		l.span.End()
	}

	l.logCounter.Add(l.ctx, 1, metric.WithAttributes(attribute.String("level", level)))
	l.durationHist.Record(l.ctx, durationMs, metric.WithAttributes(attribute.String("level", level)))
}

func initMetrics(m metric.Meter) (metric.Int64Counter, metric.Float64Histogram) {
	c, _ := m.Int64Counter("log_total")
	h, _ := m.Float64Histogram("log_duration_ms")
	return c, h
}

type defaultExporter struct{}

func (d defaultExporter) Send(level string, msg string, traceID string, spanID string) {
	logChan <- LokiEntry{
		Labels: map[string]string{
			"level":    level,
			"job":      globalCfg.JobName,
			"service":  globalCfg.ServiceName,
			"trace_id": traceID,
			"span_id":  spanID,
		},
		Message: msg,
	}
}

func (d defaultExporter) CaptureError(err error, tags map[string]string, extras map[string]any) {
	CaptureError(err, tags, extras)
}
