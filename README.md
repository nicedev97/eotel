// 📁 README.md
# Eotel - Observability Toolkit for Go

`eotel` เป็น logging, tracing และ metrics toolkit สำหรับ Go ที่รวม Zap, OpenTelemetry, Loki และ Sentry ในแพ็กเกจเดียว พร้อม middleware สำหรับ Gin

## Features

- [x] Logger แบบ structured ด้วย Zap
- [x] Tracing และ Metrics ด้วย OpenTelemetry
- [x] Async Logging ไปยัง Grafana Loki
- [x] Error Reporting ไปยัง Sentry
- [x] Gin Middleware พร้อมใช้งาน
- [x] Configurable ผ่าน `.env` หรือ `os.Getenv`

## TODO / Roadmap

- [x] context lifecycle เช่น `WithTimeout`, `WithCancel`
- [x] dynamic trace sampling จาก config/header
- [x] masking ข้อมูล sensitive เช่น token, password
- [x] custom log schema + structured field mapping
- [x] log rate limit / throttle
- [x] alert webhook เช่น Slack, Telegram
- [x] health probe / readiness probe
- [x] รองรับการ integrate กับ Zap Hook หรือ OTEL Hook

---

## การติดตั้ง

```bash
go get github.com/nicedev97/eotel@latest
```

---

## การตั้งค่า Environment

สร้างไฟล์ `.env`:

```env
SERVICE_NAME=eotel
JOB_NAME=eotel-job
LOG_LEVEL=info

OTEL_COLLECTOR=otel-collector:4317
ENABLE_TRACING=true
ENABLE_METRICS=true

ENABLE_SENTRY=true
SENTRY_DSN=https://xxxx@sentry.io/123456
SENTRY_ORG=my-org

ENABLE_LOKI=true
LOKI_URL=http://loki:3100/loki/api/v1/push
```

---
## Method Overview

| Method | Description |
|--------|-------------|
| `New(ctx, name)` | สร้าง logger ใหม่พร้อม span และ metric |
| `WithField(key, value)` | เพิ่มข้อมูลประกอบ (field + attribute) แบบ key-value |
| `WithFields(map[string]interface{})` | เพิ่ม field หลายตัวพร้อมกัน |
| `WithError(err)` | แนบ error และส่งไปยัง Sentry + span record |
| `Info()` `Error()` `Debug()` `Warn()` `Fatal()` | เขียน log พร้อม span และ metric |
| `TraceName(name)` | เปลี่ยนชื่อ span หลัก ก่อน log |
| `SpanEvent(name, attrs...)` | เพิ่ม event ลงใน span |
| `SetSpanAttr(key, value)` | เพิ่ม attribute เข้า span |
| `SetSpanError(err)` | บันทึก error ใน span |
| `Child(name)` | สร้าง logger ลูกพร้อม span ใหม่ (inherit context) |
| `InjectToGin(c)` `FromGin(c)` `FromContext(ctx)` | สำหรับ Gin / context logger tracing |
| `Start(name).Stop()` | วัดระยะเวลาเฉพาะกิจแบบ custom timer |
| `RecoverPanic()` | middleware ดัก panic และส่ง log + Sentry |

---
## การใช้งาน

### เริ่มต้นระบบ

```go
import (
    "context"
    "github.com/nicedev97/eotel"
)

func main() {
    cfg := eotel.LoadConfigFromEnv()
    shutdown, err := eotel.InitEOTEL(context.Background(), cfg)
    if err != nil {
        panic(err)
    }
    defer shutdown(context.Background())

    eto := eotel.New(context.Background(), "main")
    eto.WithField("version", "v1.0.0").Info("service started")
}
```

### ใช้ Gin Middleware

```go
r := gin.New()
r.Use(eotel.Middleware("gin-server"))
```

---

## ตัวอย่าง Use Cases

### Microservices Logging
```go
eto := eotel.New(ctx, "OrderService").
    WithField("order_id", "123456").
    WithField("customer_id", "user-789")

defer eto.Info("Order received successfully")
```

### Error Monitoring (Sentry + Span Error)
```go
err := errors.New("payment gateway timeout")

eotel.New(ctx, "PaymentService").
    WithField("payment_id", "pay-001").
    WithError(err).
    Error("Failed to process payment")
```

### HTTP Request Tracing (Gin Middleware)
```go
r := gin.New()
r.Use(eotel.Middleware("api-gateway"))

r.GET("/health", func(c *gin.Context) {
    eto := eotel.FromContext(c.Request.Context(), "HealthCheck")
    eto.Info("health check pinged")
    c.JSON(200, gin.H{"status": "ok"})
})
```

### External Call with Trace Context
```go
import (
    "net/http"
    "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func callExternal(ctx context.Context) {
    client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.example.com/data", nil)
    resp, err := client.Do(req)
	if err != nil {
        eotel.New(ctx, "ExternalCall").WithError(err).Error("failed external call")
        return
    }

    defer resp.Body.Close()
    eotel.New(ctx, "ExternalCall").WithField("status", resp.StatusCode).Info("external call succeeded")
}
```

### Testing and Debugging
```go
eto := eotel.New(ctx, "DebugLogger").
    WithField("step", "connect-db").
    WithField("retry", 3)

eto.Info("trying to connect to database")
```

### Performance Analysis (Span + Duration)
```go
eto := eotel.New(ctx, "ImageProcessor").
    WithField("image_id", "img-001")

eto.TraceName("resize-image").
    WithField("size", "1024x768").
    Info("start resize")

time.Sleep(100 * time.Millisecond)

eto.Info("resize done")
```

### Background Worker Job Logger
```go
func ProcessJob(ctx context.Context, jobID string) {
    eto := eotel.New(ctx, "Worker").
        WithField("job_id", jobID)

    eto.Info("start job")

    // simulate job processing
    time.Sleep(200 * time.Millisecond)

    eto.Info("job completed")
}
```

---

## ระบบที่รองรับ

- Grafana Loki (log aggregation)
- Sentry (error tracking)
- OpenTelemetry Collector (traces, metrics)

---

## 🙏 ขอขอบคุณ
- [ChatGPT](https://openai.com/)
- [Uber Zap](https://github.com/uber-go/zap)
- [OpenTelemetry](https://opentelemetry.io/)
- [Sentry Go](https://github.com/getsentry/sentry-go)
- [Grafana Loki](https://grafana.com/oss/loki/)
