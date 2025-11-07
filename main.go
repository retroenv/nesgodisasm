// Package main implements the main entry point for a multi retro system disassembler
package main

import (
	"errors"
	"os"

	"github.com/retroenv/retrodisasm/internal/cli"
	"github.com/retroenv/retrodisasm/internal/config"
	"github.com/retroenv/retrodisasm/internal/fileprocessor"
	"github.com/retroenv/retrogolib/log"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	opts, disasmOptions, err := cli.ParseFlags()
	if err != nil {
		logger := config.CreateLogger(opts.Debug, opts.Quiet)
		var usageErr *cli.UsageError
		if errors.As(err, &usageErr) {
			fileprocessor.PrintBanner(logger, opts, version, commit, date)
			usageErr.ShowUsage()
		} else {
			logger.Fatal(err.Error())
		}
		os.Exit(1)
	}

	logger := config.CreateLogger(opts.Debug, opts.Quiet)
	fileprocessor.PrintBanner(logger, opts, version, commit, date)

	files, err := fileprocessor.GetFilesToProcess(&opts)
	if err != nil {
		logger.Fatal(err.Error())
	}

	for _, file := range files {
		opts.Input = file
		if len(files) > 1 || opts.Output == "" {
			opts.Output = fileprocessor.GenerateOutputFilename(file)
		}

		if err := fileprocessor.ProcessFile(logger, opts, disasmOptions); err != nil {
			logger.Error("Disassembling failed", log.Err(err))
		}
	}
}
