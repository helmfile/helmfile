package remote

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"net/http"
	neturl "net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hashicorp/go-getter/v2"
	"github.com/hashicorp/go-getter/v2/helper/url"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/envvar"
	"github.com/helmfile/helmfile/pkg/filesystem"
)

var (
	protocols               = []string{"s3", "http", "https"}
	disableInsecureFeatures bool
	awsSDKLogLevel          string
)

func init() {
	disableInsecureFeatures, _ = strconv.ParseBool(os.Getenv(envvar.DisableInsecureFeatures))
	// Read AWS SDK log level configuration
	// Default to "off" for security if not specified
	awsSDKLogLevel = strings.TrimSpace(os.Getenv(envvar.AWSSDKLogLevel))
	if awsSDKLogLevel == "" {
		awsSDKLogLevel = "off"
	}
}

func CacheDir() string {
	if h := os.Getenv(envvar.CacheHome); h != "" {
		return h
	}

	dir, err := os.UserCacheDir()
	if err != nil {
		// fall back to relative path with hidden directory
		return ".helmfile"
	}
	return filepath.Join(dir, "helmfile")
}

type Remote struct {
	Logger *zap.SugaredLogger

	// Home is the directory in which remote downloads files. If empty, user cache directory is used
	Home string

	// Getter is the underlying implementation of getter used for fetching remote files
	Getter Getter

	S3Getter Getter

	HttpGetter Getter

	// Filesystem abstraction
	// Inject any implementation of your choice, like an im-memory impl for testing, os.ReadFile for the real-world use.
	fs *filesystem.FileSystem
}

// Locate takes an URL to a remote file or a path to a local file.
// If the argument was an URL, it fetches the remote directory contained within the URL,
// and returns the path to the file in the fetched directory
func (r *Remote) Locate(urlOrPath string, cacheDirOpt ...string) (string, error) {
	if r.fs.FileExistsAt(urlOrPath) || r.fs.DirectoryExistsAt(urlOrPath) {
		return urlOrPath, nil
	}
	fetched, err := r.Fetch(urlOrPath, cacheDirOpt...)
	if err != nil {
		if _, ok := err.(InvalidURLError); ok {
			return urlOrPath, nil
		}
		return "", err
	}
	return fetched, nil
}

type InvalidURLError struct {
	err string
}

func (e InvalidURLError) Error() string {
	return e.err
}

type Source struct {
	Getter, Scheme, User, Host, Dir, File, RawQuery string
}

func IsRemote(goGetterSrc string) bool {
	if filepath.IsAbs(goGetterSrc) {
		return false
	}
	if _, err := Parse(goGetterSrc); err != nil {
		return false
	}
	return true
}

func Parse(goGetterSrc string) (*Source, error) {
	items := strings.Split(goGetterSrc, "::")
	var getter string
	if len(items) == 2 {
		getter = items[0]
		goGetterSrc = items[1]
	} else {
		items = strings.Split(goGetterSrc, "://")

		if len(items) == 2 {
			return ParseNormal(goGetterSrc)
		}
	}

	// Local absolute paths (e.g., C:\path on Windows, /path on Unix)
	// are not remote URLs. Reject them before go-getter's url.Parse
	// can misinterpret Windows drive letters as URL schemes.
	if filepath.IsAbs(goGetterSrc) {
		return nil, InvalidURLError{err: fmt.Sprintf("parse url: local absolute path is not a remote URL: %s", goGetterSrc)}
	}

	u, err := url.Parse(goGetterSrc)
	if err != nil {
		return nil, InvalidURLError{err: fmt.Sprintf("parse url: %v", err)}
	}

	if u.Scheme == "" {
		return nil, InvalidURLError{err: fmt.Sprintf("parse url: missing scheme - probably this is a local file path? %s", goGetterSrc)}
	}

	pathComponents := strings.Split(u.Path, "@")
	if len(pathComponents) != 2 {
		dir := filepath.Dir(u.Path)
		if len(dir) > 0 {
			dir = dir[1:]
		}
		pathComponents = []string{dir, filepath.Base(u.Path)}
	}

	return &Source{
		Getter:   getter,
		User:     u.User.String(),
		Scheme:   u.Scheme,
		Host:     u.Host,
		Dir:      pathComponents[0],
		File:     pathComponents[1],
		RawQuery: u.RawQuery,
	}, nil
}

