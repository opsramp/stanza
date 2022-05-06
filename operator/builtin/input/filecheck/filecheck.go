package filecheck

import (
	"bufio"
	"context"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/opsramp/stanza/entry"
	"github.com/opsramp/stanza/operator/helper"
	"go.uber.org/zap"
)

// InputOperator is an operator that monitors files for entries
type InputOperator struct {
	helper.InputOperator

	finder                Finder
	FilePathField         entry.Field
	FileNameField         entry.Field
	FilePathResolvedField entry.Field
	FileNameResolvedField entry.Field
	PollInterval          time.Duration
	SplitFunc             bufio.SplitFunc
	MaxLogSize            int
	MaxConcurrentFiles    int
	SeenPaths             map[string]time.Time
	filenameRecallPeriod  time.Duration

	persister Persister

	queuedMatches []string
	maxBatchFiles int
	prevReaders   []*Reader

	startAtBeginning bool
	deleteAfterRead  bool

	CheckpointAt     int64
	FlushingInterval helper.Duration
	Delay            helper.Duration

	fingerprintSize int

	labelRegex *regexp.Regexp

	encoding helper.Encoding

	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// Start will start the file monitoring process
func (f *InputOperator) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel

	// Start polling goroutine
	f.startPoller(ctx)

	return nil
}

// Stop will stop the file monitoring process
func (f *InputOperator) Stop() error {
	f.cancel()
	f.wg.Wait()
	for _, reader := range f.prevReaders {
		reader.Close()
	}

	if err := f.persister.Close(); err != nil {
		f.Error(err)
	}
	f.cancel = nil
	return nil
}

// startPoller kicks off a goroutine that will poll the filesystem periodically,
// checking if there are new files or new logs in the watched files
func (f *InputOperator) startPoller(ctx context.Context) {

	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		globTicker := time.NewTicker(f.PollInterval)
		defer globTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-globTicker.C:
			}

			f.poll(ctx)
		}
	}()
}

// poll checks all the watched paths for new entries
func (f *InputOperator) poll(ctx context.Context) {

	// Get the list of paths on disk
	matches := f.finder.FindFiles()
	if len(matches) == 0 {
		f.Warnw("no files match the configured include patterns",
			"include", f.finder.Include,
			"exclude", f.finder.Exclude)
	}

	readers := f.makeReaders(ctx, matches)

	var wg sync.WaitGroup
	for _, reader := range readers {
		wg.Add(1)
		go func(r *Reader) {
			defer wg.Done()
			r.ReadToEnd(ctx)
		}(reader)
	}
	wg.Wait()

	if f.deleteAfterRead {
		f.Debug("cleaning up log files that have been fully consumed")
		for _, reader := range readers {
			if err := os.Remove(reader.file.Name()); err != nil {
				f.Errorf("could not delete %s", reader.file.Name())
			}
		}
		return
	}
	f.prevReaders = readers
}

// makeReaders takes a list of paths, then creates readers from each of those paths,
// discarding any that have a duplicate fingerprint to other files that have already
// been read this polling interval
func (f *InputOperator) makeReaders(ctx context.Context, filePaths []string) []*Reader {
	// Open the files first to minimize the time between listing and opening
	//TODO suppose we don't need it
	now := time.Now()
	cutoff := now.Add(f.filenameRecallPeriod * -1)
	for filename, lastSeenTime := range f.SeenPaths {
		if lastSeenTime.Before(cutoff) {
			delete(f.SeenPaths, filename)
		}
	}

	files := make([]*os.File, 0, len(filePaths))
	for _, path := range filePaths {

		file, err := os.Open(path) // #nosec - operator must read in files defined by user
		if err != nil {
			f.Errorw("Failed to open file", zap.Error(err))
			continue
		}
		files = append(files, file)
	}

	// Get fingerprints for each file
	fps := make([]*Fingerprint, 0, len(files))
	for _, file := range files {
		fp, err := f.NewFingerprint(file)
		if err != nil {
			f.Errorw("Failed creating fingerprint", zap.Error(err))
			continue
		}
		fps = append(fps, fp)
	}

	// Exclude any empty fingerprints or duplicate fingerprints to avoid doubling up on copy-truncate files
OUTER:
	for i := 0; i < len(fps); i++ {
		fp := fps[i]
		if len(fp.FirstBytes) == 0 {
			if err := files[i].Close(); err != nil {
				f.Errorf("problem closing file", "file", files[i].Name())
			}
			// Empty file, don't read it until we can compare its fingerprint, so just exclude
			fps = append(fps[:i], fps[i+1:]...)
			files = append(files[:i], files[i+1:]...)
			i--
			continue
		}
		// exclude fingerprint duplicates
		for j := i + 1; j < len(fps); j++ {
			fp2 := fps[j]
			if fp.StartsWith(fp2) || fp2.StartsWith(fp) {
				// Exclude
				fps = append(fps[:i], fps[i+1:]...)
				files = append(files[:i], files[i+1:]...)
				i--
				continue OUTER
			}
		}
	}

	// here we got all eligible files to process, and need to compare of what we had in previous poll
	readers := make([]*Reader, 0, len(fps))
	for i := 0; i < len(fps); i++ {

		reader, err := f.newReader(ctx, files[i], fps[i])
		if err != nil {
			f.Errorw("Failed to create reader", zap.Error(err))
			continue
		}
		readers = append(readers, reader)
	}

	return readers
}

func (f *InputOperator) newReader(ctx context.Context, file *os.File, fp *Fingerprint) (*Reader, error) {

	reader, ok := f.findFingerprintMatch(fp)
	if ok {
		return reader, nil
	}
	newReader, err := f.NewReader(file.Name(), file, fp, f.persister, f.FlushingInterval)
	if f.labelRegex != nil {
		newReader.ReadHeaders(ctx)
	}
	return newReader, err
}

func (f *InputOperator) findFingerprintMatch(fp *Fingerprint) (*Reader, bool) {
	// Iterate backwards to match newest first
	for i := len(f.prevReaders) - 1; i >= 0; i-- {
		oldReader := f.prevReaders[i]
		if fp.StartsWith(oldReader.Fingerprint) {
			return oldReader, true
		}
	}
	return nil, false
}
