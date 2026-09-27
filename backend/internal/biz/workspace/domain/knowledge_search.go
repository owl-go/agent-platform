package domain

// KnowledgeSearchSource identifies the active, readable document revision
// behind a retrieved excerpt. Provider metadata alone is not trusted as a
// citation source.
type KnowledgeSearchSource struct {
	DocumentID   string
	RevisionID   string
	DocumentName string
	CategoryName string
}
