package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"

	"github.com/senpro-it/grafana-report-generator/models"
)

type ReportStatus string

const (
	REPORT_STATUS_COMPLETE ReportStatus = "Report completed"
	REPORT_STATUS_PROGRESS ReportStatus = "Report in progress"
	REPORT_STATUS_FAILED   ReportStatus = "Report failed"
	REPORT_STATUS_UNKNOWN  ReportStatus = "Unknown"
)

type RGPClient struct {
	baseUrl string
	client  *http.Client
}

func NewRGPClient(baseUrl string) RGPClient {
	return RGPClient{
		baseUrl: baseUrl,
		client: &http.Client{
			Jar: &cookiejar.Jar{},
		},
	}
}

func (c RGPClient) make_query(template string, vars map[string]string) url.Values {
	vals := url.Values{}

	if template == "" {
		vals.Add("var-template", template)
	}

	for k, v := range vars {
		vals.Add(k, v)
	}

	return vals
}

func (c RGPClient) do_request(method string, endpoint string, vals url.Values) (*http.Response, error) {
	// Build request
	var apiUrl string
	if endpoint[0] == '/' {
		apiUrl = endpoint
	} else {
		apiUrl = "/api/v1/" + endpoint
	}
	fullUrl := c.baseUrl + apiUrl + "?" + vals.Encode()
	req, err := http.NewRequest(method, fullUrl, nil)
	logger.Debug(
		"Creating request",
		"method",
		method,
		"url",
		fullUrl,
	)
	if err != nil {
		logger.Error("Could not create request", "err", err)
		return nil, err
	}

	// Do request
	res, err := c.client.Do(req)
	if err != nil {
		logger.Error("Could not do request", "err", err)
		return nil, err
	}

	logger.Debug(
		"Response",
		"statusCode",
		res.StatusCode,
		"body",
		res.Body,
	)

	return res, nil
}

// Implements POST /api/v1/render
func (c RGPClient) CreateReport(template string, vars map[string]string) (int, error) {
	logger.Info("Creating new report", "template", template, "vars", vars)
	res, err := c.do_request("POST", "render", c.make_query(template, vars))
	if err != nil {
		logger.Error("Unable to do request")
		return -1, err
	}

	if res.StatusCode != 200 {
		logger.Error("Could not create report", "template", template, "vars", vars)
		return -1, errors.New("unable to create report")
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		logger.Error("Unable to read HTTP response", "body", body)
		return -1, err
	}

	var body_json interface{}
	err = json.Unmarshal(body, &body_json)
	if err != nil {
		logger.Error("Unable to parse JSON", "json", body)
		return -1, err
	}

	report_id, err := strconv.Atoi(body_json.(map[string]interface{})["report_id"].(string))
	if err != nil {
		logger.Error("Unable to convert report_id", "json", body_json)
		return -1, err
	}

	return report_id, nil
}

// Implements GET /api/v1/status
func (c RGPClient) GetReportStatus(reportId int) ReportStatus {
	vars := map[string]string{
		"report_id": strconv.Itoa(reportId),
	}
	res, err := c.do_request(
		"GET",
		"status",
		c.make_query("", vars),
	)
	if res.StatusCode != 200 || err != nil {
		logger.Error(
			"Status for report not available",
			"report_id",
			reportId,
			"statusCode",
			res.StatusCode,
			"err",
			err,
		)
		return REPORT_STATUS_UNKNOWN
	}
	if res.StatusCode == 200 {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			logger.Error("Error while decoding HTTP body", "res", res)
			return REPORT_STATUS_UNKNOWN
		}

		var report models.Reportstatus
		err = json.Unmarshal(body, &report)
		if err != nil {
			logger.Error("Unable to parse JSON", "json", body)
			return REPORT_STATUS_UNKNOWN
		}
		switch report.Status {
		case "stopped":
		case "stopping":
			return REPORT_STATUS_FAILED
		case "running":
			return REPORT_STATUS_PROGRESS
		default:
			logger.Error("Unknown status text discovered!", "status", report.Status)
			return REPORT_STATUS_UNKNOWN
		}
	}
	return REPORT_STATUS_UNKNOWN
}

// Implements GET /view_log
func (c RGPClient) GetReportLog(reportId int) (string, error) {
	vars := map[string]string{
		"report_id": strconv.Itoa(reportId),
	}
	res, err := c.do_request(
		"DELETE",
		"cancel",
		c.make_query("", vars),
	)
	if err != nil {
		logger.Error("Failed to request")
		return "", err
	}
	if res.StatusCode != 200 {
		logger.Error("Requested report logs not found", "report_id", reportId)
		return "", errors.New("report not found?")
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		logger.Error("Unable to read body", "body", res.Body)
		return "", err
	}
	return string(body), nil
}

// Implements DELETE /api/v1/cancel
func (c RGPClient) CancelReport(reportId int) (bool, error) {
	vars := map[string]string{
		"report_id": strconv.Itoa(reportId),
	}
	res, err := c.do_request(
		"DELETE",
		"cancel",
		c.make_query("", vars),
	)
	return res.StatusCode == 200 || err != nil, err
}

// Implements /view_report
func (c RGPClient) GetReport(reportId int) ([]byte, bool, error) {
	// TODO:
	// - Query report status
	// - If done, fetch report from endpoint
	// - Return report accordingly
	return []byte{0}, true, nil
}
