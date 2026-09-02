package workspace

import "testing"

func TestAttachmentNameRejectsPathsAndControlCharacters(t *testing.T) {
	for _, test := range []struct {
		value string
		want  string
		valid bool
	}{
		{value: "photo.png", want: "photo.png", valid: true},
		{value: "../notes.txt", valid: false},
		{value: "", valid: false},
		{value: "bad\nname.txt", valid: false},
	} {
		got, err := attachmentName(test.value)
		if test.valid && (err != nil || got != test.want) {
			t.Fatalf("attachmentName(%q) = %q, %v", test.value, got, err)
		}
		if !test.valid && err == nil {
			t.Fatalf("attachmentName(%q) unexpectedly succeeded", test.value)
		}
	}
}
