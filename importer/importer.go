package importer

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"golang.org/x/sync/errgroup"
)

var (
	processed uint64
	startTime time.Time
)

func ImportFromCSVFile(ctx context.Context, pb *pocketbase.PocketBase, collectionName, filePath, delimiter string, goroutinesLimit int, validate bool, printDelay time.Duration) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	if collectionName == "" {
		return fmt.Errorf("collectionName cannot be empty")
	}

	collection, err := pb.FindCollectionByNameOrId(collectionName)
	if err != nil {
		return fmt.Errorf("failed to find collection by name/id: %w", err)
	}

	csvRecordsChan, readerErrChan, headers := StartCSVReader(ctx, file, delimiter)

	go func() {
		select {
		case <-ctx.Done():
		case err, ok := <-readerErrChan:
			if !ok {
				return
			}
			log.Fatal(err)
		}
	}()

	return PocketbaseWriter(ctx, headers, csvRecordsChan, pb, collection, goroutinesLimit, validate, printDelay)
}

func PocketbaseWriter(ctx context.Context, columnNames []string, values <-chan []string, pb *pocketbase.PocketBase, col *core.Collection, goroutinesLimit int, validate bool, printDelay time.Duration) error {
	startTime = time.Now()

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(goroutinesLimit)

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(printDelay)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				count := atomic.LoadUint64(&processed)
				elapsed := time.Since(startTime).Seconds()
				var rps float64
				if elapsed > 0 {
					rps = float64(count) / elapsed
				}
				fmt.Printf("\rProcessed: %d rows | %.1f rows/sec", count, rps)
			case <-ctx.Done():
				return
			}
		}
	}()
	defer close(done)

	for recordCSV := range values {
		recordCSVCopy := recordCSV
		g.Go(func() error {
			record := core.NewRecord(col)
			for i, header := range columnNames {
				if i >= len(recordCSVCopy) {
					break
				}
				record.Set(header, recordCSVCopy[i])
			}
			var err error
			if validate {
				err = pb.SaveWithContext(ctx, record)
			} else {
				err = pb.SaveNoValidateWithContext(ctx, record)
			}
			if err != nil {
				if !strings.HasSuffix(err.Error(), "Value must be unique.") {
					return err
				}
			}
			atomic.AddUint64(&processed, 1)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		fmt.Printf("\rProcessed: %d rows (failed)\n", atomic.LoadUint64(&processed))
		return err
	}

	count := atomic.LoadUint64(&processed)
	elapsed := time.Since(startTime).Seconds()
	var rps float64
	if elapsed > 0 {
		rps = float64(count) / elapsed
	}
	fmt.Printf("\rProcessed: %d rows | %.1f rows/sec (finished)\n", count, rps)
	fmt.Println("Import completed successfully.")
	return nil
}

func StartCSVReader(ctx context.Context, r io.Reader, delimiter string) (recordsChan <-chan []string, errChan <-chan error, headers []string) {
	records := make(chan []string)
	errs := make(chan error)
	csvReader := csv.NewReader(r)

	s, err := strconv.Unquote(`"` + delimiter + `"`)
	if err != nil {
		log.Fatal("invalid delimiter:", err)
	}

	delimRune := []rune(s)
	if len(delimRune) != 1 {
		log.Fatal("delimiter must be exactly one character")
	}
	if delimRune[0] != ',' {
		fmt.Printf("Using delimiter: 0x%X\n", delimRune[0])
	}
	csvReader.Comma = delimRune[0]

	headers, err = csvReader.Read()
	if err != nil {
		log.Fatal("Input file must contain csv headers:", err)
	}

	go func() {
		defer close(records)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				record, err := csvReader.Read()
				if err != nil {
					if err != io.EOF {
						safeSend(ctx, errs, fmt.Errorf("csv read error at row: %w", err))
					}

					return
				}
				safeSend(ctx, records, record)
			}
		}
	}()

	return records, errs, headers
}

func safeSend[T any](ctx context.Context, c chan<- T, value T) {
	select {
	case <-ctx.Done():
	case c <- value:
	}
}
