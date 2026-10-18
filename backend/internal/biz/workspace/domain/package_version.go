package domain

import "strings"

// ComparePackageVersions compares already validated SemVer precedence, ignoring
// build metadata. It avoids integer overflow for arbitrarily long identifiers.
func ComparePackageVersions(left, right string) int {
	release := func(value string) ([]string, []string) {
		value = strings.SplitN(value, "+", 2)[0]
		parts := strings.SplitN(value, "-", 2)
		var pre []string
		if len(parts) == 2 {
			pre = strings.Split(parts[1], ".")
		}
		return strings.Split(parts[0], "."), pre
	}
	l, lp := release(left)
	r, rp := release(right)
	if len(l) != 3 || len(r) != 3 {
		return 0
	}
	for i := 0; i < 3; i++ {
		if c := numericVersionCompare(l[i], r[i]); c != 0 {
			return c
		}
	}
	if len(lp) == 0 && len(rp) == 0 {
		return 0
	}
	if len(lp) == 0 {
		return 1
	}
	if len(rp) == 0 {
		return -1
	}
	for i := 0; i < len(lp) && i < len(rp); i++ {
		ln := strings.Trim(lp[i], "0123456789") == ""
		rn := strings.Trim(rp[i], "0123456789") == ""
		if ln && rn {
			if c := numericVersionCompare(lp[i], rp[i]); c != 0 {
				return c
			}
		} else if ln != rn {
			if ln {
				return -1
			}
			return 1
		} else if c := strings.Compare(lp[i], rp[i]); c != 0 {
			return c
		}
	}
	if len(lp) < len(rp) {
		return -1
	}
	if len(lp) > len(rp) {
		return 1
	}
	return 0
}
func numericVersionCompare(left, right string) int {
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	return strings.Compare(left, right)
}
