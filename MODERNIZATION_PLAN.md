# Go S3 Uploader Modernization Plan

## Current State Analysis

**Go Version**: 1.13 (released September 2019)
**System Go Version**: 1.25.4
**AWS SDK**: v1 (deprecated)

## Recommended Improvements

### 1. Update Go Version

**Current**: `go 1.13`
**Recommended**: `go 1.24` (or latest stable)

**Benefits**:
- Generics support (Go 1.18+)
- Improved error handling with multiple return values
- Better performance and security fixes
- `any` type alias (replaces `interface{}`)
- Workspace mode support
- Enhanced fuzzing support

**Changes needed**:
```diff
- go 1.13
+ go 1.24
```

### 2. Migrate AWS SDK v1 → v2

**Current**: `github.com/aws/aws-sdk-go v1.34.32` (deprecated)
**Recommended**: `github.com/aws/aws-sdk-go-v2`

AWS SDK v1 is in maintenance mode. V2 offers:
- Better API design with context support
- Improved performance
- Modular package structure
- Better error handling
- Native context.Context support

**Required changes**:
- Update imports from `github.com/aws/aws-sdk-go` to `github.com/aws/aws-sdk-go-v2`
- Refactor credential chain configuration
- Update S3 client initialization
- Update upload API calls
- Add context.Context throughout

**Files affected**: `setup.go`, `main.go`

### 3. Update Dependencies

Run `go get -u` to update all dependencies:

| Package | Current | Latest | Status |
|---------|---------|--------|--------|
| aws-sdk-go | v1.34.32 | v1.55.8 | **Deprecated - migrate to v2** |
| golang.org/x/crypto | 2019 snapshot | v0.47.0 | Update available |
| golang.org/x/net | v0.0.0-20200202 | v0.49.0 | Update available |
| golang.org/x/text | v0.3.0 | v0.33.0 | Update available |
| gopkg.in/yaml.v2 | v2.2.8 | v2.4.0 | Update available |

### 4. Code Modernization Issues

#### 4.1 Error Handling

**Issue**: Using `errors.New()` with string concatenation
```go
// Current (setup.go:92)
return errors.New(label + " is not set")

// Modern
return fmt.Errorf("%s is not set", label)
```

**Issue**: Not using error wrapping with `%w`
```go
// Current (setup.go:33)
err = fmt.Errorf("%v; %v", err, err2)

// Modern
err = fmt.Errorf("%w; %w", err, err2)
```

**Issue**: Using panic for error handling
```go
// Current (main.go:111, 114, 117)
if _, err2 := io.Copy(wz, f); err2 != nil {
    panic(fmt.Errorf("decryption error: %v", err2))
}

// Modern - return errors properly through channels or context
```

#### 4.2 Context Usage

**Issue**: No context.Context usage for cancellation/timeouts

**Files affected**: `main.go`, `setup.go`

**Recommendations**:
- Add `context.Context` parameter to upload functions
- Support graceful shutdown via context cancellation
- Add timeout support for S3 operations
- Pass context through upload pipeline

```go
// Current
func upload(id string, fn uploader, uploads chan *sourceFile, ...)

// Modern
func upload(ctx context.Context, id string, fn uploader, uploads chan *sourceFile, ...)
```

#### 4.3 Control Flow

**Issue**: Using goto statements (main.go:167, 189, 194)
```go
// Current
goto Cache
// ...
Cache:
    if !opts.doCache {
        say("Skipping cache.")
        goto Done
    }

// Modern - use function decomposition
func handleCache() error { ... }
func handleUpload() error { ... }
```

#### 4.4 Sync Primitives

**Issue**: Using `new(sync.WaitGroup)` (main.go:156)
```go
// Current
wgUploads, wgWorkers := new(sync.WaitGroup), new(sync.WaitGroup)

// Modern
wgUploads, wgWorkers := &sync.WaitGroup{}, &sync.WaitGroup{}
```

#### 4.5 String Operations

**Issue**: Manual string concatenation in multiple places
```go
// Current (main.go:55, 70)
say("Pretending to upload "+src.fname, ".")
say("Failed to upload "+src.fname+": "+err.Error(), "F")

// Modern
say(fmt.Sprintf("Pretending to upload %s", src.fname), ".")
say(fmt.Sprintf("Failed to upload %s: %v", src.fname, err), "F")
```

#### 4.6 JSON Struct Tags