func ParseNormal(path string) (*Source, error) {
	_, err := ParseNormalProtocol(path)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(u.Path)
	if len(dir) > 0 {
		dir = dir[1:]
	}

	return &Source{
		Getter:   "normal",
		User:     u.User.String(),
		Scheme:   u.Scheme,
		Host:     u.Host,
		Dir:      dir,
		File:     filepath.Base(u.Path),
		RawQuery: u.RawQuery,
	}, nil
}

func ParseNormalProtocol(path string) (string, error) {
	parts := strings.Split(path, "://")

	if len(parts) == 0 {
		return "", fmt.Errorf("failed to parse URL %s", path)
	}
	protocol := strings.ToLower(parts[0])

	if slices.Contains(protocols, protocol) {
		return protocol, nil
	}
	return "", fmt.Errorf("failed to parse URL %s", path)
}

func (r *Remote) Fetch(path string, cacheDirOpt ...string) (string, error) {
	u, err := Parse(path)
	if err != nil {
		return "", err
	}

	// Block remote access if insecure features are disabled and the source is remote
	if disableInsecureFeatures && IsRemote(path) {
		return "", fmt.Errorf("remote sources are disabled due to 'HELMFILE_DISABLE_INSECURE_FEATURES'")
	}

	srcDir := fmt.Sprintf("%s://%s/%s", u.Scheme, u.Host, u.Dir)
	file := u.File

	r.Logger.Debugf("remote> getter: %s", u.Getter)
	r.Logger.Debugf("remote> scheme: %s", u.Scheme)
	r.Logger.Debugf("remote> user: %s", u.User)
	r.Logger.Debugf("remote> host: %s", u.Host)
	r.Logger.Debugf("remote> dir: %s", u.Dir)
	r.Logger.Debugf("remote> file: %s", u.File)

	// This should be shared across variant commands, so that they can share cache for the shared imports
	cacheBaseDir := ""
	if len(cacheDirOpt) == 1 {
		cacheBaseDir = cacheDirOpt[0]
	} else if len(cacheDirOpt) > 0 {
		return "", fmt.Errorf("[bug] cacheDirOpt's length: want 0 or 1, got %d", len(cacheDirOpt))
	}

	query, _ := neturl.ParseQuery(u.RawQuery)

	should_cache := query.Get("cache") != "false"
	delete(query, "cache")

	// Precompute a redacted copy of the query for cache keys and debug logs.
	// Sensitive query parameters are hashed so that different credentials produce
	// distinct cache keys without writing the raw secret to disk or log output.
	// Note: go-getter may still log the full URL internally.
	var paramsSuffix string
	var redactedQuery neturl.Values
	if len(query) > 0 {
		redactedQuery = maps.Clone(query)
		for key := range redactedQuery {
			lk := strings.ToLower(key)
			if strings.Contains(lk, "token") || strings.Contains(lk, "password") ||
				strings.Contains(lk, "secret") || strings.Contains(lk, "key") ||
				strings.Contains(lk, "signature") {
				orig := redactedQuery[key]
				hashed := make([]string, len(orig))
				for i, v := range orig {
					h := sha256.Sum256([]byte(v))
					hashed[i] = hex.EncodeToString(h[:])[:8]
				}
				redactedQuery[key] = hashed
			}
		}
		paramsSuffix = strings.ReplaceAll(redactedQuery.Encode(), "&", "_")
	}

	var cacheKey string
	replacer := strings.NewReplacer(":", "", "//", "_", "/", "_", ".", "_")
	dirKey := replacer.Replace(srcDir)
	if paramsSuffix != "" {
		cacheKey = fmt.Sprintf("%s.%s", dirKey, paramsSuffix)
	} else {
		cacheKey = dirKey
	}

	cached := false

	// e.g. https_github_com_cloudposse_helmfiles_git.ref=0.xx.0
	getterDst := filepath.Join(cacheBaseDir, cacheKey)

	// e.g. os.CacheDir()/helmfile/https_github_com_cloudposse_helmfiles_git.ref=0.xx.0
	cacheDirPath := filepath.Join(r.Home, getterDst)
	if u.Getter == "normal" {
		srcDir = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
		normalDirKey := replacer.Replace(srcDir)
		if paramsSuffix != "" {
			cacheKey = fmt.Sprintf("%s.%s", normalDirKey, paramsSuffix)
		} else {
			cacheKey = normalDirKey
		}
		getterDst = filepath.Join(cacheBaseDir, cacheKey)
		cacheDirPath = filepath.Join(r.Home, cacheKey, u.Dir)
	}

	r.Logger.Debugf("remote> home: %s", r.Home)
	r.Logger.Debugf("remote> getter dest: %s", getterDst)
	r.Logger.Debugf("remote> cached dir: %s", cacheDirPath)

	{
		if r.fs.FileExistsAt(cacheDirPath) {
			return "", fmt.Errorf("%s is not a directory. Please remove it so that helmfile can use it for dependency caching", cacheDirPath)
		}

		if u.Getter == "normal" {
			cachedFilePath := filepath.Join(cacheDirPath, file)
			ok, err := r.fs.FileExists(cachedFilePath)
			if err == nil && ok {
				cached = true
			}
		} else if r.fs.DirectoryExistsAt(cacheDirPath) {
			cached = true
		}
	}

	if !cached || !should_cache {
		var getterSrc string
		if u.User != "" {
			getterSrc = fmt.Sprintf("%s://%s@%s%s", u.Scheme, u.User, u.Host, u.Dir)
		} else {
			getterSrc = fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, u.Dir)
		}

		if len(query) > 0 {
			getterSrc = strings.Join([]string{getterSrc, query.Encode()}, "?")
		}

		logSrc := path
		if redactedQuery != nil {
			logSrc = strings.Join([]string{strings.SplitN(path, "?", 2)[0], redactedQuery.Encode()}, "?")
		}
		r.Logger.Debugf("remote> downloading %s to %s", logSrc, cacheDirPath)

		switch {
		case u.Getter == "normal" && u.Scheme == "s3":
			if err := r.S3Getter.Get(r.Home, path, cacheDirPath); err != nil {
				rmerr := os.RemoveAll(cacheDirPath)
				if rmerr != nil {
					return "", errors.Join(err, rmerr)
				}
				return "", err
			}
		case u.Getter == "s3":
			// go-getter forced-getter syntax (e.g. "s3::https://bucket.s3.region.amazonaws.com/key").
			// go-getter v2 no longer ships an S3 getter, so route these to the
			// built-in AWS SDK v2 S3Getter which understands vhost/path-style URLs.
			// helmfile's Parse splits a "@<file>" selector (if any) into u.File;
			// strip it from the source so the S3 object key is derived from the
			// URL path only, matching how the go-getter branch feeds u.Dir.
			s3Src := stripSubdirSelector(path)
			if err := r.S3Getter.Get(r.Home, s3Src, cacheDirPath); err != nil {
				rmerr := os.RemoveAll(cacheDirPath)
				if rmerr != nil {
					return "", errors.Join(err, rmerr)
				}
				return "", err
			}
		case u.Getter == "normal" && (u.Scheme == "https" || u.Scheme == "http"):
			err := r.HttpGetter.Get(r.Home, path, cacheDirPath)
			if err != nil {
				return "", err
			}
		default:
			if u.Getter != "" {
				getterSrc = u.Getter + "::" + getterSrc
			}

			if err := r.Getter.Get(r.Home, getterSrc, cacheDirPath); err != nil {
				rmerr := os.RemoveAll(cacheDirPath)
				if rmerr != nil {
					return "", errors.Join(err, rmerr)
				}
				return "", err
			}
		}
	}
	return filepath.Join(cacheDirPath, file), nil
}

