package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/kardianos/service"
	"github.com/opsramp/stanza/errors"
	"gopkg.in/fsnotify.v1"

	// This package registers its HTTP endpoints for profiling using an init hook
	_ "net/http/pprof" // #nosec
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"

	agent "github.com/opsramp/stanza/agent"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const (
	envStanzaLogFile      = "STANZA_LOG_FILE"
	envStanzaDatabaseFile = "STANZA_DATABASE_FILE"
)

// RootFlags are the root level flags that be provided when invoking stanza from the command line
type RootFlags struct {
	DatabaseFile       string
	ConfigFiles        []string
	PluginDir          string
	PprofPort          int
	CPUProfile         string
	CPUProfileDuration time.Duration
	MemProfile         string
	MemProfileDelay    time.Duration

	LogLevel      string
	LogFile       string
	MaxLogSize    int
	MaxLogBackups int
	MaxLogAge     int
	Debug         bool
}

// NewRootCmd will return a root level command
func NewRootCmd() *cobra.Command {
	rootFlags := &RootFlags{}

	root := &cobra.Command{
		Use:   "stanza [-c ./config.yaml]",
		Short: "A log parser and router",
		Long:  "A log parser and router",
		Args:  cobra.NoArgs,
		Run:   func(command *cobra.Command, args []string) { runRoot(command, args, rootFlags) },
	}

	rootFlagSet := root.PersistentFlags()
	rootFlagSet.StringVar(&rootFlags.LogLevel, "log_level", "INFO", "sets the agent's log level")
	rootFlagSet.StringVar(&rootFlags.LogFile, "log_file", "", "writes agent logs to a specified file")
	rootFlagSet.IntVar(&rootFlags.MaxLogSize, "max_log_size", 10, "sets the maximum size of agent log files in MB before rotating")
	rootFlagSet.IntVar(&rootFlags.MaxLogBackups, "max_log_backups", 5, "sets the maximum number of rotated log files to retain")
	rootFlagSet.IntVar(&rootFlags.MaxLogAge, "max_log_age", 7, "sets the maximum number of days to retain a rotated log file")
	rootFlagSet.BoolVar(&rootFlags.Debug, "debug", false, "debug logging flag - deprecated")

	rootFlagSet.StringSliceVarP(&rootFlags.ConfigFiles, "config", "c", []string{defaultConfig()}, "path to a config file")
	rootFlagSet.StringVar(&rootFlags.PluginDir, "plugin_dir", defaultPluginDir(), "path to the plugin directory")
	rootFlagSet.StringVar(&rootFlags.DatabaseFile, "database", "", "path to the stanza offset database")

	// Profiling flags
	rootFlagSet.IntVar(&rootFlags.PprofPort, "pprof_port", 0, "listen port for pprof profiling")
	rootFlagSet.StringVar(&rootFlags.CPUProfile, "cpu_profile", "", "path to cpu profile output")
	rootFlagSet.DurationVar(&rootFlags.CPUProfileDuration, "cpu_profile_duration", 60*time.Second, "duration to run the cpu profile")
	rootFlagSet.StringVar(&rootFlags.MemProfile, "mem_profile", "", "path to memory profile output")
	rootFlagSet.DurationVar(&rootFlags.MemProfileDelay, "mem_profile_delay", 10*time.Second, "time to wait before writing a memory profile")

	// Set profiling flags to hidden
	hiddenFlags := []string{"pprof_port", "cpu_profile", "cpu_profile_duration", "mem_profile", "mem_profile_delay"}
	for _, flag := range hiddenFlags {
		err := rootFlagSet.MarkHidden(flag)
		if err != nil {
			// MarkHidden only fails if the flag does not exist
			panic(err)
		}
	}

	root.AddCommand(NewGraphCommand(rootFlags))
	root.AddCommand(NewVersionCommand())
	root.AddCommand(NewOffsetsCmd(rootFlags))

	return root
}

