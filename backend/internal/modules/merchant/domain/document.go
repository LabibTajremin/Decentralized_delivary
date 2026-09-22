package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Errors returned when handling documents.
var (
	// ErrUnknownDocumentKind means the document type is not one we collect.
	ErrUnknownDocumentKind = errors.New("unknown document type")
	// ErrEmptyDocumentNumber means the licence or ID number is missing.
	ErrEmptyDocumentNumber = errors.New("a document number is required")
	// ErrEmptyDocumentFile means no scan was attached.
	ErrEmptyDocumentFile = errors.New("a photo or scan of the document is required")
	// ErrDocumentsIncomplete means a required document has not been supplied.
	ErrDocumentsIncomplete = errors.New("some required documents are missing")
	// ErrDocumentNotRequired means this type of shop does not need that document.
	ErrDocumentNotRequired = errors.New("that document is not required for this kind of shop")
)

const maxDocumentNumberLength = 60

// DocumentKind is a paper we require before a shop may trade.
type DocumentKind string

// The documents we collect.
const (
	// DocTradeLicence is the city or union council trade licence. Every shop
	// needs one.
	DocTradeLicence DocumentKind = "trade_licence"
	// DocNationalID is the owner's NID, so there is a person behind the shop.
	DocNationalID DocumentKind = "national_id"
	// DocFoodLicence is the BSTI/food safety licence a restaurant needs.
	DocFoodLicence DocumentKind = "food_licence"
	// DocDrugLicence is the drug licence a pharmacy needs. Without it we would
	// be a delivery channel for unlicensed medicine.
	DocDrugLicence DocumentKind = "drug_licence"
)

// AllDocumentKinds lists every document we collect, in a fixed order.
func AllDocumentKinds() []DocumentKind {
	return []DocumentKind{DocTradeLicence, DocNationalID, DocFoodLicence, DocDrugLicence}
}

// ParseDocumentKind reads a document type.
func ParseDocumentKind(s string) (DocumentKind, error) {
	candidate := DocumentKind(strings.TrimSpace(strings.ToLower(s)))
	for _, known := range AllDocumentKinds() {
		if candidate == known {
			return known, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownDocumentKind, s)
}

// String returns the document code.
func (k DocumentKind) String() string { return string(k) }

// RequiredDocuments returns what a type of shop must supply before review.
//
// Per type rather than one list for everyone: asking a grocer for a drug
// licence they cannot have would block registration for a shop that is
// perfectly legal, and not asking a pharmacy for one would make us the
// distribution channel for whoever skipped it.
func RequiredDocuments(t Type) []DocumentKind {
	switch t {
	case TypeRestaurant:
		return []DocumentKind{DocTradeLicence, DocNationalID, DocFoodLicence}
	case TypePharmacy:
		return []DocumentKind{DocTradeLicence, DocNationalID, DocDrugLicence}
	case TypeGrocery:
		return []DocumentKind{DocTradeLicence, DocNationalID}
	default:
		return nil
	}
}

// Document is one paper an owner has supplied.
type Document struct {
	Kind   DocumentKind
	Number string
	// FileURL points at the stored scan. The merchant module does not host it:
	// storage is an infrastructure decision, and keeping only the location here
	// means moving to object storage later touches nothing in this package.
	FileURL    string
	UploadedAt time.Time
}

// NewDocument validates a supplied document.
func NewDocument(kind DocumentKind, number, fileURL string, at time.Time) (Document, error) {
	parsed, err := ParseDocumentKind(kind.String())
	if err != nil {
		return Document{}, err
	}

	number = strings.TrimSpace(number)
	if number == "" {
		return Document{}, ErrEmptyDocumentNumber
	}
	if utf8.RuneCountInString(number) > maxDocumentNumberLength {
		return Document{}, fmt.Errorf("%w: limit %d characters", ErrTooLong, maxDocumentNumberLength)
	}

	fileURL = strings.TrimSpace(fileURL)
	if fileURL == "" {
		return Document{}, ErrEmptyDocumentFile
	}

	return Document{Kind: parsed, Number: number, FileURL: fileURL, UploadedAt: at}, nil
}

// WithDocument returns a copy carrying the document, replacing any earlier one
// of the same kind.
//
// Replacing rather than appending: an owner who re-uploads a licence is
// correcting the last one, and keeping both would leave a reviewer choosing
// between two numbers with nothing to say which is current.
func (m Merchant) WithDocument(d Document) Merchant {
	documents := make([]Document, 0, len(m.Documents)+1)
	replaced := false
	for _, existing := range m.Documents {
		if existing.Kind == d.Kind {
			documents = append(documents, d)
			replaced = true
			continue
		}
		documents = append(documents, existing)
	}
	if !replaced {
		documents = append(documents, d)
	}
	m.Documents = documents
	return m
}

// Document returns one supplied document.
func (m Merchant) Document(kind DocumentKind) (Document, bool) {
	for _, d := range m.Documents {
		if d.Kind == kind {
			return d, true
		}
	}
	return Document{}, false
}

// MissingDocuments lists what is still required, in the order it is asked for.
func (m Merchant) MissingDocuments() []DocumentKind {
	var missing []DocumentKind
	for _, required := range RequiredDocuments(m.Type) {
		if _, ok := m.Document(required); !ok {
			missing = append(missing, required)
		}
	}
	return missing
}

// RequiresDocument reports whether this shop must supply that document.
func (m Merchant) RequiresDocument(kind DocumentKind) bool {
	for _, required := range RequiredDocuments(m.Type) {
		if required == kind {
			return true
		}
	}
	return false
}

// ReadyForReview reports whether the shop can be submitted.
func (m Merchant) ReadyForReview() error {
	if !m.Placement.IsPlaced() {
		return ErrUnplaced
	}
	if missing := m.MissingDocuments(); len(missing) > 0 {
		names := make([]string, 0, len(missing))
		for _, kind := range missing {
			names = append(names, kind.String())
		}
		return fmt.Errorf("%w: %s", ErrDocumentsIncomplete, strings.Join(names, ", "))
	}
	return nil
}

// ErrUnplaced means the shop has not been resolved to a service area.
var ErrUnplaced = errors.New("this shop has not been placed in a service area")
