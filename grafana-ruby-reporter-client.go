package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"

	"github.com/samber/oops"
	"github.com/senpro-it/grafana-report-generator/models"
)

type ReportStatus string

const (
	REPORT_STATUS_COMPLETE ReportStatus = "Report completed"
	REPORT_STATUS_PROGRESS ReportStatus = "Report in progress"
	REPORT_STATUS_FAILED   ReportStatus = "Report failed"
	REPORT_STATUS_UNKNOWN  ReportStatus = "Unknown"
)

type GRRClient struct {
	baseUrl string
	client  *http.Client
}

func NewGRRClient(baseUrl string) *GRRClient {
	return &GRRClient{
		baseUrl: baseUrl,
		client: &http.Client{
			Jar: &cookiejar.Jar{},
		},
	}
}

func (c *GRRClient) make_query(template string, from string, to string, vars map[string]string) url.Values {
	vals := url.Values{}

	if template != "" {
		vals.Add("var-template", template)
	}

	vals.Add("from", from)
	vals.Add("to", to)

	for k, v := range vars {
		vals.Add("var-" + k, v)
	}

	return vals
}

func (c *GRRClient) do_request(method string, endpoint string, vals url.Values) (*http.Response, error) {
	oopsBuilder := oops.In("do_request")
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
		"method", method,
		"url", fullUrl,
	)
	if err != nil {
		return nil, oopsBuilder.Wrap(err)
	}

	// Do request
	res, err := c.client.Do(req)
	if err != nil {
		return nil, oops.Wrap(err)
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
func (c *GRRClient) CreateReport(template string, from string, to string, vars map[string]string) (int, error) {
	oopsBuilder := oops.In("CreateReport")
	logger.Info("Creating new report", "template", template, "vars", vars)
	res, err := c.do_request("POST", "render", c.make_query(template, from, to, vars))
	if err != nil {
		return -1, oopsBuilder.Wrap(err)
	}

	if res.StatusCode != 200 {
		logger.Error("Could not create report", "template", template, "vars", vars)
		return -1, oopsBuilder.Errorf("unable to create report")
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return -1, oopsBuilder.Wrap(err)
	}

	var body_json interface{}
	err = json.Unmarshal(body, &body_json)
	if err != nil {
		return -1, oopsBuilder.Wrap(err)
	}

	report_id, err := strconv.Atoi(body_json.(map[string]interface{})["report_id"].(string))
	if err != nil {
		return -1, oopsBuilder.With("json", body_json).Wrap(err)
	}

	return report_id, nil
}

// Implements GET /api/v1/status
func (c *GRRClient) GetReportStatus(reportId int) ReportStatus {
	vars := map[string]string{
		"report_id": strconv.Itoa(reportId),
	}
	res, err := c.do_request(
		"GET",
		"status",
		c.make_query("", "", "", vars),
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
func (c *GRRClient) GetReportLog(reportId int) (string, error) {
	vars := map[string]string{
		"report_id": strconv.Itoa(reportId),
	}
	res, err := c.do_request(
		"DELETE",
		"cancel",
		c.make_query("", "", "", vars),
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
func (c *GRRClient) CancelReport(reportId int) (bool, error) {
	vars := map[string]string{
		"report_id": strconv.Itoa(reportId),
	}
	res, err := c.do_request(
		"DELETE",
		"cancel",
		c.make_query("", "", "", vars),
	)
	return res.StatusCode == 200 || err != nil, err
}

// Implements /view_report
func (c *GRRClient) GetReport(reportId int) ([]byte, bool, error) {
	// TODO:
	// - Query report status
	// - If done, fetch report from endpoint
	// - Return report accordingly
	return []byte{0}, true, nil
}
