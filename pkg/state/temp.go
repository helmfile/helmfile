package state

import (
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/helmfile/helmfile/pkg/envvar"
)

func createTempValuesFile(release *ReleaseSpec, workDir string, data interface{}) (*os.File, error) {
	p, err := tempValuesFilePath(release, workDir, data)
	if err != nil {
		return nil, err
	}

	f, err := os.Create(*p)
	if err != nil {
		return nil, err
	}

	return f, nil
}

func mkdirTemp() (string, error) {
	workDir := os.Getenv(envvar.TempDir)
	var err error
	if workDir == "" {
		workDir, err = os.MkdirTemp(os.TempDir(), "helmfile")
	} else {
		err = os.MkdirAll(workDir, os.FileMode(0700))
	}
	if err != nil {
		return "", err
	}
	return workDir, nil
}

func tempValuesFilePath(release *ReleaseSpec, workDir string, data interface{}) (*string, error) {
	id, err := generateValuesID(release, data)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	d := filepath.Join(workDir, id)

	_, err = os.Stat(d)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	return &d, nil
}

func generateValuesID(release *ReleaseSpec, data interface{}) (string, error) {
	var id []string

	if release.Namespace != "" {
		id = append(id, release.Namespace)
	}

	id = append(id, release.Name, "values")

	hash, err := HashObject([]interface{}{release, data})
	if err != nil {
		return "", err
	}

	id = append(id, hash)

	return strings.Join(id, "-"), nil
}

func HashObject(obj interface{}) (string, error) {
	hash := fnv.New32a()

	hash.Reset()

	printer := spew.ConfigState{
		Indent:         " ",
		SortKeys:       true,
		DisableMethods: true,
		SpewKeys:       true,
	}
	_, err := printer.Fprintf(hash, "%#v", obj)
	if err != nil {
		return "", err
	}

	sum := fmt.Sprint(hash.Sum32())

	return rand.SafeEncodeString(sum), nil
}