type Getter interface {
	Get(wd, src, dst string) error
}

type GoGetter struct {
	Logger *zap.SugaredLogger
}

type S3Getter struct {
	Logger *zap.SugaredLogger
}

type HttpGetter struct {
	Logger *zap.SugaredLogger
}

func (g *GoGetter) Get(wd, src, dst string) error {
	ctx := context.Background()

	req := &getter.Request{
		Src:     src,
		Dst:     dst,
		Pwd:     wd,
		GetMode: getter.ModeDir,
	}

	g.Logger.Debugf("request: %+v", *req)

	client := &getter.Client{}
	if _, err := client.Get(ctx, req); err != nil {
		return fmt.Errorf("get: %v", err)
	}

	return nil
}

func (g *S3Getter) Get(wd, src, dst string) error {
	region, bucket, key, err := ParseS3Url(src)
	if err != nil {
		return err
	}
	file := path.Base(key)
	targetFilePath := filepath.Join(dst, file)

	// If the region could not be derived from the URL, S3FileExists resolves it
	// via GetBucketLocation.
	resolvedRegion, err := g.S3FileExists(src, region)
	if err != nil {
		return err
	}
	if resolvedRegion != "" {
		region = resolvedRegion
	}

	err = os.MkdirAll(dst, os.FileMode(0700))
	if err != nil {
		return err
	}

	// Create a new AWS config and S3 client using AWS SDK v2
	// Suppress AWS SDK debug logging by default to prevent sensitive information from being logged
	// Can be configured via HELMFILE_AWS_SDK_LOG_LEVEL environment variable
	// See issue #2270
	configOpts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}
	// Only add log suppression if set to "off" (default)
	// For other values (minimal, standard, verbose), AWS SDK will respect AWS_SDK_GO_LOG_LEVEL env var
	if awsSDKLogLevel == "off" {
		// ClientLogMode(0) disables all AWS SDK logging (no LogRequest, LogResponse, etc.)
		configOpts = append(configOpts, config.WithClientLogMode(0))
	}
	cfg, err := config.LoadDefaultConfig(context.TODO(), configOpts...)
	if err != nil {
		return err
	}

	// Create an S3 client using the config
	s3Client := s3.NewFromConfig(cfg)

	getObjectInput := &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	resp, err := s3Client.GetObject(context.TODO(), getObjectInput)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			g.Logger.Errorf("Error closing connection to remote data source \n%v", err)
		}
	}()

	// go-getter v2 no longer ships an S3 getter, but it still ships archive
	// decompressors. To preserve go-getter v1's automatic archive extraction
	// (used with the "@<file>" selector to reference a file inside a tarball),
	// download archives to a temp file and decompress them into dst.
	decompressor := decompressorForFile(file)
	downloadPath := targetFilePath
	if decompressor != nil {
		tmp, terr := os.CreateTemp(dst, ".s3-archive-*")
		if terr != nil {
			return terr
		}
		if cerr := tmp.Close(); cerr != nil {
			return cerr
		}
		downloadPath = tmp.Name()
		defer func() { _ = os.Remove(downloadPath) }()
	}

	localFile, err := os.Create(downloadPath)
	if err != nil {
		return err
	}
	defer func(localFile *os.File) {
		err := localFile.Close()
		if err != nil {
			g.Logger.Errorf("Error writing file \n%v", err)
		}
	}(localFile)

	_, err = localFile.ReadFrom(resp.Body)
	if err != nil {
		return err
	}

	if decompressor != nil {
		if err := decompressor.Decompress(dst, downloadPath, true, os.FileMode(0)); err != nil {
			return fmt.Errorf("decompress %s: %w", file, err)
		}
	}

	return nil
}

