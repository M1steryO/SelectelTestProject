// This file must be in package main to be used as a golangci-lint Go plugin.
package main

import (
	"github.com/M1steryO/loglint/pkg/loglint"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/tools/go/analysis"
)

func New(conf any) ([]*analysis.Analyzer, error) {
	cfg := loglint.DefaultSettings()
	if conf != nil {
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			TagName:          "mapstructure",
			WeaklyTypedInput: true,
			Result:           &cfg,
		})
		if err != nil {
			return nil, err
		}
		if err := decoder.Decode(conf); err != nil {
			return nil, err
		}
	}
	return []*analysis.Analyzer{loglint.NewAnalyzer(cfg)}, nil
}
