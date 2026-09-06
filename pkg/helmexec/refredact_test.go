package helmexec

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactedRef(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want string
	}{
		{
			name: "plain chart path unchanged",
			ref:  "./charts/demo",
			want: "./charts/demo",
		},
		{
			name: "https userinfo fully masked (username may carry the token)",
			ref:  "https://user:token@charts.example.com/repo",
			want: "https://xxxxx@charts.example.com/repo",
		},
		{
			name: "go-getter forced form preserved and sanitized",
			ref:  "git::https://x-access-token@github.com/org/repo.git//helmfile?ref=v1",
			want: "git::https://xxxxx@github.com/org/repo.git//helmfile?ref=v1",
		},
		{
			name: "s3 query credentials masked",
			ref:  "s3::https://s3.amazonaws.com/bucket/helmfile?aws_access_key_id=AKIA&aws_secret_access_key=zzz&region=us-east-1",
			want: "s3::https://s3.amazonaws.com/bucket/helmfile?aws_access_key_id=xxxxx&aws_secret_access_key=xxxxx&region=us-east-1",
		},
		{
			name: "generic token query param masked",
			ref:  "https://example.com/file?token=abc&path=ok",
			want: "https://example.com/file?path=ok&token=xxxxx",
		},
		{
			name: "non-credential query untouched",
			ref:  "https://example.com/chart?ref=main&verify=true",
			want: "https://example.com/chart?ref=main&verify=true",
		},
		{
			name: "URL without scheme or host returned unchanged",
			ref:  "git@github.com:org/repo.git",
			want: "git@github.com:org/repo.git",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, RedactedRef(tt.ref))
		})
	}
}
