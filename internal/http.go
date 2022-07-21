package internal

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/go-logr/logr"
	"github.com/jackc/pgx/v4"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	curatorv1alpha1 "github.com/operate-first/curator-operator/api/v1alpha1"
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

var timeFormat = "2006-01-02 15:04:05"

var metricsList = []string{"report_period_start", "report_period_end", "interval_start", "interval_end"}

func NewHTTPServer(ctx context.Context, httpServerPort string, conn *pgx.Conn, logger logr.Logger, c client.Client) {
	http.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		httpErr := downloadReport(ctx, w, r, conn, logger)
		if httpErr != nil {
			writeErrorResponse(logger, w, httpErr.code, httpErr.message)
		}
	})

	http.HandleFunc("/report", func(w http.ResponseWriter, r *http.Request) {
		httpErr := reportAPI(ctx, w, r, conn, c, logger)
		if httpErr != nil {
			writeErrorResponse(logger, w, httpErr.code, httpErr.message)
		}
	})
}

func reportAPI(ctx context.Context, w http.ResponseWriter, r *http.Request, conn *pgx.Conn, c client.Client, logger logr.Logger) *HTTPError {
	HTTPErr := validateReportAPIRequest(r)
	if HTTPErr != nil {
		return &HTTPError{HTTPErr.code, HTTPErr.message, nil}
	}

	query := r.URL.Query()
	reportName := strings.Join(query["reportName"], ",")
	reportNamespace := strings.Join(query["reportNamespace"], ",")

	report := &curatorv1alpha1.ReportAPI{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: reportNamespace, Name: reportName}, report); err != nil {
		return &HTTPError{http.StatusBadRequest, "error fetching report", err}
	}

	finalQuery, HTTPErr := generateDBQuery(report)
	if HTTPErr != nil {
		return &HTTPError{HTTPErr.code, HTTPErr.message, nil}
	}

	rows, err := conn.Query(ctx, finalQuery)
	if err != nil {
		log.Fatal("error while executing query")
	}
	fds := rows.FieldDescriptions()

	colJSON := make([]string, 0, len(fds))
	for _, fd := range fds {
		colJSON = append(colJSON, string(fd.Name))
	}

	var finalresult []interface{}
	for rows.Next() {
		columnVal, _ := rows.Values()
		res := make(map[string]interface{})

		for i, v := range columnVal {
			res[colJSON[i]] = v
		}
		finalresult = append(finalresult, res)
	}

	result, err := json.Marshal(finalresult)
	if err != nil {
		log.Fatalf("Error happened in JSON marshal. Err: %s", err)
	}
	w.Header().Set("Content-Type", "application/json")

	if finalresult == nil {
		resp := make(map[string]string)
		resp["message"] = "No content available."
		jsonResp, err := json.Marshal(resp)
		if err != nil {
			log.Fatalf("Error happened in JSON marshal. Err: %s", err)
		}
		if _, err := w.Write(jsonResp); err != nil {
			logger.Error(err, "failed writing HTTP response")
		}
		return nil
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(result); err != nil {
		logger.Error(err, "failed writing HTTP response")
	}
	return nil
}

func generateDBQuery(report *curatorv1alpha1.ReportAPI) (string, *HTTPError) {
	var initialQuery, finalQuery string
	endInterval := report.Spec.ReportingEnd.Format(timeFormat)

	// Check if metricsName present
	if len(report.Spec.MetricsName) > 0 {
		var str []string
		for _, value := range report.Spec.MetricsName {
			str = append(str, string(value))
		}
		st := append(metricsList, str...)
		selectedItem := strings.Join(st, ", ")
		initialQuery = fmt.Sprintf("SELECT %s FROM logs_2", selectedItem)
	} else {
		initialQuery = "SELECT * FROM logs_2"
	}

	startInterval := ""
	if report.Spec.ReportingStart != nil {
		HTTPErr := validateReportTimeFrame(report.Spec.ReportingStart, report.Spec.ReportingEnd)
		if HTTPErr != nil {
			return "", &HTTPError{HTTPErr.code, HTTPErr.message, nil}
		}
		startInterval = report.Spec.ReportingStart.Format(timeFormat)
		finalQuery = fmt.Sprintf(initialQuery+" WHERE interval_start >='%s'"+
			"::timestamp with time zone AND interval_end < '%s'::timestamp with time zone", startInterval, endInterval)
	} else {
		offset := 0
		reportPeriod := strings.ToLower(string(report.Spec.ReportAPIPeriod))

		switch reportPeriod {
		case "day":
			offset = 1
		case "week":
			offset = 7
		case "month":
			offset = 30
		default:
			offset = 0
		}

		finalQuery = fmt.Sprintf(initialQuery+" WHERE interval_start >="+
			"'%s'::timestamp with time zone - interval '%d day' AND interval_end < '%s'::timestamp with time zone", endInterval, offset, endInterval)
	}

	if report.Spec.Namespace != "" {
		finalQuery = fmt.Sprintf(finalQuery+" AND namespace='%s'", report.Spec.Namespace)
	}

	return finalQuery, nil
}

func downloadReport(ctx context.Context, w http.ResponseWriter, r *http.Request, conn *pgx.Conn, logger logr.Logger) *HTTPError {
	HTTPErr := validateRequest(r)
	if HTTPErr != nil {
		return &HTTPError{HTTPErr.code, HTTPErr.message, nil}
	}

	query := r.URL.Query()
	start := query["start"]
	end := query["end"]
	dirName := "/tmp/curator-report"
	err := os.MkdirAll(dirName, 0755)
	if err != nil {
		return &HTTPError{http.StatusInternalServerError, "error creating directory", err}
	}
	tarDirName := fmt.Sprintf("%s/%s-%s-koku-metrics.tar.gz",
		dirName, strings.Join(start, ","), strings.Join(end, ","))

	err = generateReport(ctx, conn, r, logger, tarDirName)
	if err != nil {
		return &HTTPError{http.StatusInternalServerError, "error generating report", err}
	}

	f, err := os.Open(tarDirName)
	if err != nil {
		return &HTTPError{http.StatusInternalServerError, "error generating report", err}
	}
	defer f.Close()

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_, err = io.Copy(w, f)
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

func generateReport(ctx context.Context, conn *pgx.Conn, r *http.Request, logger logr.Logger, tarDirName string) error {
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

func validateReportAPIRequest(r *http.Request) *HTTPError {
	if r.URL.Path != "/report" {
		return &HTTPError{http.StatusNotFound, "404 not found", nil}
	}

	if r.Method != "GET" {
		return &HTTPError{http.StatusMethodNotAllowed, "method is not supported", nil}
	}

	query := r.URL.Query()
	reportName, present := query["reportName"]
	if !present || len(reportName) == 0 {
		return &HTTPError{http.StatusBadRequest, "reportName not present", nil}
	}

	reportNamespace, present := query["reportNamespace"]
	if !present || len(reportNamespace) == 0 {
		return &HTTPError{http.StatusBadRequest, "reportNamespace not present", nil}
	}
	return nil
}

func validateReportTimeFrame(startTime *metav1.Time, endTime *metav1.Time) *HTTPError {
	if startTime.Time.After(endTime.Time) {
		return &HTTPError{http.StatusMethodNotAllowed, "reportingStart should never come after reportingEnd", nil}
	}
	return nil
}
