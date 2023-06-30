package state

import (
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/remote"
)

type Storage struct {
	logger *zap.SugaredLogger

	FilePath string

	basePath string
	fs       *filesystem.FileSystem
}

func NewStorage(forFile string, logger *zap.SugaredLogger, fs *filesystem.FileSystem) *Storage {
	return &Storage{
		FilePath: forFile,
		basePath: filepath.Dir(forFile),
		logger:   logger,
		fs:       fs,
	}
}

type resolveFileConfig struct {
	IgnoreMissingGitBranch bool
}

type resolveFileOption func(*resolveFileConfig)

func ignoreMissingGitBranch(v bool) func(c *resolveFileConfig) {
	return func(c *resolveFileConfig) {
		c.IgnoreMissingGitBranch = v
	}
}

func (st *Storage) resolveFile(missingFileHandler *string, tpe, path string, opts ...resolveFileOption) ([]string, bool, error) {
	title := fmt.Sprintf("%s file", tpe)

	var (
		files []string
		err   error
		conf  resolveFileConfig
	)

	for _, o := range opts {
		o(&conf)
	}

	if remote.IsRemote(path) {
		r := remote.NewRemote(st.logger, "", st.fs)

		fetchedFilePath, err := r.Fetch(path, "values")
		if err != nil {
			// https://github.com/helmfile/helmfile/issues/392
			if conf.IgnoreMissingGitBranch && strings.Contains(err.Error(), "' did not match any file(s) known to git") {
				st.logger.Debugf("Ignored missing git branch error: %v", err)
			} else {
				return nil, false, err
			}
		}

		if st.fs.FileExistsAt(fetchedFilePath) {
			files = []string{fetchedFilePath}
		}
	} else {
		files, err = st.ExpandPaths(path)
	}

	if err != nil {
		return nil, false, err
	}

	var handlerId string

	if missingFileHandler != nil {
		handlerId = *missingFileHandler
	} else {
		handlerId = MissingFileHandlerError
	}

	if len(files) == 0 {
		switch handlerId {
		case MissingFileHandlerError:
			return nil, false, fmt.Errorf("%s matching \"%s\" does not exist in \"%s\"", title, path, st.basePath)
		case MissingFileHandlerWarn:
			st.logger.Warnf("skipping missing %s matching \"%s\"", title, path)
			return nil, true, nil
		case MissingFileHandlerInfo:
			st.logger.Infof("skipping missing %s matching \"%s\"", title, path)
			return nil, true, nil
		case MissingFileHandlerDebug:
			st.logger.Debugf("skipping missing %s matching \"%s\"", title, path)
			return nil, true, nil
		default:
			available := []string{
				MissingFileHandlerError,
				MissingFileHandlerWarn,
				MissingFileHandlerInfo,
				MissingFileHandlerDebug,
			}
			return nil, false, fmt.Errorf("invalid missing file handler \"%s\" while processing \"%s\" in \"%s\": it must be one of %s", handlerId, path, st.FilePath, available)
		}
	}

	return files, false, nil
}

func (st *Storage) ExpandPaths(globPattern string) ([]string, error) {
	result := []string{}
	absPathPattern := st.normalizePath(globPattern)
	matches, err := st.fs.Glob(absPathPattern)
	if err != nil {
		return nil, fmt.Errorf("failed processing %s: %v", globPattern, err)
	}

	sort.Strings(matches)

	result = append(result, matches...)
	return result, nil
}

// normalizes relative path to absolute one
func (st *Storage) normalizePath(path string) string {
	u, _ := url.Parse(path)
	if u != nil && (u.Scheme != "" || filepath.IsAbs(path)) {
		return path
	} else {
		return st.JoinBase(path, runtime.GOOS)
	}
}

// JoinBase returns an absolute path in the form basePath/relative
// Helm's setFiles command does not support unescaped filepath separators (\) on Windows.
// Instead, it requires double backslashes (\\) as filepath separators.
// See https://github.com/helm/helm/issues/9537
func (st *Storage) JoinBase(relPath, GOOS string) string {
	path := filepath.Join(st.basePath, relPath)
	if GOOS == "windows" {
		return strings.ReplaceAll(path, "\\", "\\\\")
	}
	return path
}
