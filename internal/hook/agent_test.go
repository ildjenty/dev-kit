package hook

import "testing"

func TestFrontmatterValue(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{"sonnet # model selection", "sonnet"},
		{"'sonnet' # model selection", "sonnet"},
		{`"high" # effort selection`, "high"},
		{`"value#literal" # trailing comment`, "value#literal"},
		{"'value # literal' # trailing comment", "value # literal"},
		{"'it''s quoted' # comment", "it's quoted"},
		{`"quoted\"value" # comment`, `quoted"value`},
		{"value#literal", "value#literal"},
		{"# no value", ""},
		{" high\t# effort selection", "high"},
	} {
		t.Run(tc.source, func(t *testing.T) {
			got, err := frontmatterValue(tc.source)
			if err != nil || got != tc.want {
				t.Fatalf("parsed %q as %q, %v; want %q", tc.source, got, err, tc.want)
			}
		})
	}
}
