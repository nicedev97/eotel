package eotel

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoggerCreation(t *testing.T) {
	cfg := Config{
		ServiceName:   "test-service",
		JobName:       "test-job",
		EnableTracing: false,
		EnableMetrics: false,
		EnableSentry:  false,
		EnableLoki:    false,
	}
	_, err := InitEOTEL(context.Background(), cfg)
	assert.NoError(t, err)

	logger := New(context.Background(), "TestLogger")
	assert.NotNil(t, logger)
	logger.WithField("key", "value").Info("test log")
}

func TestLoggerError(t *testing.T) {
	logger := New(context.Background(), "TestLogger")

	mockErr := errors.New("mock error")
	logger.WithError(mockErr).Error("error occurred")

	// ตรวจสอบว่า error ถูกเซ็ตใน logger
	assert.EqualError(t, logger.err, "mock error")
}
