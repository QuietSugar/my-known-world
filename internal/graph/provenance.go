package graph

import (
	"fmt"
	"strings"
)

type EvidenceKind string

const (
	EvidenceUnknown  EvidenceKind = "unknown"
	EvidenceManual   EvidenceKind = "manual"
	EvidenceDocument EvidenceKind = "document"
	EvidenceObserved EvidenceKind = "observed"
	EvidenceDerived  EvidenceKind = "derived"
	EvidenceImported EvidenceKind = "imported"
)

type Certainty string

const (
	CertaintyUnknown     Certainty = "unknown"
	CertaintyProvisional Certainty = "provisional"
	CertaintyConfirmed   Certainty = "confirmed"
	CertaintyDisputed    Certainty = "disputed"
	CertaintyStale       Certainty = "stale"
)

func normalizeEvidence(source string, kind EvidenceKind, certainty Certainty) (string, EvidenceKind, Certainty, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		source = "unspecified"
	}
	if kind == "" {
		kind = EvidenceUnknown
	}
	if certainty == "" {
		certainty = CertaintyUnknown
	}
	switch kind {
	case EvidenceUnknown, EvidenceManual, EvidenceDocument, EvidenceObserved, EvidenceDerived, EvidenceImported:
	default:
		return "", "", "", fmt.Errorf("unknown evidence kind %q", kind)
	}
	switch certainty {
	case CertaintyUnknown, CertaintyProvisional, CertaintyConfirmed, CertaintyDisputed, CertaintyStale:
	default:
		return "", "", "", fmt.Errorf("unknown certainty %q", certainty)
	}
	return source, kind, certainty, nil
}
