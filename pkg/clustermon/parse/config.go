// Package parse configures files and domain files
package parse

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
)

// Config define the structure of the configuration file
type Config struct {
	NetMetrics []string `yaml:"net_metrics"`
	AggMetrics []string `yaml:"agg_metrics"`
	CluMetrics []string `yaml:"clu_metrics"`
	MonPeriod  int      `yaml:"mon_period"`
	Prometheus bool     `yaml:"prometheus"`
	LocalFile  bool     `yaml:"local_file"`
}

// ReadConfig read the configuration and return each entry
func ReadConfig(cfgFile string) ([]string, map[string]string, []string, int, bool, bool) {
	data, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		log.Fatal(err)
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatal(err)
	}
	aggMetrics := make(map[string]string)
	for i, metric := range config.NetMetrics {
		aggMetrics[metric] = config.AggMetrics[i]
	}
	return config.NetMetrics, aggMetrics, config.CluMetrics, config.MonPeriod, config.Prometheus, config.LocalFile
}
