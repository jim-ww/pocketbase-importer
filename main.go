package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/jim-ww/pocketbase-importer/importer"
	"github.com/pocketbase/pocketbase"
)

var (
	inputFile       = flag.String("i", "", "file to import. example: ./data.csv")
	collectionName  = flag.String("collection", "", "collection name to import data into. example: 'users'")
	goroutinesLimit = flag.Int("goroutines", 100, "max number of simultaneously ran goroutines")
	dataDir         = flag.String("dataDir", "./pb_data", "pocketbase data dir location")
	validate        = flag.Bool("validate", true, "validate records with pocketbase before inserting")
	printDelay      = flag.Duration("printDelay", time.Second, "duration before updating prints")
	delimiter       = flag.String("delimiter", ",", "csv field delimiter")
)

func main() {
	flag.Parse()

	pb := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: *dataDir})

	if err := pb.Bootstrap(); err != nil {
		log.Fatal(fmt.Errorf("failed to bootstrap pocketbase: %w", err))
	}

	if err := importer.ImportFromCSVFile(context.Background(), pb, *collectionName, *inputFile, *delimiter, *goroutinesLimit, *validate, *printDelay); err != nil {
		log.Fatal(err)
	}
}
