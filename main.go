package main

import (
	"cron_test/better_cron"
	"cron_test/custom_logger"
	"fmt"
	"github.com/robfig/cron/v3"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := custom_logger.NewLogger(custom_logger.DEBUG, os.Stdout)

	// Create enhanced cron scheduler
	ec := better_cron.NewEnhancedCron(
		better_cron.WithTimeout(1*time.Minute),
		better_cron.WithLogger(logger),
	)

	// Add jobs
	ec.AddJob("*/5 * * * * *", cron.FuncJob(func() {
		fmt.Println("Running job...")
		time.Sleep(20 * time.Second)
		fmt.Println("Done")
	}), "my-job")

	// Start the scheduler
	ec.Start()

	// Set up channel to listen for interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive a signal
	<-sigChan

	log.Println("Shutdown signal received, initiating graceful shutdown...")

	// Initiate graceful shutdown
	if err := ec.Shutdown(); err != nil {
		log.Printf("Shutdown error: %v", err)
		os.Exit(1)
	}

	log.Println("Graceful shutdown completed successfully")
}
