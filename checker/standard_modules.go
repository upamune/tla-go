package checker

import _ "embed"

// These modules come from TLA+ Tools v1.7.4, commit 5a47802b5c391f59ecdd44117981f4ff8c0656ba.
// See standard/LICENSE for the upstream MIT license.

//go:embed standard/Reals.tla
var standardReals string

//go:embed standard/RealTime.tla
var standardRealTime string

func embeddedStandardSource(name string) (string, bool) {
	switch name {
	case "Reals":
		return standardReals, true
	case "RealTime":
		return standardRealTime, true
	default:
		return "", false
	}
}
