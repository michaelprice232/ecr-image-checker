package main

import (
	"flag"
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

	imageDirectory := flag.String("image-directory", ".", "Root directory which contains the image directories (each with it's own config file)")
	flag.Parse()

	if len(flag.Args()) != 1 {
		slog.Error("expected only 1 command line parameter", "got", len(flag.Args()))
		os.Exit(1)
	}

	switch flag.Arg(0) {
	case "run":
		if err := checker.Run(*imageDirectory); err != nil {
			slog.Error("whilst running", "err", err)
			os.Exit(1)
		}
	case "lint":
		if err := checker.Lint(); err != nil {
			slog.Error("whilst linting", "err", err)
			os.Exit(1)
		}
	default:
		slog.Error("expected either 'run' or 'lint' command line parameter", "got", flag.Arg(0))
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
