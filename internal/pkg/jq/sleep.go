package jq

import (
	"fmt"
	"time"

	"github.com/itchyny/gojq"
)

func sleep(c any, a []any) any {
	if len(a) != 1 {
		return fmt.Errorf("sleep expects 1 argument")
	}

	durationStr, ok := a[0].(string)
	if !ok {
		return fmt.Errorf("sleep: argument must be a duration string, got %T", a[0])
	}

	dur, err := time.ParseDuration(durationStr)
	if err != nil {
		return fmt.Errorf("sleep: invalid duration format '%s': %v", durationStr, err)
	}

	fmt.Printf("Sleeping for %v...\n", dur)
	time.Sleep(dur)
	return c
}

func sleepOptions() []gojq.CompilerOption {
	return []gojq.CompilerOption{
		gojq.WithFunction("sleep", 1, 1, sleep),
	}
}
