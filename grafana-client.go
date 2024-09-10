package main

import (
	"errors"
	"net/url"

	"github.com/go-openapi/strfmt"
	grafana "github.com/grafana/grafana-openapi-client-go/client"
	"github.com/grafana/grafana-openapi-client-go/client/search"
	"github.com/grafana/grafana-openapi-client-go/models"
	mymodels "github.com/senpro-it/grafana-report-generator/models"
	"github.com/senpro-it/grafana-report-generator/tools"
)

type GrafanaClient struct {
	client *grafana.GrafanaHTTPAPI
}

func MakeGrafanaClient(baseUrl string, apiToken string) GrafanaClient {
	url, err := url.Parse(baseUrl)
	if err != nil {
		logger.Fatal("Unable to parse URL!", "url", baseUrl)
	}
	cfg := &grafana.TransportConfig{
		Host:     url.Host,
		BasePath: url.Path,
		APIKey:   apiToken,
	}
	client := grafana.NewHTTPClientWithConfig(strfmt.Default, cfg)
	return GrafanaClient{client}
}

func (c GrafanaClient) GetOrgs() ([]*models.OrgDTO, error) {
	orgs, err := c.client.Orgs.SearchOrgs(nil, nil)
	if err != nil {
		return nil, err
	}
	if !orgs.IsSuccess() {
		return nil, errors.New("Fetching orgs was not successful")
	}

	return orgs.GetPayload(), nil
}

func (c GrafanaClient) GetDashboardsInOrg(orgID int64) ([]mymodels.Dashboard, error) {
	params := &search.SearchParams{Type: tools.PtrOf("dashboard-ds")}

	dashboards, err := c.client.WithOrgID(orgID).Search.Search(params, nil)
	if err != nil {
		return nil, err
	}

	var filteredDashboards []mymodels.Dashboard
	for _, vv := range dashboards.GetPayload() {
		if string(vv.Type) == "dash-folder" {
			continue
		}
		filteredDashboards = append(filteredDashboards, mymodels.Dashboard{
			ID: vv.FolderID,
			UID: vv.FolderUID,
			Title: vv.Title,
			Slug: vv.Slug,
		})
	}
	
	return filteredDashboards, nil
}

func (c GrafanaClient) GetVariablesInDashboard() {}
