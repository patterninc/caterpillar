package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/babourine/x/pkg/process"

	"github.com/patterninc/caterpillar/internal/pkg/config"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline"
)

var (
	configFile string
)

func init() {

	flag.StringVar(&configFile, `conf`, ``, `config file`)
	flag.Parse()

	if configFile == `` {
		executableName, err := os.Executable()
		if err != nil {
			process.Bail(`executable`, err)
		}
		process.Bail(`usage`, fmt.Errorf("%s -conf <pipeline configuration>", executableName))
	}

}

func main() {

	p := &pipeline.Pipeline{}

	// load pipeline configuration
	if err := config.Load(configFile, p); err != nil {
		process.Bail(`config`, err)
	}

	if err := p.Init(); err != nil {
		process.Bail(`pipeline init`, err)
	}
	
	// run pipeline
	if err := p.Run(); err != nil {
		process.Bail(`pipeline`, err)
	}

}
