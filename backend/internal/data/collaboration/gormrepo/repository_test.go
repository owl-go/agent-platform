package gormrepo

import "testing"

func TestCommonLaunchPrerequisiteRequiresACommonCause(t *testing.T) {
	tests := []struct {
		name         string
		releaseCount int
		unavailable  map[string]int
		want         string
	}{
		{name: "no releases", unavailable: map[string]int{}, want: "release"},
		{name: "all runtimes unavailable", releaseCount: 2, unavailable: map[string]int{"runtime": 2}, want: "runtime"},
		{name: "different dependency failures", releaseCount: 2, unavailable: map[string]int{"runtime": 1, "model": 1}, want: "release"},
		{name: "all bindings invalid", releaseCount: 3, unavailable: map[string]int{"binding": 3}, want: "binding"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := commonLaunchPrerequisite(test.releaseCount, test.unavailable); got != test.want {
				t.Fatalf("commonLaunchPrerequisite() = %q, want %q", got, test.want)
			}
		})
	}
}