**Issue**: Inconsistent JSON naming (opts.go:10-16)
```go
// Current
type options struct {
    WorkersCount int    `json:",omitempty"`
    BucketName   string `json:",omitempty"`
    // ...
}

// Modern - explicit field names for clarity
type options struct {
    WorkersCount int    `json:"workers_count,omitempty"`
    BucketName   string `json:"bucket_name,omitempty"`
    Source       string `json:"source,omitempty"`
    CacheFile    string `json:"cache_file,omitempty"`
    Region       string `json:"region,omitempty"`
    Profile      string `json:"profile,omitempty"`
    Encrypt      bool   `json:"encrypt,omitempty"`
}
```

#### 4.7 Naked Returns

**Issue**: Using naked returns (opts.go:40, 46, 65, 70)
```go
// Current
func (o *options) dump(fname string) (err error) {
    // ...
    return
}

// Modern - explicit returns preferred for clarity
func (o *options) dump(fname string) error {
    // ...
    return err
}
```

#### 4.8 Type Naming

**Issue**: Unexported type `syncedlist` should follow Go conventions
```go
// Current (synced_list.go:5)
type syncedlist struct {

// Modern
type syncedList struct {
```

#### 4.9 Error Message Formatting

**Issue**: Using `.Error()` unnecessarily
```go
// Current (main.go:70)
say("Failed to upload "+src.fname+": "+err.Error(), "F")

// Modern - %v verb handles errors
say(fmt.Sprintf("Failed to upload %s: %v", src.fname, err), "F")
```

#### 4.10 Global Variables

**Issue**: Multiple package-level mutable globals (setup.go:19-50)

**Recommendation**: Consider moving to a struct for better testability
```go
type App struct {
    opts    *options
    sess    *session.Session
    s3svc   *s3.S3
    logger  func(...string)
    headers []pathToHeaders
}
```

### 5. Testing Improvements

**Current**: Tests require mock AWS credentials
**Recommended**: Use proper mocking with interfaces

```go
// Define interface for S3 operations
type S3Uploader interface {
    Upload(ctx context.Context, input *s3.UploadInput) (*s3.UploadOutput, error)
}

// Mock implementation for tests
type MockS3Uploader struct {
    uploads []string
}
```

### 6. Documentation

**Add**:
- Package-level documentation (already has `doc.go` - good!)
- Function documentation for exported functions
- Example usage in godoc format

### 7. Build and Tooling

**Add**:
- `.golangci.yml` for comprehensive linting
- GitHub Actions for CI/CD
- Goreleaser for releases
- Dependabot for dependency updates

### 8. CI/CD Implementation

#### 8.1 GitHub Actions Workflow

**Objective**: Automated testing and quality checks on all commits and pull requests

**Triggers**:
- Push to `main`/`master` branch
- Pull request opened/updated against `main`/`master`

**Jobs**:

##### Test Job
- **Matrix strategy**: Test against multiple Go versions (1.21, 1.22, 1.23, 1.24)
- **Steps**:
  1. Checkout code
  2. Setup Go with caching
  3. Download and verify dependencies
  4. Run tests with race detection and coverage
  5. Upload coverage to Codecov (optional)
- **Benefits**: Ensures compatibility across Go versions

##### Lint Job
- **Go version**: Latest stable (1.24)
- **Steps**:
  1. Checkout code
  2. Setup Go with caching
  3. Run golangci-lint with configured rules
- **Benefits**: Enforces code quality standards

##### Build Job
- **Matrix strategy**: Build for multiple platforms
  - OS: Linux, macOS, Windows
  - Architecture: amd64, arm64
- **Steps**:
  1. Checkout code
  2. Setup Go
  3. Cross-compile binaries
  4. Upload build artifacts (7-day retention)
- **Benefits**: Validates builds work on target platforms

#### 8.2 golangci-lint Configuration

**Enabled Linters**:
- **Error handling**: errcheck, errorlint
- **Code quality**: gosimple, govet, staticcheck, revive
- **Security**: gosec
- **Style**: gofmt, goimports, misspell, gocritic
- **Performance**: ineffassign, unused, unconvert
- **Complexity**: gocyclo (threshold: 15)

**Special Configurations**:
- G304 excluded (file path inputs are expected in CLI tools)
- G401 excluded (MD5 used for checksums, not cryptography)
- Relaxed rules for test files
- Legacy issue exclusions during migration phase

#### 8.3 Code Coverage

**Current**: No coverage tracking
**Target**: >70% coverage

**Integration**:
- Coverage reports generated on each test run
- Codecov integration (optional)
- Coverage badge in README

#### 8.4 Future Enhancements

**Dependabot** (`.github/dependabot.yml`):
```yaml
version: 2
updates:
  - package-ecosystem: "gomod"
    directory: "/"
    schedule:
      interval: "weekly"
    open-pull-requests-limit: 5
```

