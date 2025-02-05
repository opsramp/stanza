package filecheck

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/bmatcuk/doublestar/v2"
	backoff "github.com/cenkalti/backoff/v4"
	"github.com/opsramp/stanza/entry"
	"github.com/opsramp/stanza/operator"
	"github.com/opsramp/stanza/operator/builtin/leveldbpersister"
	"github.com/opsramp/stanza/operator/helper"
)

func init() {
	operator.Register("file_input_check", func() operator.Builder { return NewInputConfig("") })
}

const (
	defaultMaxLogSize           = 1024 * 1024
	defaultMaxConcurrentFiles   = 512
	defaultFilenameRecallPeriod = time.Minute
	defaultPollInterval         = 200 * time.Millisecond
	//defaultCheckPointAt         = 10
	defaultFlushingInterval = 1 * time.Minute
)

// NewInputConfig creates a new input config with default values
func NewInputConfig(operatorID string) *InputConfig {
	return &InputConfig{
		InputConfig:             helper.NewInputConfig(operatorID, "file_input"),
		PollInterval:            helper.Duration{Duration: defaultPollInterval},
		IncludeFileName:         true,
		IncludeFilePath:         false,
		IncludeFileNameResolved: false,
		IncludeFilePathResolved: false,
		StartAt:                 "end",
		FingerprintSize:         defaultFingerprintSize,
		MaxLogSize:              defaultMaxLogSize,
		MaxConcurrentFiles:      defaultMaxConcurrentFiles,
		Encoding:                helper.NewEncodingConfig(),
		FilenameRecallPeriod:    helper.Duration{Duration: defaultFilenameRecallPeriod},
	}
}

// InputConfig is the configuration of a file input operator
type InputConfig struct {
	helper.InputConfig      `yaml:",inline"`
	Finder                  `mapstructure:",squash" yaml:",inline"`
	Database                string                 `json:"database,omitempty" yaml:"database,omitempty"`
	PollInterval            helper.Duration        `json:"poll_interval,omitempty"               yaml:"poll_interval,omitempty"`
	FlushingInterval        helper.Duration        `json:"flushing_interval,omitempty"           yaml:"flushing_interval,omitempty"`
	CheckpointAt            int64                  `json:"checkpoint_at,omitempty"               yaml:"checkpoint_at,omitempty"`
	Delay                   helper.Duration        `json:"delay,omitempty" yaml:"delay,omitempty"`
	Multiline               helper.MultilineConfig `json:"multiline,omitempty"                   yaml:"multiline,omitempty"`
	IncludeFileName         bool                   `json:"include_file_name,omitempty"           yaml:"include_file_name,omitempty"`
	IncludeFilePath         bool                   `json:"include_file_path,omitempty"           yaml:"include_file_path,omitempty"`
	IncludeFileNameResolved bool                   `json:"include_file_name_resolved,omitempty"  yaml:"include_file_name_resolved,omitempty"`
	IncludeFilePathResolved bool                   `json:"include_file_path_resolved,omitempty"  yaml:"include_file_path_resolved,omitempty"`
	StartAt                 string                 `json:"start_at,omitempty"                    yaml:"start_at,omitempty"`
	FingerprintSize         helper.ByteSize        `json:"fingerprint_size,omitempty"            yaml:"fingerprint_size,omitempty"`
	MaxLogSize              helper.ByteSize        `json:"max_log_size,omitempty"                yaml:"max_log_size,omitempty"`
	MaxConcurrentFiles      int                    `json:"max_concurrent_files,omitempty"        yaml:"max_concurrent_files,omitempty"`
	DeleteAfterRead         bool                   `json:"delete_after_read,omitempty"           yaml:"delete_after_read,omitempty"`
	LabelRegex              string                 `json:"label_regex,omitempty"                 yaml:"label_regex,omitempty"`
	Encoding                helper.EncodingConfig  `json:",inline,omitempty"                     yaml:",inline,omitempty"`
	FilenameRecallPeriod    helper.Duration        `json:"filename_recall_period,omitempty"      yaml:"filename_recall_period,omitempty"`
}

