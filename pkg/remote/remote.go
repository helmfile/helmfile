package remote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	neturl "net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/hashicorp/go-getter"
	"github.com/hashicorp/go-getter/helper/url"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/envvar"
	"github.com/helmfile/helmfile/pkg/filesystem"
)

var (
	protocols               = []string{"s3", "http", "https"}
	disableInsecureFeatures bool
)

func init() {
	disableInsecureFeatures, _ = strconv.ParseBool(os.Getenv(envvar.DisableInsecureFeatures))
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

	var cacheKey string
	replacer := strings.NewReplacer(":", "", "//", "_", "/", "_", ".", "_")
	dirKey := replacer.Replace(srcDir)
	if len(query) > 0 {
		q := maps.Clone(query)
		if q.Has("sshkey") {
			q.Set("sshkey", "redacted")
		}
		paramsKey := strings.ReplaceAll(q.Encode(), "&", "_")
		cacheKey = fmt.Sprintf("%s.%s", dirKey, paramsKey)
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
		dirKey = replacer.Replace(srcDir)
		if len(query) > 0 {
			q := maps.Clone(query)
			if q.Has("sshkey") {
				q.Set("sshkey", "redacted")
			}
			paramsKey := strings.ReplaceAll(q.Encode(), "&", "_")
			cacheKey = fmt.Sprintf("%s.%s", dirKey, paramsKey)
		} else {
			cacheKey = dirKey
		}
		cacheDirPath = filepath.Join(r.Home, cacheKey, u.Dir)
	}

	r.Logger.Debugf("remote> home: %s", r.Home)
	r.Logger.Debugf("remote> getter dest: %s", getterDst)
	r.Logger.Debugf("remote> cached dir: %s", cacheDirPath)

	{
		if r.fs.FileExistsAt(cacheDirPath) {
			return "", fmt.Errorf("%s is not directory. please remove it so that variant could use it for dependency caching", getterDst)
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

		r.Logger.Debugf("remote> downloading %s to %s", getterSrc, getterDst)

		switch {
		case u.Getter == "normal" && u.Scheme == "s3":
			err := r.S3Getter.Get(r.Home, path, cacheDirPath)
			if err != nil {
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

	get := &getter.Client{
		Ctx:     ctx,
		Src:     src,
		Dst:     dst,
		Pwd:     wd,
		Mode:    getter.ClientModeDir,
		Options: []getter.ClientOption{},
	}

	g.Logger.Debugf("client: %+v", *get)

	if err := get.Get(); err != nil {
		return fmt.Errorf("get: %v", err)
	}

	return nil
}

func (g *S3Getter) Get(wd, src, dst string) error {
	u, err := url.Parse(src)
	if err != nil {
		return err
	}
	file := path.Base(u.Path)
	targetFilePath := filepath.Join(dst, file)

	region, err := g.S3FileExists(src)
	if err != nil {
		return err
	}

	bucket, key, err := ParseS3Url(src)
	if err != nil {
		return err
	}

	err = os.MkdirAll(dst, os.FileMode(0700))
	if err != nil {
		return err
	}

	// Create a new AWS session using the default AWS configuration
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config: aws.Config{
			Region: aws.String(region),
		},
	}))

	// Create an S3 client using the session
	s3Client := s3.New(sess)

	getObjectInput := &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	resp, err := s3Client.GetObject(getObjectInput)
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			g.Logger.Errorf("Error closing connection to remote data source \n%v", err)
		}
	}(resp.Body)

	if err != nil {
		return err
	}

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

func (g *S3Getter) S3FileExists(path string) (string, error) {
	g.Logger.Debugf("Parsing S3 URL %s", path)
	bucket, key, err := ParseS3Url(path)
	if err != nil {
		return "", err
	}

	// Region
	g.Logger.Debugf("Creating session for determining S3 region %s", path)
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	g.Logger.Debugf("Getting bucket %s location %s", bucket, path)
	s3Client := s3.New(sess)
	bucketRegion := "us-east-1"
	getBucketLocationInput := &s3.GetBucketLocationInput{
		Bucket: aws.String(bucket),
	}
	resp, err := s3Client.GetBucketLocation(getBucketLocationInput)
	if err != nil {
		return "", fmt.Errorf("Error: Failed to retrieve bucket location: %v\n", err)
	}
	if resp == nil || resp.LocationConstraint == nil {
		g.Logger.Debugf("Bucket has no location Assuming us-east-1")
	} else {
		bucketRegion = *resp.LocationConstraint
	}
	g.Logger.Debugf("Got bucket location %s", bucketRegion)

	// File existence
	g.Logger.Debugf("Creating new session with region to see if file exists")
	regionSession, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config: aws.Config{
			Region: aws.String(bucketRegion),
		},
	})
	if err != nil {
		g.Logger.Error(err)
	}
	g.Logger.Debugf("Creating new s3 client to check if object exists")
	s3Client = s3.New(regionSession)
	headObjectInput := &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	g.Logger.Debugf("Fethcing head %s", path)
	_, err = s3Client.HeadObject(headObjectInput)
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

func ParseS3Url(s3URL string) (string, string, error) {
	parsedURL, err := url.Parse(s3URL)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse S3 URL: %w", err)
	}

	if parsedURL.Scheme != "s3" {
		return "", "", fmt.Errorf("invalid URL scheme (expected 's3')")
	}

	bucket := parsedURL.Host
	key := strings.TrimPrefix(parsedURL.Path, "/")

	return bucket, key, nil
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
