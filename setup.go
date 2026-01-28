package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// isTestMode checks if the program is running under go test
func isTestMode() bool {
	// Check if running under go test by looking for test flags
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.") {
			return true
		}
	}
	return false
}

var opts = &options{
	WorkersCount: runtime.NumCPU() * 2,
	Source:       "output",
	CacheFile:    ".go-s3-uploader.txt",
	doUpload:     true,
	doCache:      true,
	Region:       os.Getenv("AWS_DEFAULT_REGION"),
	Profile:      os.Getenv("AWS_DEFAULT_PROFILE"),
	cfgFile:      ".go-s3-uploader.json",
}

var appEnv string

// AWS configuration and S3 client
var awsCfg aws.Config

var s3svc *s3.Client

var say func(...string)

// Order matters: first hit, first served.
// TODO: Make this configurable somehow, so that end users can provide their own mappings.
var r = regexp.MustCompile
var customHeadersDef = []pathToHeaders{
	{r("index\\.html"), headers{ContentEncoding: "gzip", CacheControl: "max-age=1800"}},       // 1800
	{r("articole.*\\.html$"), headers{ContentEncoding: "gzip", CacheControl: "max-age=3600"}}, // 86400
	{r("[^/]*\\.html$"), headers{ContentEncoding: "gzip", CacheControl: "max-age=3600"}},
	{r("\\.xml$"), headers{ContentEncoding: "gzip", CacheControl: "max-age=1800"}},
	{r("\\.ico$"), headers{ContentEncoding: "gzip", CacheControl: "max-age=31536000"}},
	{r("\\.(js|css)$"), headers{ContentEncoding: "gzip", CacheControl: "max-age=31536000"}},
	{r("images/articole/.*(jpg|JPG|png|PNG)$"), headers{CacheControl: "max-age=31536000"}},
	{r("\\.(jpg|JPG|png|PNG)$"), headers{CacheControl: "max-age=31536000"}},
}

// processCmdLineFlags wraps the command line flags handling.
func processCmdLineFlags(opts *options) {
	flag.IntVar(&opts.WorkersCount, "workers", opts.WorkersCount, "No. of workers to use for uploads")
	flag.StringVar(&opts.BucketName, "bucket", opts.BucketName, "Bucket to upload files to")
	flag.StringVar(&opts.Source, "source", opts.Source, "Source folder for files to be uploaded")
	flag.StringVar(&opts.CacheFile, "cachefile", opts.CacheFile, "Location of the cache file")
	flag.StringVar(&opts.Region, "region", opts.Region, "AWS region")
	flag.StringVar(&opts.Profile, "profile", opts.Profile, "AWS shared profile")
	flag.StringVar(&opts.cfgFile, "cfgfile", opts.cfgFile, "Config file location")
	flag.BoolVar(&opts.dryRun, "dry", opts.dryRun, "Dry run (do not upload/update cache)")
	flag.BoolVar(&opts.verbose, "verbose", opts.verbose, "Print the name of the files as they are uploaded")
	flag.BoolVar(&opts.quiet, "quiet", opts.quiet, "Print only warnings and/or errors")
	flag.BoolVar(&opts.doUpload, "upload", opts.doUpload, "Do perform an upload")
	flag.BoolVar(&opts.doCache, "cache", opts.doCache, "Do update the cache")
	flag.BoolVar(&opts.Encrypt, "encrypt", opts.Encrypt, "Encrypt files on server side")
	flag.BoolVar(&opts.saveCfg, "save", opts.saveCfg, "Saves the current commandline options to a config file")
	flag.BoolVar(&opts.version, "version", opts.version, "Print version information and exit")
	flag.Parse()
}

// validateCmdLineFlags validates some of the flags, mostly paths. Defers actual validation to validateCmdLineFlag()
func validateCmdLineFlags(opts *options) error {
	flags := map[string]string{
		"Bucket Name": opts.BucketName,
		"Source":      opts.Source,
		"Cache file":  opts.CacheFile,
	}
	for label, val := range flags {
		if err := validateCmdLineFlag(label, val); err != nil {
			return err
		}
	}
	return nil
}

// validateCmdLineFlag handles the actual validation of flags.
func validateCmdLineFlag(label, val string) error {
	switch label {
	case "Bucket Name":
		if val == "" {
			return fmt.Errorf("%s is not set", label)
		}
	case "Cache file":
		// Cache file is allowed to not exist (will be created on first run)
		if _, err := os.Stat(val); err != nil && !os.IsNotExist(err) {
			return err
		}
	default:
		_, err := os.Stat(val)
		return err
	}
	return nil
}

func initAWSClient() {
	ctx := context.Background()
	var err error

	// Build config options
	configOpts := []func(*config.LoadOptions) error{
		config.WithRetryMaxAttempts(3),
	}

	// Set region if specified
	if opts.Region != "" {
		configOpts = append(configOpts, config.WithRegion(opts.Region))
	}

	// Set shared profile if specified
	if opts.Profile != "" {
		configOpts = append(configOpts, config.WithSharedConfigProfile(opts.Profile))
	}

	// Load AWS config with credential chain (automatically includes: shared credentials, EC2 role, env vars)
	awsCfg, err = config.LoadDefaultConfig(ctx, configOpts...)
	if err != nil {
		abort(fmt.Errorf("failed to load AWS config: %w", err))
	}

	// Verify credentials are available
	_, err = awsCfg.Credentials.Retrieve(ctx)
	if err != nil {
		abort(fmt.Errorf("unable to initialize AWS credentials - please check environment: %w", err))
	}

	// Create S3 client
	s3svc = s3.NewFromConfig(awsCfg)
}

func abort(msg error) {
	say(msg.Error())
	os.Exit(SetupFailed)
}

func init() {
	// Skip full initialization in test mode - tests will set up their own mocks
	if isTestMode() {
		say = loggerGen()
		return
	}

	oldCfgFile := opts.cfgFile
	if err := opts.restore(opts.cfgFile); err != nil {
		abort(err)
	}
	processCmdLineFlags(opts)

	// Handle version flag early, before AWS initialization
	if opts.version {
		fmt.Println(GetVersion())
		os.Exit(Success)
	}

	if opts.cfgFile != oldCfgFile { // we were given a different config file, use that instead.
		if err := opts.restore(opts.cfgFile); err != nil {
			abort(err)
		}
	}
	if opts.saveCfg {
		if err := opts.dump(opts.cfgFile); err != nil {
			abort(err)
		}
	}
	appEnv = "production"
	say = loggerGen()
	initAWSClient()
}
