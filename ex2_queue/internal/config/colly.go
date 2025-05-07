package config

import (
	"github.com/gocolly/colly/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func NewColly(viper *viper.Viper, log *logrus.Logger) *colly.Collector {
	c := colly.NewCollector(
		colly.Async(true),
	)
	c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 4})

	return c
}
