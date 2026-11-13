// Standalone, low-priority billing report worker. Not an API relay process.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/QuantumNous/new-api/model"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	source, err := model.OpenDeepSeekReportDatabase("DEEPSEEK_REPORT_SOURCE_DSN", 1)
	if err != nil {
		log.Fatal(err)
	}
	pool, err := source.DB()
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	target, err := model.OpenDeepSeekReportDatabase("DEEPSEEK_REPORT_WRITE_DSN", 1)
	if err != nil {
		log.Fatal(err)
	}
	reports, err := target.DB()
	if err != nil {
		log.Fatal(err)
	}
	defer reports.Close()
	if err := model.ValidateDeepSeekReportIsolation(source, target); err != nil {
		log.Fatal(err)
	}
	if err := model.InitDeepSeekReportStore(target); err != nil {
		log.Fatal(err)
	}
	batch := 200
	if text := os.Getenv("DEEPSEEK_REPORT_BATCH"); text != "" {
		batch, err = strconv.Atoi(text)
		if err != nil || batch < 1 || batch > 200 {
			log.Fatal("DEEPSEEK_REPORT_BATCH must be 1..200")
		}
	}
	// A fixed pause after every batch prevents backlog from turning into an
	// unbounded tight loop. Source and target each own just one connection.
	for {
		deadline, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := model.ProcessDeepSeekReportBatch(deadline, source, target, batch, time.Now())
		cancel()
		if err != nil && ctx.Err() == nil {
			log.Printf("billing report batch deferred: %v", err)
		}
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
