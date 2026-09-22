package merchant

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/domain"
)

func TestDocumentKindsRoundTripThroughTheirCodes(t *testing.T) {
	for _, kind := range domain.AllDocumentKinds() {
		parsed, err := domain.ParseDocumentKind(kind.String())
		if err != nil {
			t.Fatalf("ParseDocumentKind(%q): %v", kind, err)
		}
		if parsed != kind {
			t.Errorf("ParseDocumentKind(%q) = %q", kind, parsed)
		}
	}
	if _, err := domain.ParseDocumentKind("passport"); !errors.Is(err, domain.ErrUnknownDocumentKind) {
		t.Errorf("error = %v, want ErrUnknownDocumentKind", err)
	}
}

// TestEachKindOfShopIsAskedForWhatItsLawRequires: a grocer asked for a drug
// licence they cannot have is blocked from a perfectly legal registration, and
// a pharmacy not asked for one makes us the channel for whoever skipped it.
func TestEachKindOfShopIsAskedForWhatItsLawRequires(t *testing.T) {
	want := map[domain.Type][]domain.DocumentKind{
		domain.TypeRestaurant: {domain.DocTradeLicence, domain.DocNationalID, domain.DocFoodLicence},
		domain.TypePharmacy:   {domain.DocTradeLicence, domain.DocNationalID, domain.DocDrugLicence},
		domain.TypeGrocery:    {domain.DocTradeLicence, domain.DocNationalID},
	}

	for kind, required := range want {
		got := domain.RequiredDocuments(kind)
		if len(got) != len(required) {
			t.Errorf("%s requires %v, want %v", kind, got, required)
			continue
		}
		for i := range required {
			if got[i] != required[i] {
				t.Errorf("%s document %d = %q, want %q", kind, i, got[i], required[i])
			}
		}
	}
}

// TestAnUnknownShopTypeRequiresNothing: RequiredDocuments is reached with a
// parsed type everywhere in production, so this is the branch that keeps a
// future type from silently inheriting a restaurant's paperwork.
func TestAnUnknownShopTypeRequiresNothing(t *testing.T) {
	if got := domain.RequiredDocuments(domain.Type("hardware")); got != nil {
		t.Errorf("RequiredDocuments(hardware) = %v, want nil", got)
	}
}

func TestADocumentNeedsANumberAndAScan(t *testing.T) {
	now := time.Now()

	if _, err := domain.NewDocument(domain.DocTradeLicence, "  ", "/f.png", now); !errors.Is(err, domain.ErrEmptyDocumentNumber) {
		t.Errorf("error = %v, want ErrEmptyDocumentNumber", err)
	}
	if _, err := domain.NewDocument(domain.DocTradeLicence, "TRAD-1", "  ", now); !errors.Is(err, domain.ErrEmptyDocumentFile) {
		t.Errorf("error = %v, want ErrEmptyDocumentFile", err)
	}
	if _, err := domain.NewDocument(domain.DocumentKind("passport"), "P-1", "/f.png", now); !errors.Is(err, domain.ErrUnknownDocumentKind) {
		t.Errorf("error = %v, want ErrUnknownDocumentKind", err)
	}
	if _, err := domain.NewDocument(domain.DocTradeLicence, strings.Repeat("1", 61), "/f.png", now); !errors.Is(err, domain.ErrTooLong) {
		t.Errorf("error = %v, want ErrTooLong", err)
	}
}

func TestADocumentIsTrimmed(t *testing.T) {
	document, err := domain.NewDocument(domain.DocTradeLicence, "  TRAD-1 ", "  /f.png ", time.Now())
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}
	if document.Number != "TRAD-1" || document.FileURL != "/f.png" {
		t.Errorf("document = %+v", document)
	}
}

// TestReUploadingADocumentReplacesTheLastOne: keeping both would leave a
// reviewer choosing between two numbers with nothing to say which is current.
func TestReUploadingADocumentReplacesTheLastOne(t *testing.T) {
	merchant := draftMerchant(t, domain.TypeGrocery)

	first, err := domain.NewDocument(domain.DocTradeLicence, "OLD", "/old.png", time.Now())
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}
	second, err := domain.NewDocument(domain.DocTradeLicence, "NEW", "/new.png", time.Now())
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}

	merchant = merchant.WithDocument(first).WithDocument(second)
	if len(merchant.Documents) != 1 {
		t.Fatalf("documents = %d, want 1", len(merchant.Documents))
	}
	stored, ok := merchant.Document(domain.DocTradeLicence)
	if !ok || stored.Number != "NEW" {
		t.Errorf("stored document = %+v, want the replacement", stored)
	}
}

