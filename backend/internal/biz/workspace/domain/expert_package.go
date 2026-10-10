package domain

// ExpertPackageDefinition contains only validated portable definition content.
// Import grants no account authorization and never selects execution settings.
type ExpertPackageDefinition struct {
	PackageID        string
	PackageVersion   string
	SHA256           string
	Archive          []byte
	TargetResourceID string
	ExpectedVersion  int64
	CopyName         string
	Expert           *ExpertInput
	Team             *ExpertTeamInput
}

type ExpertPackageImport struct {
	Expert   *Expert
	Team     *ExpertTeam
	Replayed bool
}
