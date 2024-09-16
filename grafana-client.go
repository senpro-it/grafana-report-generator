package main

import (
	"net/url"
	"sync"

	"github.com/go-openapi/strfmt"
	grafana "github.com/grafana/grafana-openapi-client-go/client"
	client_dashboards "github.com/grafana/grafana-openapi-client-go/client/dashboards"
	"github.com/grafana/grafana-openapi-client-go/client/search"
	"github.com/grafana/grafana-openapi-client-go/models"
	"github.com/samber/oops"
	mymodels "github.com/senpro-it/grafana-report-generator/models"
	"github.com/senpro-it/grafana-report-generator/tools"
)

type GrafanaClient struct {
	client *grafana.GrafanaHTTPAPI
}

// Crude and unencapsulated cache.
var dashboardCacheMutex = sync.Mutex{}
var dashboardCache = make(map[string]models.JSON)

func MakeGrafanaClient(baseUrl string, username string, password string) (*GrafanaClient, error) {
	gurl, err := url.Parse(baseUrl)
	if err != nil {
		return nil, oops.
			In("MakeGrafanaClient").
			User(username).
			With("baseUrl", baseUrl).
			Wrap(err)
	}
	cfg := &grafana.TransportConfig{
		Host:     gurl.Host,
		BasePath: gurl.Path,
		BasicAuth: url.UserPassword(username, password),
	}
	client := grafana.NewHTTPClientWithConfig(strfmt.Default, cfg)
	return &GrafanaClient{client}, nil
}

func (c *GrafanaClient) IsOK() (bool, error) {
	oopsBuilder := oops.In("IsOK")
	ok, err := c.client.Health.GetHealth()
	if err != nil {
		return false, oopsBuilder.Wrap(err)
	}
	if !ok.IsSuccess() {
		return false, oopsBuilder.Errorf("health request was not successful")
	}
	return true, nil
}

func (c *GrafanaClient) GetOrgs() ([]*models.OrgDTO, error) {
	oopsBuilder := oops.In("GetOrgs")
	orgs, err := c.client.Orgs.SearchOrgs(nil, nil)
	if err != nil {
		return nil, oopsBuilder.
			Hint("client.SearchOrgs != nil").
			Wrap(err)
	}
	if !orgs.IsSuccess() {
		return nil, oopsBuilder.
			Hint("client.SearchOrgs.IsSuccess()").
			With("orgs", orgs).
			Wrap(err)
	}

	return orgs.GetPayload(), nil
}

func putDashInCache(dashUid string, dash models.JSON) {
	logger := logger.WithPrefix("Dashboard cache (put)")
	logger.Debug("Locking mutex...", "uid", dashUid)
	dashboardCacheMutex.Lock()
	logger.Debug("Locked.", "uid", dashUid)

	dashboardCache[dashUid] = dash

	logger.Debug("UNlocking mutex...", "uid", dashUid)
	dashboardCacheMutex.Unlock()
	logger.Debug("Unlocked.", "uid", dashUid)
}

func isDashInCache(dashUid string) bool {
	logger := logger.WithPrefix("Dashboard cache (is)")
	logger.Debug("Locking mutex...", "uid", dashUid)
	dashboardCacheMutex.Lock()
	logger.Debug("Locked.", "uid", dashUid)

	_, ok := dashboardCache[dashUid]

	logger.Debug("UNlocking mutex...", "uid", dashUid)
	dashboardCacheMutex.Unlock()
	logger.Debug("Unlocked.", "uid", dashUid)
	return ok
}

func getDashInCache(dashUid string) models.JSON {
	logger := logger.WithPrefix("Dashboard cache (get)")
	logger.Debug("Locking mutex...", "uid", dashUid)
	dashboardCacheMutex.Lock()
	logger.Debug("Locked.", "uid", dashUid)

	dash := dashboardCache[dashUid]

	logger.Debug("UNlocking mutex...", "uid", dashUid)
	dashboardCacheMutex.Unlock()
	logger.Debug("Unlocked.", "uid", dashUid)

	return dash
}