func TestADocumentThatWasNeverSuppliedIsNotFound(t *testing.T) {
	if _, ok := draftMerchant(t, domain.TypeGrocery).Document(domain.DocTradeLicence); ok {
		t.Error("a document that was never supplied was found")
	}
}

// TestWithDocumentLeavesTheOriginalAlone: every domain mutator returns a copy,
// so a caller holding the previous value still sees what it read.
func TestWithDocumentLeavesTheOriginalAlone(t *testing.T) {
	before := draftMerchant(t, domain.TypeGrocery)
	document, err := domain.NewDocument(domain.DocTradeLicence, "TRAD-1", "/f.png", time.Now())
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}

	_ = before.WithDocument(document)
	if len(before.Documents) != 0 {
		t.Errorf("the original merchant grew a document")
	}
}

func TestMissingDocumentsAreListedInTheOrderTheyAreAskedFor(t *testing.T) {
	merchant := draftMerchant(t, domain.TypePharmacy)

	nid, err := domain.NewDocument(domain.DocNationalID, "NID-1", "/nid.png", time.Now())
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}
	merchant = merchant.WithDocument(nid)

	missing := merchant.MissingDocuments()
	if len(missing) != 2 || missing[0] != domain.DocTradeLicence || missing[1] != domain.DocDrugLicence {
		t.Errorf("missing = %v, want [trade_licence drug_licence]", missing)
	}
}

func TestAShopKnowsWhichDocumentsItNeeds(t *testing.T) {
	grocery := draftMerchant(t, domain.TypeGrocery)
	if !grocery.RequiresDocument(domain.DocTradeLicence) {
		t.Error("a grocery does not require a trade licence")
	}
	if grocery.RequiresDocument(domain.DocDrugLicence) {
		t.Error("a grocery requires a drug licence")
	}
}

// TestAShopIsNotReadyForReviewUntilItIsPlacedAndDocumented covers both gates,
// because either one alone would let a shop reach an admin in a state the admin
// cannot decide on.
func TestAShopIsNotReadyForReviewUntilItIsPlacedAndDocumented(t *testing.T) {
	unplaced := draftMerchant(t, domain.TypeGrocery)
	if err := unplaced.ReadyForReview(); !errors.Is(err, domain.ErrUnplaced) {
		t.Errorf("unplaced: error = %v, want ErrUnplaced", err)
	}

	placed := unplaced.WithPlacement(domain.Placement{DivisionCode: "BD-C"})
	err := placed.ReadyForReview()
	if !errors.Is(err, domain.ErrDocumentsIncomplete) {
		t.Fatalf("undocumented: error = %v, want ErrDocumentsIncomplete", err)
	}
	// The error names what is missing, so the owner is told what to do rather
	// than only that something is wrong.
	if !strings.Contains(err.Error(), "trade_licence") || !strings.Contains(err.Error(), "national_id") {
		t.Errorf("error %q does not name the missing documents", err)
	}

	documented := placed
	for _, kind := range domain.RequiredDocuments(documented.Type) {
		document, docErr := domain.NewDocument(kind, "N-1", "/f.png", time.Now())
		if docErr != nil {
			t.Fatalf("NewDocument: %v", docErr)
		}
		documented = documented.WithDocument(document)
	}
	if err := documented.ReadyForReview(); err != nil {
		t.Errorf("a placed, fully documented shop is not ready: %v", err)
	}
}

// draftMerchant is a shop of a given type, in draft, with nothing attached.
func draftMerchant(t *testing.T, kind domain.Type) domain.Merchant {
	t.Helper()
	details := validDetails()
	details.Type = kind

	merchant, err := domain.NewMerchant("mch_1", "usr_1", details, at(2026, time.March, 1, 9, 0))
	if err != nil {
		t.Fatalf("NewMerchant: %v", err)
	}
	return merchant
}
