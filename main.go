package main

import (
	"log/slog"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/davecgh/go-spew/spew"
	"github.com/spf13/viper"
)

type Config struct {
	grr struct {
		url string
	}
	grafana struct {
		url   string
		token string
	}
}

var logger = log.NewWithOptions(os.Stdout, log.Options{
	Prefix:       "cli",
	ReportCaller: true,
})
var v = viper.NewWithOptions(viper.WithLogger(slog.New(logger)))

func main() {
	// Configure viper
	v.SetDefault("grr.url", "http://localhost:8989")
	v.SetDefault("grafana.url", "http://localhost:7000")
	v.SetDefault("grafana.token", "")

	v.SetConfigName("reporter")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")

	v.SetEnvPrefix("grr")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logger.Warn("No config file found; using just ENV.")
		} else {
			logger.Warn("A config file was found but a different error occured", "error", err)
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		log.Fatal("Couldn't unmarshall to struct", "error", err)
	}
	logger.Info("Configuration loaded", "config", spew.Sdump(config))
	logger.Info(spew.Sdump(v.AllKeys()))
	logger.Info(v.GetString("grafana.url"))
}
