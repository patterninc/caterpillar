package sftp

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	pkgsftp "github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/patterninc/caterpillar/internal/pkg/config"
	"github.com/patterninc/caterpillar/internal/pkg/duration"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

// The package is named sftp, which collides with the github.com/pkg/sftp
// client package, so we import the library under the pkgsftp alias.

const (
	defaultPort       = 22
	defaultTimeout    = duration.Duration(30 * time.Second)
	defaultMaxRetries = 3
	defaultRetryDelay = duration.Duration(1 * time.Second)

	opUpload   = `upload`
	opDownload = `download`
	opList     = `list`
	opMove     = `move`
	opDelete   = `delete`
)

// ctx is a package-level background context used when creating source records,
// mirroring the file task (internal/pkg/pipeline/task/file/file.go).
var ctx = context.Background()

type sftp struct {
	task.Base `yaml:",inline" json:",inline"`

	// Operation selects the behaviour. upload is a sink (consumes records),
	// download and list are sources (emit records), move and delete are
	// one-shot actions that may optionally pass records through.
	Operation string `yaml:"operation" json:"operation" validate:"required,oneof=upload download list move delete"`

	// Connection.
	Host     string `yaml:"host" json:"host" validate:"required"`
	Port     int    `yaml:"port,omitempty" json:"port,omitempty"`
	Username string `yaml:"username" json:"username" validate:"required"`

	// Authentication: exactly one of Password or PrivateKey. These come from
	// SSM via {{ secret }} in the pipeline YAML; never log them.
	Password   string `yaml:"password,omitempty" json:"password,omitempty"`
	PrivateKey string `yaml:"private_key,omitempty" json:"private_key,omitempty"`
	Passphrase string `yaml:"passphrase,omitempty" json:"passphrase,omitempty"`

	// Host key verification (secure by default — see hostkey.go).
	HostKey                  string `yaml:"host_key,omitempty" json:"host_key,omitempty"`
	KnownHostsPath           string `yaml:"known_hosts_path,omitempty" json:"known_hosts_path,omitempty"`
	InsecureSkipHostKeyCheck bool   `yaml:"insecure_skip_host_key_check,omitempty" json:"insecure_skip_host_key_check,omitempty"`

	// Paths. config.String supports {{ macro }}/{{ context }} templating, so
	// they can be evaluated per record.
	RemotePath      config.String `yaml:"remote_path,omitempty" json:"remote_path,omitempty"`
	DestinationPath config.String `yaml:"destination_path,omitempty" json:"destination_path,omitempty"`

	// Reliability.
	Timeout    duration.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	MaxRetries int               `yaml:"max_retries,omitempty" json:"max_retries,omitempty"`
	RetryDelay duration.Duration `yaml:"retry_delay,omitempty" json:"retry_delay,omitempty"`

	// Prepared in Init, used in Run.
	authMethod ssh.AuthMethod
	hostKeyCB  ssh.HostKeyCallback
}

func New() (task.Task, error) {
	return &sftp{
		Port:       defaultPort,
		Timeout:    defaultTimeout,
		MaxRetries: defaultMaxRetries,
		RetryDelay: defaultRetryDelay,
	}, nil
}

// Init validates the credentials and host-key settings and prepares the SSH
// auth method and host-key callback. It deliberately does NOT open a network
// connection: Init runs for every task at config-load time, and an SSH session
// held open from then until Run could time out. We dial in Run instead.
func (s *sftp) Init() error {

	authMethod, err := s.buildAuthMethod()
	if err != nil {
		return err
	}
	s.authMethod = authMethod

	hostKeyCB, err := s.buildHostKeyCallback()
	if err != nil {
		return err
	}
	s.hostKeyCB = hostKeyCB

	return nil

}

func (s *sftp) Run(input <-chan *record.Record, output chan<- *record.Record) error {

	if err := s.validateChannels(input, output); err != nil {
		return err
	}

	sshClient, sftpClient, err := s.connect()
	if err != nil {
		return err
	}
	// LIFO: the SFTP subsystem is torn down before the SSH transport.
	defer sshClient.Close()
	defer sftpClient.Close()

	switch s.Operation {
	case opUpload:
		return s.upload(sftpClient, input, output)
	case opDownload:
		return s.download(sftpClient, output)
	case opList:
		return s.list(sftpClient, output)
	case opMove:
		return s.move(sftpClient, input, output)
	case opDelete:
		return s.remove(sftpClient, input, output)
	default:
		return fmt.Errorf(`unsupported operation: %s`, s.Operation)
	}

}

// validateChannels enforces each operation's pipeline role so a misplaced task
// fails fast with a clear message instead of silently doing nothing.
func (s *sftp) validateChannels(input <-chan *record.Record, output chan<- *record.Record) error {

	switch s.Operation {
	case opUpload:
		if input == nil {
			return fmt.Errorf(`operation %q is a sink and requires an input: place it after a task that emits files (e.g. a file task reading s3://)`, opUpload)
		}
	case opDownload, opList:
		if input != nil {
			return fmt.Errorf(`operation %q is a source and must not have an input`, s.Operation)
		}
		if output == nil {
			return fmt.Errorf(`operation %q is a source and requires an output: place a task after it`, s.Operation)
		}
	}

	return nil

}

// connect dials the SSH transport and opens an SFTP session, retrying on
// transient failures. The dial honours Timeout so a hung server does not block
// the pipeline indefinitely.
func (s *sftp) connect() (*ssh.Client, *pkgsftp.Client, error) {

	addr := net.JoinHostPort(s.Host, strconv.Itoa(s.Port))

	clientConfig := &ssh.ClientConfig{
		User:            s.Username,
		Auth:            []ssh.AuthMethod{s.authMethod},
		HostKeyCallback: s.hostKeyCB,
		Timeout:         time.Duration(s.Timeout),
	}

	var (
		sshClient  *ssh.Client
		sftpClient *pkgsftp.Client
	)

	err := s.retry(`connect`, func() error {
		c, err := ssh.Dial(`tcp`, addr, clientConfig)
		if err != nil {
			return fmt.Errorf(`dialing %s: %w`, addr, err)
		}
		sc, err := pkgsftp.NewClient(c)
		if err != nil {
			c.Close()
			return fmt.Errorf(`opening sftp session on %s: %w`, addr, err)
		}
		sshClient, sftpClient = c, sc
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return sshClient, sftpClient, nil

}
