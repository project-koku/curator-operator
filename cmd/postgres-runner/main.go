package main

import (
	"bufio"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/lib/pq"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	// TODO: all of this const variables could be CLI inputs to this executable.
	const (
		csvDataDir = "cmd/postgres-runner/test-data"
		tableName  = "testing"
		uri        = "postgres://postgresadmin:admin123@localhost:5432/postgresdb?sslmode=disable"
	)
	var (
		csvFilePath string
	)

	// TODO: this is a hack only to get a single CSV file name in the test-data
	// directory for now to get the actual runner implementation working for reading
	// and parsing a single CSV file, then inserting those values into a postgres database.
	csvDataFS := os.DirFS(csvDataDir)
	err := fs.WalkDir(csvDataFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".csv" {
			csvFilePath = filepath.Join(csvDataDir, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(csvFilePath) == 0 {
		return fmt.Errorf("failed to find a CSV file in the %s input directory", csvDataDir)
	}

	f, err := os.Open(csvFilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	db, err := sql.Open("postgres", uri)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Query(`
	CREATE TABLE IF NOT EXISTS testing (
		report_period_start timestamp with time zone,
		report_period_end timestamp with time zone,
		interval_start timestamp with time zone,
		interval_end timestamp with time zone,
		namespace text,
		namespace_labels text
	)`); err != nil {
		return err
	}

	csv := csv.NewReader(bufio.NewReader(f))
	csv.TrimLeadingSpace = true

	// call csv.Read() to skip the first line and then ReadAll to read until EOF
	if _, err := csv.Read(); err != nil {
		return err
	}
	records, err := csv.ReadAll()
	if err != nil {
		return err
	}

	for _, record := range records {
		var sanitizedValues []string
		for _, r := range record {
			if len(r) == 0 {
				continue
			}
			sanitizedValues = append(sanitizedValues, r)
		}

		values := strings.Join(sanitizedValues, ",")
		values = strings.ReplaceAll(values, " UTC", "")
		expression := fmt.Sprintf("VALUES (%s)", values)
		fmt.Println(expression)

		_, err := db.Query(`INSERT INTO testing (report_period_start, report_period_end, interval_start, interval_end, namespace, namespace_labels) $1`, expression)
		if err != nil {
			return err
		}
	}

	return nil
}
