package remote

import (
	"context"
	"encoding/json"
	"fmt"
	neturl "net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hashicorp/go-getter"
	"github.com/hashicorp/go-getter/helper/url"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/helmfile/helmfile/pkg/envvar"
	"github.com/helmfile/helmfile/pkg/filesystem"
)

var disableInsecureFeatures bool

func init() {
	disableInsecureFeatures, _ = strconv.ParseBool(os.Getenv(envvar.DisableInsecureFeatures))
}

func CacheDir() string {
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

	// Filesystem abstraction
	// Inject any implementation of your choice, like an im-memory impl for testing, os.ReadFile for the real-world use.
	fs *filesystem.FileSystem
}

func (r *Remote) Unmarshal(src string, dst interface{}) error {
	bytes, err := r.GetBytes(src)
	if err != nil {
		return err
	}

	strs := strings.Split(src, "/")
	file := strs[len(strs)-1]
	ext := filepath.Ext(file)

	{
		r.Logger.Debugf("unmarshalling %s", string(bytes))

		var err error
		switch ext {
		case "json":
			err = json.Unmarshal(bytes, dst)
		default:
			err = yaml.Unmarshal(bytes, dst)
		}

		r.Logger.Debugf("unmarshalled to %v", dst)

		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Remote) GetBytes(goGetterSrc string) ([]byte, error) {
	f, err := r.Fetch(goGetterSrc)
	if err != nil {
		return nil, err
	}

	bytes, err := r.fs.ReadFile(f)
	if err != nil {
		return nil, fmt.Errorf("read file: %v", err)
	}

	return bytes, nil
}

// Locate takes an URL to a remote file or a path to a local file.
// If the argument was an URL, it fetches the remote directory contained within the URL,
// and returns the path to the file in the fetched directory
func (r *Remote) Locate(urlOrPath string) (string, error) {
	if r.fs.FileExistsAt(urlOrPath) || r.fs.DirectoryExistsAt(urlOrPath) {
		return urlOrPath, nil
	}
	fetched, err := r.Fetch(urlOrPath)
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
		return nil, fmt.Errorf("invalid src format: it must be `[<getter>::]<scheme>://<host>/<path/to/dir>@<path/to/file>?key1=val1&key2=val2: got %s", goGetterSrc)
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

func (r *Remote) Fetch(goGetterSrc string, cacheDirOpt ...string) (string, error) {
	u, err := Parse(goGetterSrc)
	if err != nil {
		return "", err
	}

	srcDir := fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, u.Dir)
	file := u.File

	r.Logger.Debugf("getter: %s", u.Getter)
	r.Logger.Debugf("scheme: %s", u.Scheme)
	r.Logger.Debugf("user: %s", u.User)
	r.Logger.Debugf("host: %s", u.Host)
	r.Logger.Debugf("dir: %s", u.Dir)
	r.Logger.Debugf("file: %s", u.File)

	// This should be shared across variant commands, so that they can share cache for the shared imports
	cacheBaseDir := ""
	if len(cacheDirOpt) == 1 {
		cacheBaseDir = cacheDirOpt[0]
	} else if len(cacheDirOpt) > 0 {
		return "", fmt.Errorf("[bug] cacheDirOpt's length: want 0 or 1, got %d", len(cacheDirOpt))
	}

	query := u.RawQuery

	var cacheKey string
	replacer := strings.NewReplacer(":", "", "//", "_", "/", "_", ".", "_")
	dirKey := replacer.Replace(srcDir)
	if len(query) > 0 {
		q, _ := neturl.ParseQuery(query)
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

	r.Logger.Debugf("home: %s", r.Home)
	r.Logger.Debugf("getter dest: %s", getterDst)
	r.Logger.Debugf("cached dir: %s", cacheDirPath)

	{
		if r.fs.FileExistsAt(cacheDirPath) {
			return "", fmt.Errorf("%s is not directory. please remove it so that variant could use it for dependency caching", getterDst)
		}

		if r.fs.DirectoryExistsAt(cacheDirPath) {
			cached = true
		}
	}

	if !cached {
		var getterSrc string
		if u.User != "" {
			getterSrc = fmt.Sprintf("%s://%s@%s%s", u.Scheme, u.User, u.Host, u.Dir)
		} else {
			getterSrc = fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, u.Dir)
		}

		if len(query) > 0 {
			getterSrc = strings.Join([]string{getterSrc, query}, "?")
		}

		if u.Getter != "" {
			getterSrc = u.Getter + "::" + getterSrc
		}

		r.Logger.Debugf("downloading %s to %s", getterSrc, getterDst)

		if err := r.Getter.Get(r.Home, getterSrc, cacheDirPath); err != nil {
			rmerr := os.RemoveAll(cacheDirPath)
			if rmerr != nil {
				return "", multierr.Append(err, rmerr)
			}
			return "", err
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

func NewRemote(logger *zap.SugaredLogger, homeDir string, fs *filesystem.FileSystem) *Remote {
	if disableInsecureFeatures {
		panic("Remote sources are disabled due to 'DISABLE_INSECURE_FEATURES'")
	}
	remote := &Remote{
		Logger: logger,
		Home:   homeDir,
		Getter: &GoGetter{Logger: logger},
		fs:     fs,
	}

	if remote.Home == "" {
		// Use for remote charts
		remote.Home = CacheDir()
	}

	return remote
}