**GoReleaser** (`.goreleaser.yml`):
- Automated releases when tags are pushed
- Multi-platform binary distribution
- Homebrew tap support
- Docker image publishing
- Changelog generation

**Pre-commit Hooks**:
```bash
# Install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
# .git/hooks/pre-commit
#!/bin/bash
golangci-lint run
go test ./...
```

#### 8.5 Branch Protection Rules

**Recommended settings for `main` branch**:
- ✅ Require pull request reviews (1 approval)
- ✅ Require status checks to pass:
  - `test` (all Go versions)
  - `lint`
  - `build` (all platforms)
- ✅ Require branches to be up to date
- ✅ Require conversation resolution before merging
- ✅ Do not allow bypassing the above settings

#### 8.6 Migration Path

**Step 1**: Create workflow files (CI pipeline)
```bash
.github/
├── workflows/
│   └── ci.yml
└── dependabot.yml (optional)
```

**Step 2**: Create linter configuration
```bash
.golangci.yml
```

**Step 3**: Fix existing linting issues
- Run `golangci-lint run --fix` for auto-fixable issues
- Manually address remaining issues
- Add temporary exclusions if needed for legacy code

**Step 4**: Enable GitHub Actions
- Push workflow files
- Verify jobs execute successfully
- Configure branch protection rules

**Step 5**: Add badges to README.md
```markdown
[![CI](https://github.com/petems/go-s3-uploader/workflows/CI/badge.svg)](https://github.com/petems/go-s3-uploader/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/petems/go-s3-uploader)](https://goreportcard.com/report/github.com/petems/go-s3-uploader)
[![codecov](https://codecov.io/gh/petems/go-s3-uploader/branch/main/graph/badge.svg)](https://codecov.io/gh/petems/go-s3-uploader)
```

## Implementation Priority

### Phase 1: Foundation (Low Risk)
1. ✅ Update go.mod to Go 1.24
2. ✅ Update non-AWS dependencies
3. ✅ Run `go mod tidy`
4. ✅ Fix linting issues (string concat, error formatting)
5. ✅ Add explicit JSON tags
6. ✅ Fix type naming conventions

### Phase 2: Code Quality (Medium Risk)
1. ✅ Refactor goto statements to functions
2. ✅ Replace panic with proper error handling
3. ✅ Add context.Context support
4. ✅ Improve error wrapping with %w
5. ✅ Fix naked returns

### Phase 3: Major Changes (High Risk - Breaking)
1. ✅ Migrate AWS SDK v1 → v2
2. ✅ Refactor global state to struct
3. ✅ Add proper interfaces for testing
4. ✅ Update tests to use mocks

### Phase 4: Infrastructure
1. ✅ Add `.golangci.yml` configuration with sensible defaults
2. ✅ Create `.github/workflows/ci.yml` with test, lint, and build jobs
3. ✅ Configure matrix builds for multiple Go versions (1.21-1.24)
4. ✅ Configure cross-platform builds (Linux, macOS, Windows)
5. ✅ Run golangci-lint and fix auto-fixable issues
6. ✅ Add CI status badges to README
7. ✅ Configure branch protection rules on GitHub
8. ⬜ Add Dependabot configuration (optional)
9. ⬜ Add goreleaser config for releases (future)

## Breaking Changes

The following changes will require user action:

1. **Config file format**: JSON field names will change (snake_case)
   - Migration: Regenerate config with `-save` flag

2. **Minimum Go version**: Will require Go 1.21+ to build
   - Users need to upgrade their Go installation

3. **AWS SDK v2**: Different credential configuration
   - Should be transparent for most users

## Testing Strategy

1. Run existing tests after each phase
2. Test with actual S3 bucket (test environment)
3. Verify backward compatibility of cache files
4. Test config file migration

## Success Metrics

- [ ] All tests pass with Go 1.24
- [ ] No deprecated dependencies
- [ ] golangci-lint passes with strict settings
- [ ] No panics in upload pipeline
- [ ] Context cancellation works correctly
- [ ] AWS SDK v2 integration functional
- [ ] Performance equivalent or better than current
- [ ] CI pipeline runs successfully on all commits
- [ ] Tests pass on Linux, macOS, and Windows
- [ ] Multi-version Go testing (1.21-1.24) passes
- [ ] Code coverage tracked and visible

## Notes

- Consider keeping a v1 branch for users who can't upgrade immediately
- Document migration path in README
- Update CLAUDE.md with new architecture after changes
