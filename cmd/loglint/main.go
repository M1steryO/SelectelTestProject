package main

import (
	"github.com/M1steryO/loglint/pkg/loglint"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(loglint.NewAnalyzer(loglint.DefaultSettings()))
}