func (c *GrafanaClient) DoesDashboardExist(dashUid string) bool {
	logger := logger.
		WithPrefix("Does Dashboard exist?").
		With("uid", dashUid)
	if isDashInCache(dashUid) {
		logger.Debug("In cache")
		return true
	}
	dashReq, err := c.client.Dashboards.GetDashboardByUID(dashUid)
	if _, ok := err.(*client_dashboards.GetDashboardByUIDNotFound); ok {
		logger.Debug("Not found")
		return false
	}
	// TODO: Technically an ACL check, but we'll figure this out later.
	if _, ok := err.(*client_dashboards.GetDashboardByUIDForbidden); ok {
		logger.Debug("Forbidden")
		return false
	}
	logger.With("dashReq", dashReq).Debug("Probably exists")
	return dashReq.IsSuccess()
}

func (c *GrafanaClient) GetDashboardsInOrg(orgID int64) ([]mymodels.Dashboard, error) {
	oopsMaker := oops.In("GetDashboardsInOrg")
	params := &search.SearchParams{Type: tools.PtrOf("dashboard-ds")}

	orgClient := c.client.WithOrgID(orgID)
	org, err := orgClient.Org.GetCurrentOrg()
	if err != nil {
		return nil, oopsMaker.Wrap(err)
	}
	if !org.IsSuccess() {
		return nil, oopsMaker.
			With("org", org).
			Wrap(err)
	}
	orgData := org.GetPayload()

	dashboards, err := orgClient.Search.Search(params, nil)
	if err != nil {
		return nil, oopsMaker.
			With("dashboards", dashboards).
			Wrap(err)
	}

	var filteredDashboards []mymodels.Dashboard
	for _, vv := range dashboards.GetPayload() {
		oopsMaker = oopsMaker.With("searchHit", vv)
		if !isDashInCache(vv.UID) {
			if !c.DoesDashboardExist(vv.UID) {
				continue
			}	
			dashReq, err := c.client.Dashboards.GetDashboardByUID(vv.UID)
			if _, ok := err.(*client_dashboards.GetDashboardByUIDNotFound); ok {
				continue
			}
			if err != nil {
				if dashReq != nil {
					oopsMaker = oopsMaker.With("dashReq", dashReq)
				}
				return nil, oopsMaker.
					Wrap(err)
			}
			if !dashReq.IsSuccess() {
				return nil, oopsMaker.
					With("dashReq", dashReq).
					Wrap(err)
			}
			if string(vv.Type) == "dash-folder" || dashReq.GetPayload().Meta.IsFolder {
				continue
			}
			// Write to cache; mind the threads!
			putDashInCache(vv.UID, dashReq.GetPayload().Dashboard)
		}
	
		filteredDashboards = append(filteredDashboards, mymodels.Dashboard{
			ID: vv.ID,
			UID: vv.UID,
			Title: vv.Title,
			FolderTitle: vv.FolderTitle,
			Slug: vv.Slug,
			OrgID: orgData.ID,
			OrgName: orgData.Name,
		})
	}
	
	return filteredDashboards, nil
}

func (c *GrafanaClient) GetVariablesInDashboard(dashUid string) (map[string]string, error) {
	oopsMaker := oops.In("GetDashboardsInOrg")
	var thisDash models.JSON
	if isDashInCache(dashUid) {
		thisDash = getDashInCache(dashUid)
	} else {
		dashReq, err := c.client.Dashboards.GetDashboardByUID(dashUid)
		if err != nil {
			if dashReq != nil {
				oopsMaker = oopsMaker.With("dashReq", dashReq)
			}
			return nil, oopsMaker.
				Wrap(err)
		}
		if !dashReq.IsSuccess() {
			return nil, oopsMaker.
				With("dashReq", dashReq).
				Wrap(err)
		}
		thisDash = dashReq.GetPayload().Dashboard
		putDashInCache(dashUid, thisDash)
	}

	// I hate this SO! MUCH!
	var vars = make(map[string]string)
	templating := thisDash.(map[string]interface{})["templating"]
	if templating != nil {
		templateList := templating.(map[string]interface{})["list"]
		if templateList != nil {
			for _, v := range templateList.([]interface{}) {
				v := v.(map[string]interface{})
				//spew.Dump(v)
				if v == nil { continue }
				var varName string
				var varValue string
				var okName bool
				var okCurrent bool
				var okCurrentValue bool
				name, okName := v["name"].(string)
				if okName {
					varName = name
				}
				current, okCurrent := v["current"].(map[string]interface{})
				if okCurrent {
					currentValue, sub_okCurrentValue := current["text"].(string)
					okCurrentValue = sub_okCurrentValue
					if okCurrentValue {
						varValue = currentValue
					}
				}
				vars[varName] = varValue
			}
		}
	}
	return vars, nil
}
