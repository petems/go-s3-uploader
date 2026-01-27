package main

import (
	"bytes"
	"errors"
	"sync"
	"testing"
)

const (
	_ = iota
	noError
	recoverableError
	fatalError
)

func TestValidateCmdLineFlags(t *testing.T) {
	opts1 := &options{BucketName: "example_bucket", Source: "test/output", CacheFile: "test/.go3up.txt", Region: "us-west-1"}
	if err := validateCmdLineFlags(opts1); err != nil {
		t.Errorf("Expected %v to pass validation", opts1)
	}

	opts1 = &options{BucketName: "", Source: "test/output", CacheFile: "test/.go3up.txt"}
	if err := validateCmdLineFlags(opts1); err == nil {
		t.Error("Expected to fail validation")
	}
}

func TestValidateCmdLineFlag(t *testing.T) {
	if err := validateCmdLineFlag("output folder", "test/output"); err != nil {
		t.Error("Expected test/output to pass validation")
	}

	if err := validateCmdLineFlag("output folder", "test/bogus"); err == nil {
		t.Error("Expected test/bogus to fail validation")
	}

	if err := validateCmdLineFlag("Bucket Name", "foobar"); err != nil {
		t.Error("Expected foobar bucket name to pass validation")
	}

	if err := validateCmdLineFlag("Bucket Name", ""); err == nil {
		t.Error("Expected foobar bucket name to fail validation")
	}

	// Cache file is allowed to not exist (first-time users)
	if err := validateCmdLineFlag("Cache file", "test/nonexistent-cache.txt"); err != nil {
		t.Error("Expected non-existent cache file to pass validation")
	}

	// Existing cache file should also pass validation
	if err := validateCmdLineFlag("Cache file", "test/.go3up.txt"); err != nil {
		t.Error("Expected existing cache file to pass validation")
	}
}

func fakeUploaderGen(opts ...int) (uploader, *[]*sourceFile) {
	errorKind, m := noError, sync.Mutex{}
	if len(opts) > 0 {
		errorKind = opts[0]
	}

	out := &[]*sourceFile{}
	fn := func(src *sourceFile) error {
		m.Lock()
		*out = append(*out, src)
		m.Unlock()

		switch errorKind {
		case noError:
			return nil
		case recoverableError:
			return errors.New("Something something. " + recoverableErrorsSuffixes[0])
		default:
			return errors.New("Some made up error")
		}
	}

	return fn, out
}

var _ = func() bool {
	testing.Init()
	return true
}()

func init() {
	opts.BucketName = "example_bucket"
	opts.Source = "test/output"
	opts.CacheFile = "test/.go3up.txt"
	appEnv = "test"
	fakeBuffer := &bytes.Buffer{}
	sayLock := &sync.Mutex{}
	sayFn := loggerGen(fakeBuffer)
	say = func(msg ...string) {
		sayLock.Lock()
		defer sayLock.Unlock()
		sayFn(msg...)
	}
}
