package main

import (
	"compress/gzip"
	"context"
	"flag"
	"fmt"
	"io"
	"math"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/alexaandru/utils"
)

// Exit codes
const (
	Success = iota
	SetupFailed
	S3AuthError
	CmdLineOptionError
	CachingFailure
)

// max number of attempts to retry a failed upload.
const maxTries = 10

// test environment constant
const testEnv = "test"

// signature of an s3 uploader func
type uploader func(*sourceFile) error

// filesLists returns both the current files list as well as the difference from the old (cached) files list.
func filesLists() (utils.FileHashes, []string) {
	current := utils.FileHashesNew(opts.Source)
	old := utils.FileHashes{}
	old.Load(opts.CacheFile)
	diff := current.Diff(old)

	return current, diff
}

// upload fetches sourceFiles from uploads chan, attempts to upload them and enqueue the results to
// completed list. On failure it attempts to retry, up to maxTries per source file.
func upload(fn uploader, uploads chan *sourceFile, rejected *syncedList, wgUploads, wgWorkers *sync.WaitGroup) {
	defer wgWorkers.Done()

	for src := range uploads {
		src := src

		if opts.dryRun {
			say(fmt.Sprintf("Pretending to upload %s", src.fname), ".")
			wgUploads.Done()
			continue
		}

		err := fn(src)
		if err == nil {
			wgUploads.Done()
			say(fmt.Sprintf("Uploaded %s", src.fname), ".")
			continue
		}

		src.recordAttempt()
		if !src.retriable() || !isRecoverable(err) {
			rejected.add(src.fname)
			say(fmt.Sprintf("Failed to upload %s: %v", src.fname, err), "F")
			wgUploads.Done()
			continue
		}

		go func() {
			say(fmt.Sprintf("Retrying %s", src.fname), "r")
			wait := time.Duration(100.0*math.Pow(2, float64(src.attempts))) * time.Millisecond
			if appEnv == testEnv {
				wait = time.Nanosecond
			}
			<-time.After(wait)
			uploads <- src
		}()
	}
}

// s3putGen generates an S3 upload function using the S3Uploader interface.
// In test mode, it returns a no-op function.
// In production, it uses the global s3Uploader to perform actual uploads.
func s3putGen() uploader {
	return s3putGenWithUploader(s3Uploader)
}

// s3putGenWithUploader generates an S3 upload function using the provided uploader.
// This allows for dependency injection in tests.
func s3putGenWithUploader(u S3Uploader) uploader {
	if appEnv == testEnv && u == nil {
		return func(_ *sourceFile) error {
			return nil
		}
	}

	if u == nil {
		return func(_ *sourceFile) error {
			return fmt.Errorf("s3 uploader is not initialized")
		}
	}

	return func(src *sourceFile) (err error) {
		ctx := context.Background()
		f, err := os.Open(filepath.Join(opts.Source, src.fname))
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := f.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
		}()

		var body io.Reader = f
		contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(src.fname)))

		// Handle gzip compression
		if src.gzip {
			pr, pw := io.Pipe()
			gz := gzip.NewWriter(pw)

			go func() {
				if _, copyErr := io.Copy(gz, f); copyErr != nil {
					pw.CloseWithError(fmt.Errorf("compression error: %w", copyErr))
					return
				}
				if closeErr := gz.Close(); closeErr != nil {
					pw.CloseWithError(fmt.Errorf("gzip close error: %w", closeErr))
					return
				}
				// pw.Close() after successful gz.Close() typically doesn't fail.
				// If it does, the error will be caught by the Upload call.
				if closeErr := pw.Close(); closeErr != nil {
					// Can't call CloseWithError after Close, error will surface in Upload
					return
				}
			}()

			body = pr
		}

		input := &UploadInput{
			Bucket:               opts.BucketName,
			Key:                  src.fname,
			Body:                 body,
			ContentType:          &contentType,
			ContentEncoding:      src.getHeader(ContentEncoding),
			CacheControl:         src.getHeader(CacheControl),
			ServerSideEncryption: src.getHeader(Encryption),
		}

		_, err = u.Upload(ctx, input)
		return err
	}
}

func main() {
	if err := validateCmdLineFlags(opts); err != nil {
		fmt.Printf("Required field missing: %v.\n\nUsage:\n", err)
		flag.PrintDefaults()
		os.Exit(CmdLineOptionError)
	}

	s3put := s3putGen()

	uploads, rejected := make(chan *sourceFile), &syncedList{}
	wgUploads, wgWorkers := new(sync.WaitGroup), new(sync.WaitGroup)

	current, diff := filesLists()
	if len(diff) == 0 {
		say("Nothing to upload.", "Nothing to upload.\n")
		os.Exit(Success)
	}
	say(fmt.Sprintf("There are %d files to be uploaded to '%s'", len(diff), opts.BucketName), "Uploading ")

	if !opts.doUpload {
		say("Skipping upload")
		goto Cache
	}

	wgUploads.Add(len(diff))
	wgWorkers.Add(opts.WorkersCount)
	for i := 0; i < opts.WorkersCount; i++ {
		go upload(s3put, uploads, rejected, wgUploads, wgWorkers)
	}

	sort.Strings(diff)
	for _, fname := range diff {
		uploads <- newSourceFile(fname)
	}

	wgUploads.Wait()
	close(uploads)
	wgWorkers.Wait()
	say("Done uploading files.")

Cache:
	if !opts.doCache {
		say("Skipping cache.")
		goto Done
	}

	if opts.dryRun {
		say("Pretending to update cache.")
		goto Done
	}

	current = current.Reject(rejected.list)
	if err := current.Dump(opts.CacheFile); err != nil {
		fmt.Println("Caching failed: ", err)
		os.Exit(CachingFailure)
	}
	say("Done updating cache.")

Done:
	say("All done!", " done!\n")
}