func runRoot(command *cobra.Command, _ []string, flags *RootFlags) {
	// If log file flag is not set, use environment if set.
	if flags.LogFile == "" {
		flags.LogFile = os.Getenv(envStanzaLogFile)
	}

	// If database flag is not set, use environment if set.
	if flags.DatabaseFile == "" {
		flags.DatabaseFile = os.Getenv(envStanzaDatabaseFile)
	}

	logger := newLogger(*flags).Sugar()
	defer func() {
		_ = logger.Sync()
	}()
	ctx, cancel := context.WithCancel(command.Context())

	service, err := buildService(ctx, cancel, logger, flags)
	if err != nil {
		logger.Errorw("Failed to build agent", zap.Any("error", err))
		os.Exit(1)
	}

	watcher, err := fsnotify.NewWatcher()
	watcher.Add(flags.ConfigFiles[0])
	serviceWG := &sync.WaitGroup{}
	serviceWG.Add(1)

	if err != nil {
		logger.Errorw("Failed to run file watcher", zap.Any("error", err))
		os.Exit(1)
	}

	profilingWg := startProfiling(ctx, flags, logger)

	var sigChan = make(chan os.Signal, 3)
	signal.Notify(sigChan, syscall.SIGTERM, os.Interrupt)

	// Run file watcher to track changes in config
	go runWatcher(serviceWG, flags, logger, command, cancel)

	err = service.Run()
	if err != nil {
		logger.Errorw("Failed to run agent service", zap.Any("error", err))
		os.Exit(1)
	}

	serviceWG.Wait()
	profilingWg.Wait()
}

func runWatcher(serviceWG *sync.WaitGroup, flags *RootFlags, logger *zap.SugaredLogger, command *cobra.Command, cancel context.CancelFunc) {
	var sigChan = make(chan os.Signal, 3)
	signal.Notify(sigChan, syscall.SIGTERM, os.Interrupt)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Errorw("Failed to run watcher", zap.Any("error", err))
		os.Exit(1)
	}
	watcher.Add(flags.ConfigFiles[0])

	//we need this var to avoid it's shadowing
	var ctx context.Context

	for {
		select {
		case <-sigChan:
			serviceWG.Done()
			return
		case event, _ := <-watcher.Events:
			if event.Op == fsnotify.Write {
				logger.Debug("New configuration detected, reload pipeline...")
				cancel()
				ctx, cancel = context.WithCancel(command.Context())
				service, err := buildService(ctx, cancel, logger, flags)
				if err != nil {
					logger.Errorw("Failed to run new config", zap.Any("error", err))
					continue
				}
				go func() {
					err = service.Run()
					if err != nil {
						logger.Errorw("Failed to run agent service", zap.Any("error", err))
					}
				}()
			}
		}
	}

}

func buildService(ctx context.Context, cancel context.CancelFunc, logger *zap.SugaredLogger, flags *RootFlags) (service.Service, error) {

	agent, err := agent.NewBuilder(logger).
		WithConfigFiles(flags.ConfigFiles).
		WithPluginDir(flags.PluginDir).
		WithDatabaseFile(flags.DatabaseFile).
		Build()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to build agent")

	}
	service, err := newAgentService(ctx, agent, cancel)
	if err != nil {
		return nil, err
	}

	return service, nil

}

func startProfiling(ctx context.Context, flags *RootFlags, logger *zap.SugaredLogger) *sync.WaitGroup {
	wg := &sync.WaitGroup{}

	// Start pprof listening on port
	if flags.PprofPort != 0 {
		// pprof endpoints registered by importing net/pprof
		var srv http.Server
		srv.Addr = fmt.Sprintf(":%d", flags.PprofPort)

		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Info(srv.ListenAndServe())
		}()

		wg.Add(1)

		go func() {
			defer wg.Done()
			<-ctx.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err := srv.Shutdown(ctx)
			if err != nil {
				logger.Warnw("Errored shutting down pprof server", zap.Error(err))
			}
		}()
	}

	// Start CPU profile for configured duration
	if flags.CPUProfile != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()

			f, err := os.Create(flags.CPUProfile)
			if err != nil {
				logger.Errorw("Failed to create CPU profile", zap.Error(err))
				return
			}

			if err := pprof.StartCPUProfile(f); err != nil {
				log.Fatal("could not start CPU profile: ", err)
			}

			select {
			case <-ctx.Done():
			case <-time.After(flags.CPUProfileDuration):
			}
			pprof.StopCPUProfile()

			if f != nil {
				if err := f.Close(); err != nil { // #nosec G307
					logger.Errorf(err.Error())
				}
			}
		}()
	}

	// Start memory profile after configured delay
	if flags.MemProfile != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case <-ctx.Done():
			case <-time.After(flags.MemProfileDelay):
			}

			f, err := os.Create(flags.MemProfile)
			if err != nil {
				logger.Errorw("Failed to create memory profile", zap.Error(err))
			}

			runtime.GC() // get up-to-date statistics
			if err := pprof.WriteHeapProfile(f); err != nil {
				log.Fatal("could not write memory profile: ", err)
			}

			if f != nil {
				if err := f.Close(); err != nil {
					logger.Errorw("Failed to close file", zap.Error(err))
				}
			}
		}()
	}

	return wg
}
