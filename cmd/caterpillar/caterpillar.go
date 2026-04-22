package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/babourine/x/pkg/process"

	"github.com/patterninc/caterpillar/internal/pkg/config"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline"
	"github.com/patterninc/caterpillar/internal/pkg/profile"
)

var (
	configFile    string
	profileDump   string
	profileServer string
)

func init() {

	flag.StringVar(&configFile, `conf`, ``, `config file`)
	flag.StringVar(&profileDump, `profile-dump`, ``, `directory or s3://bucket/prefix to write pprof files (cpu, heap, goroutine, block, mutex) on exit`)
	flag.StringVar(&profileServer, `profile-serve`, ``, `address for net/http/pprof server (e.g. :6060)`)
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

	if profileServer != `` {
		profile.Serve(profileServer)
	}

	bail := process.Bail
	if profileDump != `` {
		flush, err := profile.Dump(profileDump)
		if err != nil {
			process.Bail(`profile-dump`, err)
		}
		bail = func(ctx string, err error) {
			flush()
			process.Bail(ctx, err)
		}

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			s := <-sigChan
			flush()
			if s == syscall.SIGTERM {
				os.Exit(143)
			}
			os.Exit(130)
		}()
		defer flush()
	}

	p := &pipeline.Pipeline{}

	// load pipeline configuration
	if err := config.Load(configFile, p); err != nil {
		bail(`config`, err)
	}
	// run pipeline
	if err := p.Run(); err != nil {
		bail(`pipeline`, err)
	}

}
