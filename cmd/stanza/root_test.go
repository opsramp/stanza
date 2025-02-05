package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/stretchr/testify/require"
)

func TestRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Skipping for windows, because it returns an unexplained error.
		// "The service process could not connect to the service controller"
		// This error does not occur when running the binary directly.
		t.Skip("Skipping root test on windows")
	}

	tempDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	input := []byte("log1\nlog2\nlog3\n")
	err = ioutil.WriteFile(filepath.Join(tempDir, "input.log"), input, 0666)
	require.NoError(t, err)

	dbPath := filepath.Join(tempDir, "logagent.db")
	inputPath := filepath.Join(tempDir, "input.log")
	outputPath := filepath.Join(tempDir, "output.json")
	configPath := filepath.Join(tempDir, "config.yaml")

	config := `
pipeline:
  - id: file_input
    type: file_input
    include: ['%s']
    write_to: message
    start_at: beginning
    output: file_output

  - id: file_output
    type: file_output
    path: '%s'
`
	config = fmt.Sprintf(config, inputPath, outputPath)
	err = ioutil.WriteFile(configPath, []byte(config), 0666)
	require.NoError(t, err)

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{"-c", configPath, "--database", dbPath})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	go func() {
		err := rootCmd.ExecuteContext(ctx)
		require.NoError(t, err)
	}()

	expectedPattern := `{"timestamp":".*","severity":0,"labels":{"file_name":"input.log"},"record":{"message":"log1"}}
{"timestamp":".*","severity":0,"labels":{"file_name":"input.log"},"record":{"message":"log2"}}
{"timestamp":".*","severity":0,"labels":{"file_name":"input.log"},"record":{"message":"log3"}}
`

	time.Sleep(1000 * time.Millisecond)

	actual, err := ioutil.ReadFile(outputPath)
	require.NoError(t, err)

	require.Regexp(t, expectedPattern, string(actual))
}

func TestFileWatcher(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	testFile := filepath.Join(tempDir, "test.txt")

	input := []byte("Test\n")
	err = ioutil.WriteFile(testFile, input, 0666)
	require.NoError(t, err)

	watcher, err := fsnotify.NewWatcher()
	require.NoError(t, err)
	defer watcher.Close()

	err = watcher.Add(testFile)
	require.NoError(t, err)

	err = ioutil.WriteFile(testFile, input, 0666)
	event, _ := <-watcher.Events
	require.Equal(t, event.Op, fsnotify.Chmod)

	event, _ = <-watcher.Events
	require.Equal(t, event.Op, fsnotify.Write)

	err = os.Remove(testFile)
	require.NoError(t, err)

}
