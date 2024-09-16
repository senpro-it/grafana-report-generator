package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/grafana/grafana-openapi-client-go/models"
	"github.com/samber/oops"
	mymodels "github.com/senpro-it/grafana-report-generator/models"
	"github.com/sourcegraph/conc/iter"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type GrafanaConfig struct {
	Url   string
	Username string
	Password string
}
type GrrConfig struct {
	Url string
}

type Config struct {
	Grr GrrConfig
	Grafana GrafanaConfig
}

var logger = log.NewWithOptions(os.Stdout, log.Options{
	Prefix:       "",
	ReportCaller: false,
	ReportTimestamp: true,
})
var v = viper.NewWithOptions(viper.WithLogger(slog.New(logger)))

func main() {
	// configure oops
	oops.SourceFragmentsHidden = false
	
	// Configure viper
	pflag.String("grr.url", "http://localhost:8989", "Grafana Ruby Reporter endpoint")
	pflag.String("grafana.url", "http://localhost:7000/api", "Grafana endpoint")
	pflag.String("grafana.user", "", "Grafana Username (BasicAuth)")
	pflag.String("grafana.pass", "", "Grafana Password (BasicAuth)")
	pflag.Bool("verbose", false, "Enable debug logs")
	pflag.Parse()
	v.BindPFlags(pflag.CommandLine)

	v.SetConfigName("reporter")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")

	v.SetEnvPrefix("grg")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logger.Warn("No config file found; using just ENV.")
		} else {
			//logger.Error("A config file was found but a different error occured", "error", err)
			err := oops.Wrap(err)
			logger.Fatal(err.Error(), "error", err) //.WithAttrs(slog.Any("err", err)
		}
	}

	// Import the config
	config := Config{
		Grr: GrrConfig{
			Url: v.GetString("grr.url"),
		},
		Grafana: GrafanaConfig{
			Url: v.GetString("grafana.url"),
			Username: v.GetString("grafana.user"),
			Password: v.GetString("grafana.pass"),
		},
	}
	if v.GetBool("verbose") {
		logger.SetLevel(log.DebugLevel)
	}

	logger.Info("Configuration loaded!")
	//logger.Info(spew.Sdump(v.AllKeys()))
	//logger.Info(v.GetString("grafana.url"))

	// Step 0: Open Grafana
	grafana, err := MakeGrafanaClient(
		config.Grafana.Url,
		config.Grafana.Username,
		config.Grafana.Password,
	)
	if err != nil {
		logger.Fatal(err.Error(), "error", err)
	}

	// Step 1: Get orgs
	orgs, err := grafana.GetOrgs()
	if err != nil {
		err := oops.Wrap(err)
		logger.Fatal(err.Error(), "error", err)
	}

	// Step 2: Get Dashboards
	iter.ForEach(orgs, func(orgPtr **models.OrgDTO){
		org := *orgPtr
		logger := logger.WithPrefix(fmt.Sprintf("(%d) %s", org.ID, org.Name))
		dashboards, err := grafana.GetDashboardsInOrg(org.ID)
		if err != nil {
			err = oops.
				With("orgID", org.ID).
				With("orgName", org.Name).
				Wrap(err)
			logger.Fatal(err.Error(), "error", err)
		}

		detailedDashboards, err := iter.MapErr(dashboards, func(dashPtr *mymodels.Dashboard) (*mymodels.DetailedDashboard, error) {
			dash := *dashPtr
			vars, err := grafana.GetVariablesInDashboard(dash.UID)
			if err != nil {
				err = oops.
					With("dash", dash).
					Wrap(err)
				return nil, err
			}

			logger.
				With("ID", dash.ID).
				With("UID", dash.UID).
				With("Title", dash.Title).
				Info("Found.")
			
			details := &mymodels.DetailedDashboard{
				Dashboard: dash,
				Variables: vars,
			}
			return details, nil
		})

		for _, dash := range detailedDashboards {
			logger.Info(dash.Variables)
		}
	})
}
