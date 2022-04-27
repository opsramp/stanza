package filecheck

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/opsramp/stanza/entry"
	"github.com/opsramp/stanza/errors"
	"github.com/opsramp/stanza/operator/helper"
	"go.uber.org/zap"
	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"
)

// File labels contains information about file paths
type fileLabels struct {
	Name         string
	Path         string
	ResolvedName string
	ResolvedPath string
}

// resolveFileLabels resolves file labels
// and sets it to empty string in case of error
func (f *InputOperator) resolveFileLabels(path string) *fileLabels {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		f.Error(err)
	}

	abs, err := filepath.Abs(resolved)
	if err != nil {
		f.Error(err)
	}

	return &fileLabels{
		Path:         path,
		Name:         filepath.Base(path),
		ResolvedPath: abs,
		ResolvedName: filepath.Base(abs),
	}
}

type FileIdentifier struct {
	FingerPrint *Fingerprint
	Offset      int64
}

// Reader manages a single file
type Reader struct {
	Fingerprint      *Fingerprint
	persister        Persister
	path             string
	Offset           int64
	flushingInterval helper.Duration

	// HeaderLabels is an optional map that contains entry labels
	// derived from a log files' headers, added to every record
	HeaderLabels map[string]string

	generation int
	fileInput  *InputOperator
	file       *os.File
	fileLabels *fileLabels

	decoder      *encoding.Decoder
	decodeBuffer []byte

	*zap.SugaredLogger `json:"-"`
}

// NewReader creates a new file reader
func (f *InputOperator) NewReader(path string, file *os.File, fp *Fingerprint, persister Persister, flushingInterval helper.Duration) (*Reader, error) {
	r := &Reader{
		Fingerprint:      fp,
		persister:        persister,
		flushingInterval: flushingInterval,
		HeaderLabels:     make(map[string]string),
		file:             file,
		path:             path,
		fileInput:        f,
		SugaredLogger:    f.SugaredLogger.With("path", path),
		decoder:          f.encoding.Encoding.NewDecoder(),
		decodeBuffer:     make([]byte, 1<<12),
		fileLabels:       f.resolveFileLabels(path),
	}

	// this is new Reader, so it's his responsibility to find about his previous life, and resurrect
	if !f.checkCheckpointAndFP(r, path, fp) {
		r.initializeOffset(f.startAtBeginning)
	}
	return r, nil
}

// We need to check both checkpoint and fingerprint
func (f *InputOperator) checkCheckpointAndFP(reader *Reader, path string, fp *Fingerprint) bool {
	checkpoint := f.persister.Get(path)
	if checkpoint == nil {
		return false
	}
	id := new(FileIdentifier)
	dec := json.NewDecoder(bytes.NewReader(checkpoint))
	if err := dec.Decode(id); err != nil {
		f.Logger().Errorf("checkpont decoding failed, %s", err)
		return false
	}
	// if it's new fingerprint
	if !id.FingerPrint.StartsWith(fp) {
		return false
	}
	reader.Offset = id.Offset
	return true
}

// initializeOffset sets the starting offset
func (r *Reader) initializeOffset(startAtBeginning bool) error {
	if !startAtBeginning {
		info, err := r.file.Stat()
		if err != nil {
			return fmt.Errorf("stat: %s", err)
		}
		r.Offset = info.Size()
	}
	return nil
}

type consumerFunc func(context.Context, []byte) error

// ReadToEnd will read until the end of the file
func (r *Reader) ReadToEnd(ctx context.Context) {
	r.readFile(ctx, r.emit)
}

// ReadHeaders will read a files headers
func (r *Reader) ReadHeaders(ctx context.Context) {
	r.readFile(ctx, r.readHeaders)
}

func (r *Reader) readFile(ctx context.Context, consumer consumerFunc) {
	checkPointed := false
	var checkpointCounter int64
	if r.fileInput.CheckpointAt > 0 {
		checkPointed = true
	}
	if _, err := r.file.Seek(r.Offset, 0); err != nil {
		r.Errorw("Failed to seek", zap.Error(err))
		return
	}
	scanner := NewPositionalScanner(r, r.fileInput.MaxLogSize, r.Offset, r.fileInput.SplitFunc)

	flushingTicker := time.Tick(r.flushingInterval.Raw())

	// Iterate over the tokenized file
	for {

		select {
		case <-ctx.Done():
			return
		case <-flushingTicker:
			if err := r.encodeAndPutAndFlush(r.Offset); err != nil {
				r.Errorw("Failed to save at flushing interval", zap.Error(err))
			}

		default:
		}

		if ok := scanner.Scan(); !ok {
			if err := getScannerError(scanner); err != nil {
				r.Errorw("Failed during scan", zap.Error(err))
			}
			break
		}
		if err := consumer(ctx, scanner.Bytes()); err != nil {
			// return if header parsing is done
			if err == errEndOfHeaders {
				return
			}
			r.Error("Failed to consume entry", zap.Error(err))
		}
		r.Offset = scanner.Pos()

		if checkPointed {
			checkpointCounter++
			if checkpointCounter == r.fileInput.CheckpointAt {
				if err := r.encodeAndPutAndFlush(r.Offset); err != nil {
					r.Errorw("Failed to save", zap.Error(err))
				}
				checkpointCounter = 0
			}
		}

	}

	// when file completely read, we need to save offset
	if err := r.encodeAndPutAndFlush(r.Offset); err != nil {
		r.Errorw("Failed to save", zap.Error(err))
	}
}

