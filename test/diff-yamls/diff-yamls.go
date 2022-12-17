package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-test/deep"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type diffSource string

const (
	diffSourceLeft  diffSource = "left"
	diffSourceRight diffSource = "right"
)

var (
	rootCmd = &cobra.Command{
		Use:   "diff-yamls dir-with-yamls/ dir-with-yamls/",
		Short: "Print any diff between the given directories",
		Long: `Similar to the 'diff' command, but file contents
are compared after being parsed as a set of Kubernetes manifests.`,
		Args: cobra.ExactArgs(2),
		Run:  run,
	}
)

func run(cmd *cobra.Command, args []string) {
	left := args[0]
	right := args[1]
	leftYamls, err := globYamlFilenames(left)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	rightYamls, err := globYamlFilenames(right)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	exitCode := 0

	onlyInLeft := leftYamls.Difference(rightYamls)
	if onlyInLeft.Len() > 0 {
		exitCode = 1
		for _, f := range sets.List(onlyInLeft) {
			fmt.Fprintf(os.Stderr, "Only in %s: %s\n", left, f)
		}
	}

	onlyInRight := rightYamls.Difference(leftYamls)
	if onlyInRight.Len() > 0 {
		exitCode = 1
		for _, f := range sets.List(onlyInRight) {
			fmt.Fprintf(os.Stderr, "Only in %s: %s\n", right, f)
		}
	}

	inBoth := leftYamls.Intersection(rightYamls)
	for _, f := range sets.List(inBoth) {
		leftPath := filepath.Join(left, f)
		leftNodes, err := readManifest(leftPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		rightPath := filepath.Join(right, f)
		rightNodes, err := readManifest(rightPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		ps := pairs{}
		for _, node := range leftNodes {
			if err := ps.add(node, diffSourceLeft); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		}
		for _, node := range rightNodes {
			if err := ps.add(node, diffSourceRight); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		}
		for _, p := range ps.list {
			switch {
			case p.left != nil && p.right == nil:
				fmt.Fprintf(os.Stderr, "Only in %s: %s\n", leftPath, p.left.getID())
				exitCode = 1
			case p.left == nil && p.right != nil:
				fmt.Fprintf(os.Stderr, "Only in %s: %s\n", rightPath, p.right.getID())
				exitCode = 1
			default:
				diff := deep.Equal(p.left, p.right)
				if diff != nil {
					id := p.left.getID()
					fmt.Fprintf(os.Stderr, "< %s %s\n", id, leftPath)
					fmt.Fprintf(os.Stderr, "> %s %s\n", id, rightPath)
					for _, d := range diff {
						fmt.Fprintf(os.Stderr, "%s\n", d)
					}
					exitCode = 1
				}
			}
		}
	}

	os.Exit(exitCode)
}

func globYamlFilenames(dir string) (sets.Set[string], error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	set := sets.Set[string]{}
	for _, f := range matches {
		set.Insert(filepath.Base(f))
	}
	return set, nil
}

type resource map[string]interface{}

type meta struct {
	apiVersion string
	kind       string
	name       string
	namespace  string
}

func (res resource) getMeta() meta {
	if len(res) == 0 {
		return meta{}
	}
	m := meta{}
	apiVersion, _ := res["apiVersion"].(string)
	m.apiVersion = apiVersion
	kind, _ := res["kind"].(string)
	m.kind = kind
	metadata, _ := res["metadata"].(map[string]interface{})
	name, _ := metadata["name"].(string)
	m.name = name
	namespace, _ := metadata["namespace"].(string)
	m.namespace = namespace
	return m
}

func readManifest(path string) ([]resource, error) {
	var err error
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()
	decoder := yaml.NewYAMLToJSONDecoder(f)
	resources := []resource{}
	for {
		r := make(resource)
		err = decoder.Decode(&r)
		if err != nil {
			break
		}
		if len(r) > 0 {
			resources = append(resources, r)
		}
	}
	if err != nil && err != io.EOF {
		return nil, err
	}
	return resources, nil
}

func (res resource) getID() string {
	meta := res.getMeta()

	ns := meta.namespace
	if ns == "" {
		ns = "~X"
	}
	nm := meta.name
	if nm == "" {
		nm = "~N"
	}
	gv := meta.apiVersion
	if gv == "" {
		gv = "~G_~V"
	}
	k := meta.kind
	if k == "" {
		k = "~K"
	}
	gvk := strings.Join([]string{gv, k}, "_")
	return strings.Join([]string{gvk, ns, nm}, "|")
}

// lifted from kustomize/kyaml/kio/filters/merge3.go
type pairs struct {
	list []*pair
}

func (ps *pairs) isSameResource(meta1, meta2 meta) bool {
	if meta1.name != meta2.name {
		return false
	}
	if meta1.namespace != meta2.namespace {
		return false
	}
	if meta1.apiVersion != meta2.apiVersion {
		return false
	}
	if meta1.kind != meta2.kind {
		return false
	}
	return true
}

func (ps *pairs) add(node resource, source diffSource) error {
	nodeMeta := node.getMeta()

	for i := range ps.list {
		p := ps.list[i]
		if ps.isSameResource(p.meta, nodeMeta) {
			return p.add(node, source)
		}
	}
	p := &pair{meta: nodeMeta}
	if err := p.add(node, source); err != nil {
		return err
	}
	ps.list = append(ps.list, p)
	return nil
}

type pair struct {
	meta  meta
	left  resource
	right resource
}

func (p *pair) add(node resource, source diffSource) error {
	switch source {
	case diffSourceLeft:
		if p.left != nil {
			return fmt.Errorf("left source already specified")
		}
		p.left = node
	case diffSourceRight:
		if p.right != nil {
			return fmt.Errorf("right source already specified")
		}
		p.right = node
	default:
		return fmt.Errorf("unknown diff source value: %s", source)
	}
	return nil
}

func main() {
	err := rootCmd.Execute()

	if err != nil {
		fmt.Println(fmt.Errorf("unexpected error: %v", err))
		os.Exit(1)
	}
}
