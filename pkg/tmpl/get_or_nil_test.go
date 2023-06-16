package tmpl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type EmptyStruct struct {
}

func TestComplexStruct(t *testing.T) {
	type CS struct {
		Bar map[string]string
	}
	obj := CS{
		Bar: map[string]string{
			"foo": "bar",
		},
	}

	existValue, err := get("Bar.foo", obj)
	require.NoErrorf(t, err, "unexpected error: %v", err)
	require.Equalf(t, existValue, "bar", "unexpected value for path Bar.foo in %v: expected=bar, actual=%v", obj, existValue)

	noExistValue, err := get("Bar.baz", obj)
	require.Errorf(t, err, "expected error but was not occurred")
	require.Nilf(t, noExistValue, "expected nil but was not occurred")

	noExistValueWithDefault, err := get("Bar.baz", "default", obj)
	require.NoErrorf(t, err, "unexpected error: %v", err)
	require.Equalf(t, noExistValueWithDefault, "default", "unexpected value for path Bar.baz in %v: expected=default, actual=%v", obj, noExistValueWithDefault)
}

func TestGetSimpleStruct(t *testing.T) {
	type Foo struct{ Bar string }

	obj := struct{ Foo }{Foo{Bar: "Bar"}}

	v1, err := get("Foo.Bar", obj)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if v1 != "Bar" {
		t.Errorf("unexpected value for path Foo.Bar in %v: expected=Bar, actual=%v", obj, v1)
	}

	_, err = get("Foo.baz", obj)

	if err == nil {
		t.Errorf("expected error but was not occurred")
	}

	_, err = get("foo", EmptyStruct{})

	if err == nil {
		t.Errorf("expected error but was not occurred")
	}
}

func TestGetMap(t *testing.T) {
	obj := map[string]any{"Foo": map[string]any{"Bar": "Bar"}}

	v1, err := get("Foo.Bar", obj)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if v1 != "Bar" {
		t.Errorf("unexpected value for path Foo.Bar in %v: expected=Bar, actual=%v", obj, v1)
	}

	_, err = get("Foo.baz", obj)

	if err == nil {
		t.Errorf("expected error but was not occurred")
	}
}

func TestGetMapPtr(t *testing.T) {
	obj := map[string]any{"Foo": map[string]any{"Bar": "Bar"}}
	objPrt := &obj

	v1, err := get("Foo.Bar", objPrt)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if v1 != "Bar" {
		t.Errorf("unexpected value for path Foo.Bar in %v: expected=Bar, actual=%v", objPrt, v1)
	}

	_, err = get("Foo.baz", objPrt)

	if err == nil {
		t.Errorf("expected error but was not occurred")
	}
}

func TestGet_Default(t *testing.T) {
	obj := map[string]any{"Foo": map[string]any{}, "foo": 1}

	v1, err := get("Foo.Bar", "Bar", obj)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if v1 != "Bar" {
		t.Errorf("unexpected value for path Foo.Bar in %v: expected=Bar, actual=%v", obj, v1)
	}

	v2, err := get("Baz", "Baz", obj)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if v2 != "Baz" {
		t.Errorf("unexpected value for path Baz in %v: expected=Baz, actual=%v", obj, v2)
	}

	_, err = get("foo.Bar", "fooBar", obj)

	if err == nil {
		t.Errorf("expected error but was not occurred")
	}
}

func TestGetOrNilStruct(t *testing.T) {
	type Foo struct{ Bar string }

	obj := struct{ Foo }{Foo{Bar: "Bar"}}

	v1, err := getOrNil("Foo.Bar", obj)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if v1 != "Bar" {
		t.Errorf("unexpected value for path Foo.Bar in %v: expected=Bar, actual=%v", obj, v1)
	}

	v2, err := getOrNil("Foo.baz", obj)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if v2 != nil {
		t.Errorf("unexpected value for path Foo.baz in %v: expected=nil, actual=%v", obj, v2)
	}
}

func TestGetOrNilMap(t *testing.T) {
	obj := map[string]any{"Foo": map[string]any{"Bar": "Bar"}}

	v1, err := getOrNil("Foo.Bar", obj)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if v1 != "Bar" {
		t.Errorf("unexpected value for path Foo.Bar in %v: expected=Bar, actual=%v", obj, v1)
	}

	v2, err := getOrNil("Foo.baz", obj)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if v2 != nil {
		t.Errorf("unexpected value for path Foo.baz in %v: expected=nil, actual=%v", obj, v2)
	}
}
