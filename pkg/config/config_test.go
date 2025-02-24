package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCLIConfigImpl_StateValuesSet(t *testing.T) {
	tests := []struct {
		name string
		set  []string
		want map[string]any
	}{
		{
			name: "no overrides",
			want: map[string]any{},
		},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGlobalImpl(&GlobalOptions{StateValuesSet: tt.set})
			require.NoError(t, NewCLIConfigImpl(g))
			assert.Equal(t, tt.want, g.StateValuesSet())
		})
	}
}
