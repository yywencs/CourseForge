package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "prepare" {
		if err := runPrepareCommand(args[1:], os.Stdout, os.Stderr); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return
			}
			fmt.Fprintf(os.Stderr, "prepare 失败: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if len(args) > 0 && args[0] == "run" {
		args = args[1:]
	}

	config, err := parseConfig(args, os.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintf(os.Stderr, "benchmark 配置错误: %v\n", err)
		os.Exit(2)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	fmt.Println("CourseForge enrollment benchmark")
	fmt.Printf("  endpoint:          %s\n", config.endpoint())
	fmt.Printf("  scenario:          %s\n", config.normalizedScenario())
	fmt.Printf("  round:             %d\n", config.RoundID)
	fmt.Printf("  teaching class:    %d\n", config.TeachingClassID)
	fmt.Printf("  student ID range:  %d-%d\n", config.StudentIDStart, config.StudentIDStart+uint64(config.Users-1))
	fmt.Printf("  users:             %d\n", config.Users)
	fmt.Printf("  concurrency:       %d\n", config.Concurrency)
	fmt.Printf("  duration:          %s\n", config.Duration)
	fmt.Printf("  timeout:           %s\n", config.Timeout)

	runner, err := newBenchmarkRunner(config, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "benchmark 请求准备失败: %v\n", err)
		os.Exit(1)
	}
	summary := runner.run(ctx)
	printSummary(summary)
	executionErr := summary.validateExecution(config)
	if config.Verify {
		report, err := verifyBenchmark(ctx, config, summary)
		if err != nil {
			fmt.Fprintf(os.Stderr, "benchmark 最终校验失败: %v\n", err)
			os.Exit(1)
		}
		printVerificationReport(report)
	}
	if executionErr != nil {
		fmt.Fprintf(os.Stderr, "benchmark 执行校验失败: %v\n", executionErr)
		os.Exit(1)
	}
}
