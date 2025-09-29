package config

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"
)

const (
	macroPlaceholderString = "___CATERPILLAR___MACRO___%s___"
)

var (
	macroRegex = regexp.MustCompile(fmt.Sprintf(macroPlaceholderString, placeholderRegex))
	macroFuncs = map[string]func() string{
		"unixtime": func() string {
			return strconv.FormatInt(time.Now().Unix(), 10)
		},
		"timestamp": func() string {
			return time.Now().Format("2006-01-02T15:04:05Z07:00")
		},
		"microtimestamp": func() string {
			return strconv.FormatInt(time.Now().UnixNano()/1e3, 10)
		},
		"uuid": func() string {
			return uuid.NewString()
		},
	}
)

// return placeholder string for macro name
func setMacroPlaceholder(name string) (string, error) {

	if _, ok := macroFuncs[name]; !ok {
		return ``, fmt.Errorf("ERROR: macro '%s' is not defined in macro list", name)
	}

	return fmt.Sprintf(macroPlaceholderString, name), nil

}

// evaluateMacro replaces ___CATERPILLAR___MACRO___name___ patterns with their values
func evaluateMacro(data string) string {
	macroCache := make(map[string]string)

	result := macroRegex.ReplaceAllStringFunc(data, func(ph string) string {
		submatches := macroRegex.FindStringSubmatch(ph)

		if len(submatches) < 2 {
			return ph // leave as is if not matched
		}

		macroName := submatches[1]
		val, ok := macroCache[macroName]

		if !ok {
			val = macroFuncs[macroName]()
			macroCache[macroName] = val
		}

		return val
	})

	return result

}