// decompressorForFile returns the go-getter decompressor matching the file's
// archive extension (e.g. ".tar.gz", ".zip"), or nil if the file is not a
// recognized archive. The detection mirrors go-getter's own suffix matching.
func decompressorForFile(file string) getter.Decompressor {
	matchLen := 0
	var match string
	for k := range getter.Decompressors {
		if strings.HasSuffix(file, "."+k) && len(k) > matchLen {
			match = k
			matchLen = len(k)
		}
	}
	if match == "" {
		return nil
	}
	return getter.Decompressors[match]
}

func (g *HttpGetter) Get(wd, src, dst string) error {
	u, err := url.Parse(src)
	if err != nil {
		return err
	}
	file := path.Base(u.Path)
	targetFilePath := filepath.Join(dst, file)

	err = HttpFileExists(src)
	if err != nil {
		return err
	}

	err = os.MkdirAll(dst, os.FileMode(0700))
	if err != nil {
		return err
	}

	resp, err := http.Get(src)

	if err != nil {
		fmt.Printf("Error %v", err)
		return err
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			g.Logger.Errorf("Error closing connection to remote data source\n%v", err)
		}
	}()

	localFile, err := os.Create(targetFilePath)
	if err != nil {
		return err
	}
	defer func(localFile *os.File) {
		err := localFile.Close()
		if err != nil {
			g.Logger.Errorf("Error writing file \n%v", err)
		}
	}(localFile)

	_, err = localFile.ReadFrom(resp.Body)

	return err
}

