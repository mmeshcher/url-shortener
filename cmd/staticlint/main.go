package main

import (
	"github.com/mmeshcher/url-shortener/cmd/linter"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(linter.Analyzer)
}
