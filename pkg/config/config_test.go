package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCLIConfigImplStateValuesSet(t *testing.T) {
	tests := []struct {
		name    string
		set     []string
		want    map[string]any
		wantErr string
	}{
		{
			name: "comma separated assignments retain their types",
			set:  []string{"count=2,enabled=true,disabled=false,empty=null,zero=0,code=001"},
			want: map[string]any{
				"count": int64(2), "enabled": true, "disabled": false,
				"empty": nil, "zero": int64(0), "code": "001",
			},
		},
		{
			name: "escaped commas within multiple values",
			set:  []string{`a=1,b=2\,3,c=9\,10\,11`},
			want: map[string]any{"a": int64(1), "b": "2,3", "c": "9,10,11"},
		},
		{
			name: "escaped commas at the start and end of a value",
			set:  []string{`message=\,hello\,`},
			want: map[string]any{"message": ",hello,"},
		},
		{
			name: "repeated flags overwrite earlier values",
			set:  []string{`message=first\,second`, `message=third\,fourth,count=2`},
			want: map[string]any{"message": "third,fourth", "count": int64(2)},
		},
		{
			name: "equals signs remain within values",
			set:  []string{`message=first\,second=value,count=2`},
			want: map[string]any{"message": "first,second=value", "count": int64(2)},
		},
		{
			name: "unicode values",
			set:  []string{`message=你好\,世界`},
			want: map[string]any{"message": "你好,世界"},
		},
		{
			name: "empty value",
			set:  []string{"message=,count=2"},
			want: map[string]any{"message": "", "count": int64(2)},
		},
		{
			name: "backslashes before other characters are preserved",
			set:  []string{`path=C:\tmp\config`},
			want: map[string]any{"path": `C:\tmp\config`},
		},
		{
			name: "trailing backslash is preserved",
			set:  []string{`path=C:\tmp\`},
			want: map[string]any{"path": `C:\tmp\`},
		},
		{
			name: "escaped dots in keys are preserved for key parsing",
			set:  []string{`annotations.example\.com/key=first\,second`},
			want: map[string]any{
				"annotations": map[string]any{"example.com/key": "first,second"},
			},
		},
		{
			name: "nested array keys",
			set:  []string{`servers[0].host=east\,west,enabled=true`},
			want: map[string]any{
				"servers": []any{map[string]any{"host": "east,west"}},
				"enabled": true,
			},
		},
		{
			name: "even backslashes leave comma as an assignment separator",
			set:  []string{`path=C:\\,count=2`},
			want: map[string]any{"path": `C:\\`, "count": int64(2)},
		},
		{
			name: "odd backslashes escape the comma",
			set:  []string{`message=first\\\,second`},
			want: map[string]any{"message": `first\\,second`},
		},
		{
			name:    "assignment without equals sign is rejected",
			set:     []string{"message"},
			wantErr: `invalid assignment "message": expected <key>=<value>`,
		},
		{
			name:    "trailing comma is rejected",
			set:     []string{"message=hello,"},
			wantErr: `invalid assignment "": expected <key>=<value>`,
		},
		{
			name:    "empty key is rejected",
			set:     []string{"=hello"},
			wantErr: `invalid assignment "=hello": expected <key>=<value>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGlobalImpl(&GlobalOptions{StateValuesSet: tt.set})
			err := NewCLIConfigImpl(g)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, g.StateValuesSet())
		})
	}
}

func TestNewCLIConfigImplStateValuesSetString(t *testing.T) {
	tests := []struct {
		name    string
		set     []string
		want    map[string]any
		wantErr string
	}{
		{
			name: "values are kept as strings without type conversion",
			set:  []string{"count=2,enabled=true"},
			want: map[string]any{"count": "2", "enabled": "true"},
		},
		{
			name: "quoted values may contain commas",
			set:  []string{`zone="zone1,zone2",imageTag=1.23.3`},
			want: map[string]any{"zone": `"zone1,zone2"`, "imageTag": "1.23.3"},
		},
		{
			name: "escaped commas within values",
			set:  []string{`zone=zone1\,zone2,imageTag=1.23.3`},
			want: map[string]any{"zone": "zone1,zone2", "imageTag": "1.23.3"},
		},
		{
			name: "backslashes and trailing backslash are preserved",
			set:  []string{`path=C:\tmp\config,trailing=C:\tmp\`},
			want: map[string]any{"path": `C:\tmp\config`, "trailing": `C:\tmp\`},
		},
		{
			name: "even backslashes leave comma as an assignment separator",
			set:  []string{`path=C:\\,count=2`},
			want: map[string]any{"path": `C:\\`, "count": "2"},
		},
		{
			name: "empty value",
			set:  []string{"message=,count=2"},
			want: map[string]any{"message": "", "count": "2"},
		},
		{
			name: "repeated flags overwrite earlier values",
			set:  []string{`zone=a\,b`, `zone=c\,d`},
			want: map[string]any{"zone": "c,d"},
		},
		{
			name: "escaped dots in keys are preserved for key parsing",
			set:  []string{`annotations.example\.com/key=first\,second`},
			want: map[string]any{
				"annotations": map[string]any{"example.com/key": "first,second"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGlobalImpl(&GlobalOptions{StateValuesSetString: tt.set})
			err := NewCLIConfigImpl(g)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, g.StateValuesSet())
		})
	}
}

func TestNewCLIConfigImplBothSetFlags(t *testing.T) {
	g := NewGlobalImpl(&GlobalOptions{
		StateValuesSet:       []string{"count=2"},
		StateValuesSetString: []string{"imageTag=1.23.3"},
	})
	require.NoError(t, NewCLIConfigImpl(g))

	want := map[string]any{
		"count":    int64(2),
		"imageTag": "1.23.3",
	}
	assert.Equal(t, want, g.StateValuesSet())
}

func TestSplitOnUnescapedCommas(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "no commas", input: "a=1", want: []string{"a=1"}},
		{name: "plain separators", input: "a=1,b=2", want: []string{"a=1", "b=2"}},
		{name: "escaped separator", input: `a=1\,2,b=3`, want: []string{`a=1\,2`, "b=3"}},
		{name: "escaped separator only", input: `a=1\,2`, want: []string{`a=1\,2`}},
		{name: "even backslashes unescape the separator", input: `a=1\\,b=2`, want: []string{`a=1\\`, "b=2"}},
		{name: "trailing separator", input: "a=1,", want: []string{"a=1", ""}},
		{name: "trailing backslash", input: `a=1\`, want: []string{`a=1\`}},
		{name: "unicode", input: `a=你好\,世界,b=2`, want: []string{`a=你好\,世界`, "b=2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, splitOnUnescapedCommas(tt.input))
		})
	}
}

func TestUnescapeCommas(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "no escapes", input: "a=1,b=2", want: "a=1,b=2"},
		{name: "escaped comma", input: `a=1\,2`, want: "a=1,2"},
		{name: "escaped dot preserved", input: `a\.b=1`, want: `a\.b=1`},
		{name: "even backslashes preserved", input: `a=1\\`, want: `a=1\\`},
		{name: "trailing backslash preserved", input: `a=1\`, want: `a=1\`},
		{name: "unicode preserved", input: `a=你好\,世界`, want: "a=你好,世界"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, unescapeCommas(tt.input))
		})
	}
}