func (g *S3Getter) S3FileExists(path, regionHint string) (string, error) {
	g.Logger.Debugf("Parsing S3 URL %s", path)
	_, bucket, key, err := ParseS3Url(path)
	if err != nil {
		return "", err
	}

	bucketRegion := regionHint
	if bucketRegion == "" {
		// Region
		g.Logger.Debugf("Creating config for determining S3 region %s", path)
		// Suppress AWS SDK debug logging by default to prevent sensitive information from being logged
		// Can be configured via HELMFILE_AWS_SDK_LOG_LEVEL environment variable
		// See issue #2270
		var configOpts []func(*config.LoadOptions) error
		if awsSDKLogLevel == "off" {
			// ClientLogMode(0) disables all AWS SDK logging (no LogRequest, LogResponse, etc.)
			configOpts = append(configOpts, config.WithClientLogMode(0))
		}
		cfg, err := config.LoadDefaultConfig(context.TODO(), configOpts...)
		if err != nil {
			return "", err
		}

		g.Logger.Debugf("Getting bucket %s location %s", bucket, path)
		s3Client := s3.NewFromConfig(cfg)
		bucketRegion = "us-east-1"
		getBucketLocationInput := &s3.GetBucketLocationInput{
			Bucket: &bucket,
		}
		resp, err := s3Client.GetBucketLocation(context.TODO(), getBucketLocationInput)
		if err != nil {
			return "", fmt.Errorf("failed to retrieve bucket location: %v", err)
		}
		if resp == nil || string(resp.LocationConstraint) == "" {
			g.Logger.Debugf("Bucket has no location Assuming us-east-1")
		} else {
			bucketRegion = string(resp.LocationConstraint)
		}
		g.Logger.Debugf("Got bucket location %s", bucketRegion)
	}

	// File existence
	g.Logger.Debugf("Creating new config with region to see if file exists")
	// Suppress AWS SDK debug logging by default to prevent sensitive information from being logged
	// Can be configured via HELMFILE_AWS_SDK_LOG_LEVEL environment variable
	// See issue #2270
	regionConfigOpts := []func(*config.LoadOptions) error{
		config.WithRegion(bucketRegion),
	}
	if awsSDKLogLevel == "off" {
		// ClientLogMode(0) disables all AWS SDK logging (no LogRequest, LogResponse, etc.)
		regionConfigOpts = append(regionConfigOpts, config.WithClientLogMode(0))
	}
	regionCfg, err := config.LoadDefaultConfig(context.TODO(), regionConfigOpts...)
	if err != nil {
		g.Logger.Error(err)
		return bucketRegion, err
	}
	g.Logger.Debugf("Creating new s3 client to check if object exists")
	s3Client := s3.NewFromConfig(regionCfg)
	headObjectInput := &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	g.Logger.Debugf("Fethcing head %s", path)
	_, err = s3Client.HeadObject(context.TODO(), headObjectInput)
	return bucketRegion, err
}

func HttpFileExists(path string) error {
	head, err := http.Head(path)
	statusOK := head.StatusCode >= 200 && head.StatusCode < 300
	if !statusOK {
		return fmt.Errorf("%s is not exists: head request get http code %d", path, head.StatusCode)
	}
	defer func() {
		_ = head.Body.Close()
	}()
	return err
}

// stripSubdirSelector removes the helmfile "@<file>" selector from a remote
// source URL, preserving any query string.
//
// helmfile's Parse splits the URL path on "@" into the download source (before
// "@") and the file selector (after "@") — see the comment in Parse. The
// selector lives in the path component only, so a "@" inside the query string
// is left intact. When the path contains no "@" (single-file downloads) the URL
// is returned unchanged.
func stripSubdirSelector(src string) string {
	pathPart := src
	queryPart := ""
	if q := strings.Index(src, "?"); q >= 0 {
		pathPart = src[:q]
		queryPart = src[q:]
	}
	// Mirror Parse: only treat a single "@" as the dir/file separator.
	if parts := strings.Split(pathPart, "@"); len(parts) == 2 {
		pathPart = parts[0]
	}
	return pathPart + queryPart
}

