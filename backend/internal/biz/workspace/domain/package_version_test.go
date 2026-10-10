package domain

import "testing"

func TestComparePackageVersionPrecedence(t *testing.T) {
	for _, tc := range []struct {
		left, right string
		want        int
	}{
		{"1.0.0", "1.0.0+build", 0}, {"1.0.0", "1.0.0-rc.1", 1},
		{"1.0.0-alpha.9", "1.0.0-alpha.10", -1}, {"1.0.0-9", "1.0.0-alpha", -1},
		{"1.0.0-alpha", "1.0.0-alpha.1", -1}, {"999999999999999999999.0.0", "99999999999999999999.0.0", 1},
	} {
		got := ComparePackageVersions(tc.left, tc.right)
		if got < 0 {
			got = -1
		}
		if got > 0 {
			got = 1
		}
		if got != tc.want {
			t.Errorf("%s against %s = %d", tc.left, tc.right, got)
		}
	}
}
func TestTeamDelegationRequiresExplicitRequiredFlag(t *testing.T) {
	roster := []ExecutionStageSnapshot{{TeamMemberID: "reviewer"}}
	for _, field := range []string{"", `,"required":null`, `,"required":"false"`} {
		_, err := ParseTeamAction(`{"action":"delegate","tasks":[{"id":"review","member_id":"reviewer","instruction":"Review"`+field+`}]}`, roster)
		if err == nil {
			t.Errorf("accepted missing or invalid required flag: %s", field)
		}
	}
	if _, err := ParseTeamAction(`{"action":"delegate","tasks":[{"id":"review","member_id":"reviewer","instruction":"Review","required":false}]}`, roster); err != nil {
		t.Fatal(err)
	}
}
