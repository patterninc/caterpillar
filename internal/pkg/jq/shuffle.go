package jq

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/itchyny/gojq"
)

func shuffle(c any, _ []any) any {

	arr, ok := c.([]any)
	if !ok {
		return fmt.Errorf("shuffle: expected array, got %T", c)
	}

	r := make([]any, len(arr))
	copy(r, arr)

	rand.New(rand.NewSource(time.Now().UnixNano()))
	rand.Shuffle(len(r), func(i, j int) {
		r[i], r[j] = r[j], r[i]
	})

	return r

}

func shuffleOptions() []gojq.CompilerOption {
	return []gojq.CompilerOption{
		gojq.WithFunction("shuffle", 0, 0, shuffle),
	}
}
