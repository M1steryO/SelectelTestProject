package main

import (
	"context"
	"log/slog"

	"go.uber.org/zap"
)

func main() {
	// Rule 1: lowercase start
	slog.Info("Started processing")

	// Rule 2: English-only
	slog.Info("привет")

	// Rule 3: no special symbols/emoji
	slog.Info("ok 🙂")

	// Rule 4: sensitive data in dynamic message
	token := "abc"
	slog.Info("token=" + token)

	logger := zap.NewNop()
	logger.Info("Hello from zap")
	logger.Sugar().Infow("password=" + token)
	_ = context.Background()
}
