package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/michaelprice232/ecr-image-checker/internal/checker"
)

func main() {
	l := os.Getenv("LOG_LEVEL")
	if err := setLogLevel(l); err != nil {
		slog.Error("setting log level", "err", err)
		os.Exit(1)
	}

	// Default to current directory
	imageDirectory := os.Getenv("IMAGE_DIRECTORY")
	if imageDirectory == "" {
		imageDirectory = "."
	}

	if err := checker.Run(imageDirectory); err != nil {
		slog.Error("whilst running", "err", err)
		os.Exit(1)
	}
}

func setLogLevel(level string) error {
	logLevel := slog.LevelVar{}

	if level == "" {
		logLevel.Set(slog.LevelError)
	} else {
		if err := logLevel.UnmarshalText([]byte(level)); err != nil {
			return fmt.Errorf("unable to parse log level: %w", err)
		}
	}

	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: &logLevel,
	})
	slog.SetDefault(slog.New(h))

	return nil
}
