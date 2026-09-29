package domain

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

const MaxExecutionEvidence = 64

// Evidence is a bounded, owner-visible execution fact derived from a platform
// retrieval or broker boundary. It must never contain raw tool output, prompts,
// model responses, credentials, or unrestricted external payloads.
type Evidence struct {
	ID            string            `json:"id"`
	Kind          string            `json:"kind"`
	SourceID      string            `json:"source_id"`
	SourceName    string            `json:"source_name"`
	ContainerID   string            `json:"container_id,omitempty"`
	State         string            `json:"state"`
	Action        string            `json:"action"`
	StagePosition int               `json:"stage_position"`
	Citation      *EvidenceCitation `json:"citation,omitempty"`
}

type EvidenceCitation struct {
	RevisionID     string  `json:"revision_id"`
	CategoryName   string  `json:"category_name,omitempty"`
	SourceLocation string  `json:"source_location,omitempty"`
	Relevance      float32 `json:"relevance,omitempty"`
}

func ValidateEvidence(items []Evidence) error {
	if len(items) > MaxExecutionEvidence {
		return fmt.Errorf("%w: execution Evidence exceeds %d items", ErrInvalid, MaxExecutionEvidence)
	}
	for index, item := range items {
		if item.ID == "" || item.SourceID == "" || strings.TrimSpace(item.SourceName) == "" || strings.TrimSpace(item.Action) == "" || item.StagePosition <= 0 {
			return fmt.Errorf("%w: execution Evidence %d identity is invalid", ErrInvalid, index)
		}
		if item.Kind != "file" && item.Kind != "knowledge" && item.Kind != "connector" && item.Kind != "artifact" {
			return fmt.Errorf("%w: execution Evidence %d kind is invalid", ErrInvalid, index)
		}
		if item.State != "requested" && item.State != "succeeded" && item.State != "failed" && item.State != "not_used" {
			return fmt.Errorf("%w: execution Evidence %d state is invalid", ErrInvalid, index)
		}
		if !boundedEvidenceText(item.ID, 240) || !boundedEvidenceText(item.SourceName, 255) || !boundedEvidenceText(item.Action, 240) || !boundedEvidenceText(item.SourceID, 240) || !boundedEvidenceText(item.ContainerID, 240) {
			return fmt.Errorf("%w: execution Evidence %d text is invalid", ErrInvalid, index)
		}
		if citation := item.Citation; citation != nil {
			if item.Kind != "knowledge" || strings.TrimSpace(citation.RevisionID) == "" || math.IsNaN(float64(citation.Relevance)) || math.IsInf(float64(citation.Relevance), 0) || citation.Relevance < 0 || citation.Relevance > 1 || !boundedEvidenceText(citation.RevisionID, 240) || !boundedEvidenceText(citation.CategoryName, 255) || !boundedEvidenceText(citation.SourceLocation, 500) {
				return fmt.Errorf("%w: execution Evidence %d citation is invalid", ErrInvalid, index)
			}
		}
		if item.Kind == "knowledge" && item.State == "succeeded" && (strings.TrimSpace(item.ContainerID) == "" || item.Citation == nil) {
			return fmt.Errorf("%w: execution Evidence %d Knowledge citation is incomplete", ErrInvalid, index)
		}
	}
	return nil
}

func boundedEvidenceText(value string, maximum int) bool {
	return !strings.ContainsRune(value, '\x00') && utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximum
}
