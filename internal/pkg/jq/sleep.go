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

	var seconds float64
	switch v := a[0].(type) {
	case float64:
		seconds = v
	case int:
		seconds = float64(v)
	default:
		return fmt.Errorf("sleep: argument must be a number, got %T", a[0])
	}

	fmt.Printf("Sleeping for %.0f seconds...\n", seconds)
	time.Sleep(time.Duration(seconds * float64(time.Second)))
	return c
}

func sleepOptions() []gojq.CompilerOption {
	return []gojq.CompilerOption{
		gojq.WithFunction("sleep", 1, 1, sleep),
	}
}
