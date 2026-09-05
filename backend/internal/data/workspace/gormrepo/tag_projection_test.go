package gormrepo

import "testing"

func TestParseProjectedTagsNormalizesAndLimitsModelOutput(t *testing.T) {
	tags, err := parseProjectedTags("```json\n[\" Go \", \"Architecture\", \"go\", \"Testing\", \"Security\", \"Delivery\", \"Ignored\"]\n```")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Go", "Architecture", "Testing", "Security", "Delivery"}
	if len(tags) != len(want) {
		t.Fatalf("tags = %#v", tags)
	}
	for index := range want {
		if tags[index] != want[index] {
			t.Fatalf("tags = %#v", tags)
		}
	}
}
