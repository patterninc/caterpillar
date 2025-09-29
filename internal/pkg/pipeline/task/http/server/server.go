package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/patterninc/caterpillar/internal/pkg/duration"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

const (
	defaultPort         = 8080
	defaultReadTimeout  = duration.Duration(15 * time.Second)
	defaultWriteTimeout = duration.Duration(15 * time.Second)
	defaultIdleTimeout  = duration.Duration(5 * time.Second)
	addressFormat       = `:%d`
	defaultMethod       = `GET`
)

type pathConfig struct {
	Method string `yaml:"method,omitempty" json:"method,omitempty"`
	Path   string `yaml:"path,omitempty" json:"path,omitempty"`
}

type server struct {
	task.ServerBase `yaml:",inline" json:",inline"`

	Port         int               `yaml:"port,omitempty" json:"port,omitempty"`
	ReadTimeout  duration.Duration `yaml:"read_timeout,omitempty" json:"read_timeout,omitempty"`
	WriteTimeout duration.Duration `yaml:"write_timeout,omitempty" json:"write_timeout,omitempty"`
	IdleTimeout  duration.Duration `yaml:"idle_timeout,omitempty" json:"idle_timeout,omitempty"`
	Auth         *authBehavior     `yaml:"auth,omitempty" json:"auth,omitempty"`
	Paths        []pathConfig      `yaml:"paths,omitempty" json:"paths,omitempty"`
}

func New() (task.Task, error) {
	return &server{
		Port:         defaultPort,
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
		IdleTimeout:  defaultIdleTimeout,
		Paths: []pathConfig{{
			Method: defaultMethod,
			Path:   "/",
		}},
	}, nil
}

func (s *server) Run(input <-chan *record.Record, output chan<- *record.Record) error {

	if output != nil {
		defer close(output)
	}

	// input channel must be nil
	if input != nil {
		return task.ErrPresentInput
	}

	// validate and set default port if not provided
	if s.Port <= 0 {
		s.Port = defaultPort
	}

	// Create individual handlers for each configured path
	for _, pathConfig := range s.Paths {
		handler := s.createPathHandler(pathConfig, output)
		// Register the handler directly
		http.HandleFunc(pathConfig.Path, handler)
	}

	// Apply authentication middleware to all handlers at once
	var h http.Handler = http.DefaultServeMux
	if s.Auth != nil && s.Auth.Behavior != `` {
		authBehavior, found := map[string]func(http.Handler) http.Handler{
			`api-key`:      s.apiKeyMiddleware,
			`ip-whitelist`: s.ipWhitelistMiddleware,
			`basic-auth`:   s.basicAuthMiddleware,
		}[s.Auth.Behavior]

		if !found {
			return fmt.Errorf("unknown behavior: %s", s.Auth.Behavior)
		}
		h = authBehavior(h)
	}

	server := &http.Server{
		Addr:         fmt.Sprintf(addressFormat, s.Port),
		ReadTimeout:  time.Duration(s.ReadTimeout),
		WriteTimeout: time.Duration(s.WriteTimeout),
		IdleTimeout:  time.Duration(s.IdleTimeout),
		Handler:      h,
	}

	// if we have server shutdown requirement, let's set the timer...
	if s.EndAfter > 0 {
		go func() {
			time.Sleep(time.Duration(s.EndAfter))
			contextWithTimeout, cancel := context.WithTimeout(ctx, time.Duration(s.ReadTimeout*2))
			defer cancel()
			if err := server.Shutdown(contextWithTimeout); err != nil {
				// TODO: add proper error loging
				fmt.Println("Server Shutdown Error:", err)
			}
		}()
	}

	if err := server.ListenAndServe(); err != nil {
		if err != http.ErrServerClosed {
			return err
		}
	}

	return nil

}
