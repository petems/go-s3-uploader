# Go S3 Uploader

[![Go Report Card](https://goreportcard.com/badge/github.com/petems/go-s3-uploader)](https://goreportcard.com/report/github.com/petems/go-s3-uploader)
[![GoDoc](https://godoc.org/github.com/petems/go-s3-uploader?status.png)](https://godoc.org/github.com/petems/go-s3-uploader)

> **Note**: This is a fork of [alexaandru/go3up](https://github.com/alexaandru/go3up) maintained to add improvements and updates. The original project appears to be unmaintained. All credit for the original implementation goes to Alexandru Ungur.

Go S3 Uploader is a small S3 uploader tool.

It was created in order to speed up S3 uploads by employing a local caching of files' md5 sums.
That way, on subsequent runs, the tool can compute a list of the files that changed since the
last upload and only upload those.

The initial use case was a large static site (with 10k+ files) that frequently changed only
a small subset of files (about ~100 routinely). In that particular case, the time reduction was significant.

On uploads with empty cache there may not be any benefit.

The current focus of the tool is just one way/uploads (without deleting things that were removed
locally, yet).

## Usage

Run `go-s3-uploader -h` to get the help. You can save your preferences to a .go-s3-uploader.json config file by
passing your command line flags as usual and adding "-save" at the end.

For authentication, see http://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html
as we pretty much support all of those options, in this order: shared profile; EC2 role; env vars.

## Migration from go3up

If you're migrating from the original `go3up` tool:

1. The binary name has changed from `go3up` to `go-s3-uploader`
2. Default config file changed from `.go3up.json` to `.go-s3-uploader.json`
3. Default cache file changed from `.go3up.txt` to `.go-s3-uploader.txt`

To continue using your existing cache file, specify it explicitly: `-cachefile=.go3up.txt`

## TODO

 - implement (optional) deletion of remote files missing on local.
