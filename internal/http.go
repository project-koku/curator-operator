package internal

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/go-logr/logr"
	"github.com/jackc/pgx/v4"
)

// Server defines the http server configuration
type Server struct {
	Addr     string
	ErrorLog *log.Logger
}

// HTTPError defines the http error codes
type HTTPError struct {
	code    int
	message string
	err     error
}

// errorResponse defines error response in json
type errorResponse struct {
	Error string `json:"error"`
}

func NewHTTPServer(ctx context.Context, httpServerPort string, conn *pgx.Conn, logger logr.Logger) {
	http.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		httpErr := downloadReport(ctx, w, r, conn, logger)
		if httpErr != nil {
			writeErrorResponse(logger, w, httpErr.code, httpErr.message)
		}
	})
}

func downloadReport(ctx context.Context, w http.ResponseWriter, r *http.Request, conn *pgx.Conn, logger logr.Logger) *HTTPError {
	HTTPErr := validateRequest(r)
	if HTTPErr != nil {
		return &HTTPError{HTTPErr.code, HTTPErr.message, nil}
	}

	err := generateReport(ctx, conn, r, logger)
	if err != nil {
		return &HTTPError{http.StatusInternalServerError, "error generating report", err}
	}

	w.WriteHeader(http.StatusOK)

	_, err = w.Write([]byte("generated report"))
	if err != nil {
		return &HTTPError{http.StatusInternalServerError, "error generating report", err}
	}

	return nil
}

func validateRequest(r *http.Request) *HTTPError {
	if r.URL.Path != "/download" {
		return &HTTPError{http.StatusNotFound, "404 not found", nil}
	}

	if r.Method != "GET" {
		return &HTTPError{http.StatusMethodNotAllowed, "method is not supported", nil}
	}

	query := r.URL.Query()
	start, present := query["start"]
	if !present || len(start) == 0 {
		return &HTTPError{http.StatusBadRequest, "start time not present", nil}
	}

	end, present := query["end"]
	if !present || len(end) == 0 {
		return &HTTPError{http.StatusBadRequest, "end time not present", nil}
	}
	return nil
}

func generateReport(ctx context.Context, conn *pgx.Conn, r *http.Request, logger logr.Logger) error {
	query := r.URL.Query()
	start := query["start"]
	end := query["end"]
	dirName := "/tmp/curator-report"
	err := os.MkdirAll(dirName, 0755)
	if err != nil {
		return err
	}
	tarDirName := fmt.Sprintf("%s/%s-%s-koku-metrics.tar.gz",
		dirName, strings.Join(start, ","), strings.Join(end, ","))
	outputFile, err := os.OpenFile(tarDirName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer outputFile.Close()
	writer, err := gzip.NewWriterLevel(outputFile, gzip.BestCompression)
	if err != nil {
		return err
	}
	defer writer.Close()
	tw := tar.NewWriter(writer)
	defer tw.Close()
	err = exporter(ctx, conn, tw, r)
	if err != nil {
		return err
	}
	logger.Info("report generated successfully")
	return nil
}

func exporter(ctx context.Context, conn *pgx.Conn, tw *tar.Writer, r *http.Request) error {
	query := r.URL.Query()
	start := query["start"]
	end := query["end"]
	numberOfTables := 4
	for i := 0; i < numberOfTables; i++ {
		sqlQuery := fmt.Sprintf("COPY (SELECT * FROM logs_%d WHERE interval_start >= '%s'::timestamp with time zone AND interval_end < '%s' "+
			"::timestamp with time zone) TO STDOUT WITH CSV DELIMITER ',' HEADER", i, strings.Join(start, ","), strings.Join(end, ","))
		outfile := fmt.Sprintf("/tmp/curator-report/%s-%s-koku-metrics.%d.csv", strings.Join(start, ","), strings.Join(end, ","), i)
		ef, err := os.OpenFile(outfile, os.O_RDWR|os.O_CREATE, 0755)
		if err != nil {
			return err
		}
		_, err = conn.PgConn().CopyTo(ctx, ef, sqlQuery)
		if err != nil {
			return err
		}

		ef.Close()
		err = addToArchive(tw, outfile)
		if err != nil {
			return err
		}
		err = os.Remove(outfile)
		if err != nil {
			return err
		}
	}

	return nil
}

func addToArchive(tw *tar.Writer, filename string) error {
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	if body != nil {
		hdr := &tar.Header{
			Name: path.Base(filename),
			Mode: int64(0644),
			Size: int64(len(body)),
		}
		err := tw.WriteHeader(hdr)
		if err != nil {
			return err
		}

		if _, err := tw.Write(body); err != nil {
			return err
		}
	}
	return nil
}

func writeErrorResponse(logger logr.Logger, w http.ResponseWriter, status int, message string, args ...interface{}) {
	msg := fmt.Sprintf(message, args...)
	writeResponseAsJSON(logger, w, status, errorResponse{Error: msg})
}

func writeResponseAsJSON(logger logr.Logger, w http.ResponseWriter, code int, resp interface{}) {
	enc, err := json.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if _, err = w.Write(enc); err != nil {
		logger.Error(err, "failed writing HTTP response")
	}
}
