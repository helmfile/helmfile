package state

import (
	"fmt"
	"github.com/helmfile/helmfile/pkg/remote"
	"go.uber.org/zap"
	"net/url"
	"path/filepath"
	"sort"
)

type Storage struct {
	logger *zap.SugaredLogger

	FilePath string

	readFile func(string) ([]byte, error)
	basePath string
	glob     func(string) ([]string, error)
}

func NewStorage(forFile string, logger *zap.SugaredLogger, glob func(string) ([]string, error)) *Storage {
	return &Storage{
		FilePath: forFile,
		basePath: filepath.Dir(forFile),
		logger:   logger,
		glob:     glob,
	}
}

func (st *Storage) resolveFile(missingFileHandler *string, tpe, path string) ([]string, bool, error) {
	title := fmt.Sprintf("%s file", tpe)

	var files []string
	var err error
	if remote.IsRemote(path) {
		r := remote.NewRemote(st.logger, "", st.readFile, directoryExistsAt, fileExistsAt)

		fetchedDir, _ := r.Fetch(path, "values")
		files = []string{fetchedDir}
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
	matches, err := st.glob(absPathPattern)
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
		return st.JoinBase(path)
	}
}

// JoinBase returns an absolute path in the form basePath/relative
func (st *Storage) JoinBase(relPath string) string {
	return filepath.Join(st.basePath, relPath)
}
