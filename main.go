package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/grafana/grafana-openapi-client-go/models"
	"github.com/joho/godotenv"
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

	err := godotenv.Load()
	if err != nil {
		err := oops.Wrap(err)
		logger.Fatal(err.Error(), "error", err) //.WithAttrs(slog.Any("err", err)
	}
	
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

	logger.Info("Configuration loaded!",
		"grafana.url", v.GetString("grafana.url"),
		"grafana.user", v.GetString("grafana.user"),
		"grr.url", v.GetString("grr.url"),
	)

	// Step 0: Open Grafana
	grafana, err := MakeGrafanaClient(
		config.Grafana.Url,
		config.Grafana.Username,
		config.Grafana.Password,
	)
	if err != nil {
		logger.Fatal(err.Error(), "error", err)
	}

	ok, err := grafana.IsOK()
	if err != nil {
		logger.Error(err.Error(), "error", err)
	}
	if !ok {
		logger.Fatal("Grafana is NOT OK. Are the user creds correct?")
	}

	// Step 1: Get orgs
	orgs, err := grafana.GetOrgs()
	if err != nil {
		err := oops.Wrap(err)
		logger.Fatal(err.Error(), "error", err)
	}

	// step 1.5: Reorganize orgs
	orgMap := make(map[int64]*models.OrgDTO)
	for _, o := range orgs {
		orgMap[o.ID] = o
	}

	// Step 2: Get Dashboards per org
	dashLock := sync.Mutex{}
	dashMap := make(map[int64][]*mymodels.DetailedDashboard)
	_, err = iter.MapErr(orgs, func(orgPtr **models.OrgDTO) ([]*mymodels.DetailedDashboard, error) {
		org := *orgPtr
		logger := logger.WithPrefix(fmt.Sprintf("(%d) %s", org.ID, org.Name))
		dashboards, err := grafana.GetDashboardsInOrg(org.ID)
		if err != nil {
			return nil, oops.
				With("orgID", org.ID).
				With("orgName", org.Name).
				Wrap(err)
		}

		detailedDashboards, err := iter.MapErr(dashboards, func(dashPtr *mymodels.Dashboard) (*mymodels.DetailedDashboard, error) {
			dash := *dashPtr
			vars, err := grafana.GetVariablesInDashboard(dash.UID)
			if err != nil {
				return nil, oops.
					With("dash", dash).
					Wrap(err)
			}

			logger.
				With("ID", dash.ID).
				With("UID", dash.UID).
				With("Title", dash.Title).
				With("FolderTitle", dash.FolderTitle).
				Info("Found.")
			
			details := &mymodels.DetailedDashboard{
				Dashboard: dash,
				Variables: vars,
			}
			return details, nil
		})
		if err != nil {
			return nil, err
		}

		dashLock.Lock()
		dashMap[org.ID] = detailedDashboards
		dashLock.Unlock()

		return detailedDashboards, nil
	})

	// Step 3: Convert org+dashboard to GRR requests
	
}