func (r *Reader) encodeAndPutAndFlush(offset int64) error {
	encoded, err := r.encodeFileIdentifier(offset)
	if err != nil {
		r.Errorf("error encoding checkpoint for file %s, err:%s", r.path, err)
	}

	return r.persister.Put(r.path, encoded)
}

func (r *Reader) encodeFileIdentifier(offset int64) ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	fileID := &FileIdentifier{
		FingerPrint: r.Fingerprint,
		Offset:      offset,
	}
	if err := enc.Encode(fileID); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

var errEndOfHeaders = fmt.Errorf("finished header parsing, no header found")

func (r *Reader) readHeaders(_ context.Context, msgBuf []byte) error {
	byteMatches := r.fileInput.labelRegex.FindSubmatch(msgBuf)
	if len(byteMatches) != 3 {
		// return early, assume this failure means the file does not
		// contain anymore headers
		return errEndOfHeaders
	}
	matches := make([]string, len(byteMatches))
	for i, byteSlice := range byteMatches {
		matches[i] = string(byteSlice)
	}
	if r.HeaderLabels == nil {
		r.HeaderLabels = make(map[string]string)
	}
	r.HeaderLabels[matches[1]] = matches[2]
	return nil
}

// Close will close the file
func (r *Reader) Close() {
	if r.file != nil {
		if err := r.file.Close(); err != nil {
			r.Warnf("Problem closing reader", "Error", err.Error())
		}
	}
}

// Emit creates an entry with the decoded message and sends it to the next
// operator in the pipeline
func (r *Reader) emit(ctx context.Context, msgBuf []byte) error {
	// Skip the entry if it's empty
	if len(msgBuf) == 0 {
		return nil
	}

	msg, err := r.decode(msgBuf)
	if err != nil {
		return fmt.Errorf("decode: %s", err)
	}

	e, err := r.fileInput.NewEntry(msg)
	if err != nil {
		return fmt.Errorf("create entry: %s", err)
	}

	if err := e.Set(r.fileInput.FilePathField, r.fileLabels.Path); err != nil {
		return err
	}
	if err := e.Set(r.fileInput.FileNameField, filepath.Base(r.fileLabels.Path)); err != nil {
		return err
	}

	if err := e.Set(r.fileInput.FilePathResolvedField, r.fileLabels.ResolvedPath); err != nil {
		return err
	}
	if err := e.Set(r.fileInput.FileNameResolvedField, r.fileLabels.ResolvedName); err != nil {
		return err
	}

	// Set W3C headers as labels
	for k, v := range r.HeaderLabels {
		field := entry.NewLabelField(k)
		if err := e.Set(field, v); err != nil {
			return err
		}
	}

	r.fileInput.Write(ctx, e)
	return nil
}

// decode converts the bytes in msgBuf to utf-8 from the configured encoding
func (r *Reader) decode(msgBuf []byte) (string, error) {
	for {
		r.decoder.Reset()
		nDst, _, err := r.decoder.Transform(r.decodeBuffer, msgBuf, true)
		if err != nil && err == transform.ErrShortDst {
			r.decodeBuffer = make([]byte, len(r.decodeBuffer)*2)
			continue
		} else if err != nil {
			return "", fmt.Errorf("transform encoding: %s", err)
		}
		return string(r.decodeBuffer[:nDst]), nil
	}
}

func getScannerError(scanner *PositionalScanner) error {
	err := scanner.Err()
	if err == bufio.ErrTooLong {
		return errors.NewError("log entry too large", "increase max_log_size or ensure that multiline regex patterns terminate")
	} else if err != nil {
		return errors.Wrap(err, "scanner error")
	}
	return nil
}
func (r *Reader) Read(dst []byte) (int, error) {
	return r.file.Read(dst)
}