// Build will build a file input operator from the supplied configuration
func (c InputConfig) Build(context operator.BuildContext) ([]operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(context)
	if err != nil {
		return nil, err
	}

	if len(c.Include) == 0 {
		return nil, fmt.Errorf("required argument `include` is empty")
	}

	// Ensure includes can be parsed as globs
	for _, include := range c.Include {
		_, err := doublestar.PathMatch(include, "matchstring")
		if err != nil {
			return nil, fmt.Errorf("parse include glob: %s", err)
		}
	}

	// Ensure excludes can be parsed as globs
	for _, exclude := range c.Exclude {
		_, err := doublestar.PathMatch(exclude, "matchstring")
		if err != nil {
			return nil, fmt.Errorf("parse exclude glob: %s", err)
		}
	}

	if c.MaxLogSize <= 0 {
		return nil, fmt.Errorf("`max_log_size` must be positive")
	}

	if c.MaxConcurrentFiles <= 1 {
		return nil, fmt.Errorf("`max_concurrent_files` must be greater than 1")
	}

	if c.FingerprintSize == 0 {
		c.FingerprintSize = defaultFingerprintSize
	} else if c.FingerprintSize < minFingerprintSize {
		return nil, fmt.Errorf("`fingerprint_size` must be at least %d bytes", minFingerprintSize)
	}

	encoding, err := c.Encoding.Build(context)
	if err != nil {
		return nil, err
	}

	splitFunc, err := c.Multiline.Build(context, encoding.Encoding, false)
	if err != nil {
		return nil, err
	}

	var startAtBeginning bool
	switch c.StartAt {
	case "beginning":
		startAtBeginning = true
	case "end":
		if c.DeleteAfterRead {
			return nil, fmt.Errorf("delete_after_read cannot be used with start_at 'end'")
		}
		startAtBeginning = false
	default:
		return nil, fmt.Errorf("invalid start_at location '%s'", c.StartAt)
	}

	var labelRegex *regexp.Regexp
	if c.LabelRegex != "" {
		r, err := regexp.Compile(c.LabelRegex)
		if err != nil {
			return nil, fmt.Errorf("compiling regex: %s", err)
		}

		keys := r.SubexpNames()
		// keys[0] is always the empty string
		if x := len(keys); x != 3 {
			return nil, fmt.Errorf("label_regex must contain two capture groups named 'key' and 'value', got %d capture groups", x)
		}

		hasKeys := make(map[string]bool)
		hasKeys[keys[1]] = true
		hasKeys[keys[2]] = true
		if !hasKeys["key"] || !hasKeys["value"] {
			return nil, fmt.Errorf("label_regex must contain two capture groups named 'key' and 'value'")
		}
		labelRegex = r
	}

	fileNameField := entry.NewNilField()
	if c.IncludeFileName {
		fileNameField = entry.NewLabelField("file_name")
	}

	filePathField := entry.NewNilField()
	if c.IncludeFilePath {
		filePathField = entry.NewLabelField("file_path")
	}

	fileNameResolvedField := entry.NewNilField()
	if c.IncludeFileNameResolved {
		fileNameResolvedField = entry.NewLabelField("file_name_resolved")
	}

	filePathResolvedField := entry.NewNilField()
	if c.IncludeFilePathResolved {
		filePathResolvedField = entry.NewLabelField("file_path_resolved")
	}

	op := &InputOperator{
		InputOperator:         inputOperator,
		finder:                c.Finder,
		SplitFunc:             splitFunc,
		persister:             initPersister(c, inputOperator),
		PollInterval:          c.PollInterval.Raw(),
		CheckpointAt:          c.CheckpointAt,
		Delay:                 c.Delay,
		FlushingInterval:      c.FlushingInterval,
		FilePathField:         filePathField,
		FileNameField:         fileNameField,
		FilePathResolvedField: filePathResolvedField,
		FileNameResolvedField: fileNameResolvedField,
		startAtBeginning:      startAtBeginning,
		deleteAfterRead:       c.DeleteAfterRead,
		queuedMatches:         make([]string, 0),
		labelRegex:            labelRegex,
		encoding:              encoding,
		cancel:                func() {},
		fingerprintSize:       int(c.FingerprintSize),
		MaxLogSize:            int(c.MaxLogSize),
		MaxConcurrentFiles:    c.MaxConcurrentFiles,
		SeenPaths:             make(map[string]time.Time, 100),
		filenameRecallPeriod:  c.FilenameRecallPeriod.Raw(),
	}

	return []operator.Operator{op}, nil
}

func initPersister(c InputConfig, op helper.InputOperator) Persister {
	if len(c.Database) > 0 {
		persister, err := leveldbpersister.NewLevelDBPersister(c.ID(), c.FlushingInterval, c.Database)

		// this is workaround for the cases when we get `resource not available` error after dynamic
		// reconfiguration, because of db file lock
		if err != nil {

			b := &backoff.ExponentialBackOff{
				InitialInterval:     200 * time.Millisecond,
				RandomizationFactor: backoff.DefaultRandomizationFactor,
				Multiplier:          backoff.DefaultMultiplier,
				MaxInterval:         1 * time.Second,
				MaxElapsedTime:      5 * time.Second,
				Stop:                backoff.Stop,
				Clock:               backoff.SystemClock,
			}
			b.Reset()

			for {
				waitTime := b.NextBackOff()
				op.Errorf("database connection failure, trying to reconnect in %q", waitTime)
				if waitTime == b.Stop {
					op.Errorw("Reached max backoff time for database connection", "file", c.Database)
					os.Exit(1)
				}

				<-time.After(waitTime)
				persister, err = leveldbpersister.NewLevelDBPersister(c.ID(), c.FlushingInterval, c.Database)
				if err != nil {
					continue
				}
				op.Debugf("database connection to %q established", c.Database)
				return persister
			}
		}
		return persister
	}
	op.Debugf("stub database is used")
	return leveldbpersister.NewStubDBPersister()

}
