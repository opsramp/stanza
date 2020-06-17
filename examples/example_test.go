package example

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/bluemedora/bplogagent/commands"
	"github.com/bluemedora/bplogagent/plugin/builtin/output"
	"github.com/bluemedora/bplogagent/plugin/testutil"
	"github.com/stretchr/testify/require"
)

type muxWriter struct {
	buffer bytes.Buffer
	sync.Mutex
}

func (b *muxWriter) Write(p []byte) (n int, err error) {
	b.Lock()
	defer b.Unlock()
	return b.buffer.Write(p)
}

func (b *muxWriter) String() string {
	b.Lock()
	defer b.Unlock()
	return b.buffer.String()
}

func TestTomcatExample(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on windows because of service failures")
	}
	err := os.Chdir("./tomcat")
	require.NoError(t, err)
	defer func() {
		err := os.Chdir("../")
		require.NoError(t, err)
	}()

	_ = os.Remove("./logagent.db")

	tempDir := testutil.NewTempDir(t)

	cmd := commands.NewRootCmd()
	cmd.SetArgs([]string{"--database", filepath.Join(tempDir, "logagent.db")})

	buf := muxWriter{}
	output.Stdout = &buf

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		err = cmd.ExecuteContext(ctx)
		require.NoError(t, err)
	}()
	defer func() { <-done }()

	expected := `{"timestamp":"2019-03-13T10:43:00-04:00","record":{"bytes_sent":"-","http_method":"GET","http_status":"404","remote_host":"10.66.2.46","remote_user":"-","timestamp":"13/Mar/2019:10:43:00 -0400","url_path":"/"}}
{"timestamp":"2019-03-13T10:43:01-04:00","record":{"bytes_sent":"-","http_method":"GET","http_status":"404","remote_host":"10.66.2.46","remote_user":"-","timestamp":"13/Mar/2019:10:43:01 -0400","url_path":"/favicon.ico"}}
{"timestamp":"2019-03-13T10:43:08-04:00","record":{"bytes_sent":"-","http_method":"GET","http_status":"302","remote_host":"10.66.2.46","remote_user":"-","timestamp":"13/Mar/2019:10:43:08 -0400","url_path":"/manager"}}
{"timestamp":"2019-03-13T10:43:08-04:00","record":{"bytes_sent":"3420","http_method":"GET","http_status":"403","remote_host":"10.66.2.46","remote_user":"-","timestamp":"13/Mar/2019:10:43:08 -0400","url_path":"/manager/"}}
{"timestamp":"2019-03-13T11:00:26-04:00","record":{"bytes_sent":"2473","http_method":"GET","http_status":"401","remote_host":"10.66.2.46","remote_user":"-","timestamp":"13/Mar/2019:11:00:26 -0400","url_path":"/manager/html"}}
{"timestamp":"2019-03-13T11:00:53-04:00","record":{"bytes_sent":"11936","http_method":"GET","http_status":"200","remote_host":"10.66.2.46","remote_user":"tomcat","timestamp":"13/Mar/2019:11:00:53 -0400","url_path":"/manager/html"}}
{"timestamp":"2019-03-13T11:00:53-04:00","record":{"bytes_sent":"19698","http_method":"GET","http_status":"200","remote_host":"10.66.2.46","remote_user":"-","timestamp":"13/Mar/2019:11:00:53 -0400","url_path":"/manager/images/asf-logo.svg"}}
`

	timeout := time.After(5 * time.Second)
	for {
		select {
		case <-time.After(50 * time.Millisecond):
			if len(buf.String()) == len(expected) {
				require.Equal(t, expected, buf.String())
				cancel()
				return
			}
		case <-timeout:
			require.FailNow(t, "Timed out waiting for logs to be written to stdout")
		}
	}
}