// ParseS3Url parses an S3 URL and returns the region, bucket, and object key.
//
// Supported URL formats:
//   - s3://<bucket>/<key>                    (region resolved dynamically via GetBucketLocation)
//   - s3::https://s3.amazonaws.com/<bucket>/<key>
//   - s3::https://s3-<region>.amazonaws.com/<bucket>/<key>
//   - s3::https://<bucket>.s3.<region>.amazonaws.com/<key>
//   - s3::https://<bucket>.s3-<region>.amazonaws.com/<key>
//   - s3::http://...                         (same amazonaws.com forms over plain HTTP)
//
// The "s3::" forced-getter prefix (the go-getter syntax for selecting the S3
// getter with an HTTPS URL) is optional and stripped before parsing.
func ParseS3Url(s3URL string) (region, bucket, key string, err error) {
	raw := s3URL
	// Strip any forced-getter prefix like "s3::" so that vhost-style URLs such
	// as "s3::https://bucket.s3.region.amazonaws.com/key" parse correctly.
	if idx := strings.Index(raw, "::"); idx >= 0 {
		raw = raw[idx+2:]
	}

	parsedURL, err := url.Parse(raw)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse S3 URL: %w", err)
	}

	switch parsedURL.Scheme {
	case "s3":
		// Path-style: s3://<bucket>/<key>
		bucket = parsedURL.Host
		key = strings.TrimPrefix(parsedURL.Path, "/")
		// Region is unknown for the bare s3:// form; it is resolved later
		// via GetBucketLocation.
		return "", bucket, key, nil
	case "http", "https":
		// Continue to amazonaws.com vhost/path-style parsing below.
	default:
		return "", "", "", fmt.Errorf("invalid URL scheme (expected 's3', 'http', or 'https'): %s", s3URL)
	}

	// Amazon S3 supports both virtual-hosted-style and path-style URLs.
	// See https://docs.aws.amazon.com/AmazonS3/latest/userguide/access-bucket-intro.html
	if !strings.Contains(parsedURL.Host, "amazonaws.com") {
		return "", "", "", fmt.Errorf("URL is not a valid S3 URL (host must be amazonaws.com): %s", s3URL)
	}

	hostParts := strings.Split(parsedURL.Host, ".")
	switch len(hostParts) {
	case 3:
		// Path-style: s3.amazonaws.com/<bucket>/<key> or s3-<region>.amazonaws.com/<bucket>/<key>
		region = strings.TrimPrefix(strings.TrimPrefix(hostParts[0], "s3-"), "s3")
		if region == "" {
			region = "us-east-1"
		}
		pathParts := strings.SplitN(parsedURL.Path, "/", 3)
		if len(pathParts) < 3 {
			return "", "", "", fmt.Errorf("URL is not a valid S3 URL: %s", s3URL)
		}
		bucket = pathParts[1]
		key = pathParts[2]
	case 4:
		// Virtual-hosted-style, dash region: <bucket>.s3-<region>.amazonaws.com/<key>
		region = strings.TrimPrefix(strings.TrimPrefix(hostParts[1], "s3-"), "s3")
		if region == "" {
			return "", "", "", fmt.Errorf("URL is not a valid S3 URL: %s", s3URL)
		}
		pathParts := strings.SplitN(parsedURL.Path, "/", 2)
		if len(pathParts) < 2 {
			return "", "", "", fmt.Errorf("URL is not a valid S3 URL: %s", s3URL)
		}
		bucket = hostParts[0]
		key = pathParts[1]
	case 5:
		// Virtual-hosted-style, dot region: <bucket>.s3.<region>.amazonaws.com/<key>
		region = hostParts[2]
		if region == "" {
			return "", "", "", fmt.Errorf("URL is not a valid S3 URL: %s", s3URL)
		}
		pathParts := strings.SplitN(parsedURL.Path, "/", 2)
		if len(pathParts) < 2 {
			return "", "", "", fmt.Errorf("URL is not a valid S3 URL: %s", s3URL)
		}
		bucket = hostParts[0]
		key = pathParts[1]
	default:
		return "", "", "", fmt.Errorf("URL is not a valid S3 URL: %s", s3URL)
	}

	return region, bucket, key, nil
}

func NewRemote(logger *zap.SugaredLogger, homeDir string, fs *filesystem.FileSystem) *Remote {
	remote := &Remote{
		Logger:     logger,
		Home:       homeDir,
		Getter:     &GoGetter{Logger: logger},
		S3Getter:   &S3Getter{Logger: logger},
		HttpGetter: &HttpGetter{Logger: logger},
		fs:         fs,
	}

	if remote.Home == "" {
		// Use for remote charts
		remote.Home = CacheDir()
	}

	return remote
}
