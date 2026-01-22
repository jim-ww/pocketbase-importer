package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"golang.org/x/sync/errgroup"
)

var (
	inputFile       = flag.String("i", "", "file to import. example: ./data.csv")
	collectionName  = flag.String("collection", "", "collection name to import data into. example: 'users'")
	goroutinesLimit = flag.Int("goroutines", 100, "max number of simultaneously ran goroutines")
	dataDir         = flag.String("dataDir", "./pb_data", "pocketbase data dir location")
)

func main() {
	flag.Parse()
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	file, err := os.Open(*inputFile)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	if *collectionName == "" {
		*collectionName = strings.Split(filepath.Base(*inputFile), ".")[0] // TODO handle names without '.'
		slog.Warn("Collection name not set explicitely, setting to filename", "filename", *collectionName)
	}

	pb := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: *dataDir})

	if err := pb.Bootstrap(); err != nil {
		return fmt.Errorf("failed to bootstrap pocketbase: %w", err)
	}

	collection, err := pb.FindCollectionByNameOrId(*collectionName)
	if err != nil {
		return fmt.Errorf("failed to find collection by name/id: %w", err)
	}

	csvRecordsChan, readerErrChan, headers := StartCSVReader(ctx, file)

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

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(*goroutinesLimit)

	for recordCSV := range csvRecordsChan {
		recordCSVCopy := recordCSV
		g.Go(func() error {
			record := core.NewRecord(collection)
			for i, header := range headers {
				if i > len(recordCSVCopy) {
					break
				}
				record.Set(header, recordCSVCopy[i])
			}
			if err := pb.Save(record); err != nil {
				return err
			}
			return nil
		})
	}

	return g.Wait()
}

func StartCSVReader(ctx context.Context, r io.Reader) (recordsChan <-chan []string, errChan <-chan error, headers []string) {
	records := make(chan []string)
	errs := make(chan error)
	csvReader := csv.NewReader(r)

	headers, err := csvReader.Read()
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
						safeSend(ctx, errs, err)
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
