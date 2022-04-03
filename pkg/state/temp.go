package state

import (
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"

	"github.com/davecgh/go-spew/spew"
)

func createTempValuesFile(release *ReleaseSpec, data interface{}) (*os.File, error) {
	p, err := tempValuesFilePath(release, data)
	if err != nil {
		return nil, err
	}

	f, err := os.Create(*p)
	if err != nil {
		return nil, err
	}

	return f, nil
}

func tempValuesFilePath(release *ReleaseSpec, data interface{}) (*string, error) {
	id, err := generateValuesID(release, data)
	if err != nil {
		panic(err)
	}

	workDir := os.Getenv("HELMFILE_TEMPDIR")
	if workDir == "" {
		workDir, err = os.MkdirTemp(os.TempDir(), "helmfile")
		if err != nil {
			panic(err)
		}
	}

	d := filepath.Join(workDir, id)

	_, err = os.Stat(d)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		panic(err)
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
	printer.Fprintf(hash, "%#v", obj)

	sum := fmt.Sprint(hash.Sum32())

	return SafeEncodeString(sum), nil
}
