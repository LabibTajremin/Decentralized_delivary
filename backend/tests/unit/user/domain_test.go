package user

import (
	"errors"
	"strings"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/user/domain"
)

// ---------------------------------------------------------------- profile

func TestAProfileNeedsAnAccount(t *testing.T) {
	if _, err := domain.NewProfile("  ", "Ayesha", "", domain.LanguageBengali); !errors.Is(err, domain.ErrEmptyUserID) {
		t.Errorf("error = %v, want ErrEmptyUserID", err)
	}
}

// A customer can order without giving a name: demanding one at sign-up costs
// more sign-ups than the name is worth.
func TestANameIsOptional(t *testing.T) {
	profile, err := domain.NewProfile("usr_1", "", "", domain.LanguageBengali)
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	if profile.Name != "" {
		t.Errorf("name = %q", profile.Name)
	}
}

// TestNameLengthIsCountedInCharactersNotBytes: a Bengali name is three bytes
// per character, and a byte limit would give it a third of the room an English
// name gets.
func TestNameLengthIsCountedInCharactersNotBytes(t *testing.T) {
	bengali := strings.Repeat("অ", 80) // 80 characters, 240 bytes
	if _, err := domain.NewProfile("usr_1", bengali, "", domain.LanguageBengali); err != nil {
		t.Errorf("an 80-character Bengali name was refused: %v", err)
	}

	tooLong := strings.Repeat("অ", 81)
	if _, err := domain.NewProfile("usr_1", tooLong, "", domain.LanguageBengali); !errors.Is(err, domain.ErrNameTooLong) {
		t.Errorf("error = %v, want ErrNameTooLong", err)
	}
}

func TestProfileFieldsAreTrimmedAndNormalised(t *testing.T) {
	profile, err := domain.NewProfile("usr_1", "  Ayesha Rahman  ", "  Ayesha@Example.COM ", domain.LanguageEnglish)
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	if profile.Name != "Ayesha Rahman" {
		t.Errorf("name = %q", profile.Name)
	}
	if profile.Email != "ayesha@example.com" {
		t.Errorf("email = %q, want it lower-cased and trimmed", profile.Email)
	}
}

// The email check is deliberately loose: the only way to know an address works
// is to send to it, and a strict pattern turns away real customers.
func TestPlausibleEmailsAreAccepted(t *testing.T) {
	for _, email := range []string{
		"ayesha@example.com",
		"a.b+tag@sub.example.co.uk",
		"o'brien@example.com",
		"user_name@example.museum",
	} {
		if _, err := domain.NewProfile("usr_1", "", email, domain.LanguageBengali); err != nil {
			t.Errorf("NewProfile(%q): %v", email, err)
		}
	}
}

func TestUnusableEmailsAreRejected(t *testing.T) {
	for _, email := range []string{
		"no-at-sign",
		"@example.com",
		"user@",
		"user@nodot",
		"user@.example.com",
		"user@example.com.",
		"two@at@example.com",
		"has space@example.com",
		"has\nnewline@example.com",
	} {
		if _, err := domain.NewProfile("usr_1", "", email, domain.LanguageBengali); !errors.Is(err, domain.ErrInvalidEmail) {
			t.Errorf("NewProfile(%q) = %v, want ErrInvalidEmail", email, err)
		}
	}
}

func TestAnEmptyEmailIsFine(t *testing.T) {
	if _, err := domain.NewProfile("usr_1", "Ayesha", "   ", domain.LanguageBengali); err != nil {
		t.Errorf("an empty email was refused: %v", err)
	}
}

func TestLanguagesRoundTrip(t *testing.T) {
	for _, language := range domain.AllLanguages() {
		got, err := domain.ParseLanguage(language.String())
		if err != nil || got != language {
			t.Errorf("%s round-tripped to %v, %v", language, got, err)
		}
	}
	if got, err := domain.ParseLanguage("  EN  "); err != nil || got != domain.LanguageEnglish {
		t.Errorf("ParseLanguage with case and space = %v, %v", got, err)
	}
	if got, err := domain.ParseLanguage(""); err != nil || got != domain.DefaultLanguage {
		t.Errorf("an empty language = %v, %v; want the default", got, err)
	}
	if _, err := domain.ParseLanguage("fr"); !errors.Is(err, domain.ErrUnsupportedLanguage) {
		t.Errorf("ParseLanguage(fr) = %v", err)
	}
}

func TestAnEmptyLanguageDefaults(t *testing.T) {
	profile, err := domain.NewProfile("usr_1", "Ayesha", "", "")
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	if profile.Language != domain.DefaultLanguage {
		t.Errorf("language = %q, want %q", profile.Language, domain.DefaultLanguage)
	}
}

func TestAnUnsupportedLanguageIsRefused(t *testing.T) {
	if _, err := domain.NewProfile("usr_1", "Ayesha", "", domain.Language("fr")); !errors.Is(err, domain.ErrUnsupportedLanguage) {
		t.Errorf("error = %v, want ErrUnsupportedLanguage", err)
	}
}

// The server decides the greeting fallback: an app that says "there" and
// another that renders a blank line is two products.
func TestTheDisplayNameNeverComesBackEmpty(t *testing.T) {
	named, _ := domain.NewProfile("usr_1", "Ayesha", "", domain.LanguageBengali)
	if named.DisplayName() != "Ayesha" {
		t.Errorf("display name = %q", named.DisplayName())
	}

	bengali, _ := domain.NewProfile("usr_1", "", "", domain.LanguageBengali)
	if bengali.DisplayName() == "" {
		t.Error("an unnamed Bengali profile has no display name")
	}
	english, _ := domain.NewProfile("usr_1", "", "", domain.LanguageEnglish)
	if english.DisplayName() != "Customer" {
		t.Errorf("display name = %q, want an English fallback", english.DisplayName())
	}
	if bengali.DisplayName() == english.DisplayName() {
		t.Error("the fallback is not translated")
	}
}

// ---------------------------------------------------------------- pin

func TestAPinMustBeOnEarth(t *testing.T) {
	for _, c := range []struct{ lat, lng float64 }{
		{91, 90}, {-91, 90}, {23, 181}, {23, -181},
	} {
		if _, err := domain.NewPin(c.lat, c.lng); !errors.Is(err, domain.ErrInvalidPin) {
			t.Errorf("NewPin(%v, %v) was accepted", c.lat, c.lng)
		}
	}
}

// TestTheNullIslandPinIsRefused: (0,0) is in the Gulf of Guinea and is almost
// always an uninitialised variable rather than where somebody lives. Storing it
// means an order that cannot be delivered, discovered hours later at dispatch.
func TestTheNullIslandPinIsRefused(t *testing.T) {
	if _, err := domain.NewPin(0, 0); !errors.Is(err, domain.ErrInvalidPin) {
		t.Errorf("error = %v, want the unset pin refused", err)
	}
}

func TestAValidPinIsAccepted(t *testing.T) {
	pin, err := domain.NewPin(23.7461, 90.3742)
	if err != nil {
		t.Fatalf("NewPin: %v", err)
	}
	if pin.Lat != 23.7461 || pin.Lng != 90.3742 {
		t.Errorf("pin = %+v", pin)
	}
}

// ---------------------------------------------------------------- address

func validPin(t *testing.T) domain.Pin {
	t.Helper()
	pin, err := domain.NewPin(23.7461, 90.3742)
	if err != nil {
		t.Fatalf("NewPin: %v", err)
	}
	return pin
}

func newAddress(t *testing.T, mutate func(*addressArgs)) (domain.Address, error) {
	t.Helper()
	args := addressArgs{
		id: "adr_1", userID: "usr_1", label: "Home",
		recipientName: "Ayesha", recipientPhone: "01712345678",
		line1: "House 12, Road 7", line2: "Dhanmondi", instructions: "Blue gate",
		pin: validPin(t),
	}
	if mutate != nil {
		mutate(&args)
	}
	return domain.NewAddress(args.id, args.userID, args.label, args.recipientName,
		args.recipientPhone, args.line1, args.line2, args.instructions, args.pin)
}

type addressArgs struct {
	id, userID, label             string
	recipientName, recipientPhone string
	line1, line2, instructions    string
	pin                           domain.Pin
}

func TestAValidAddressIsBuilt(t *testing.T) {
	address, err := newAddress(t, nil)
	if err != nil {
		t.Fatalf("NewAddress: %v", err)
	}
	if address.Line1 != "House 12, Road 7" || address.RecipientPhone != "01712345678" {
		t.Errorf("address = %+v", address)
	}
}

func TestAnAddressNeedsAStreetLine(t *testing.T) {
	if _, err := newAddress(t, func(a *addressArgs) { a.line1 = "   " }); !errors.Is(err, domain.ErrEmptyAddressLine) {
		t.Errorf("error = %v, want ErrEmptyAddressLine", err)
	}
}

// The recipient may not be the account holder — people send food to their
// parents — so the rider needs a name and a number for whoever opens the door.
func TestAnAddressNeedsARecipient(t *testing.T) {
	if _, err := newAddress(t, func(a *addressArgs) { a.recipientName = "" }); !errors.Is(err, domain.ErrMissingRecipient) {
		t.Errorf("no name = %v, want ErrMissingRecipient", err)
	}
	if _, err := newAddress(t, func(a *addressArgs) { a.recipientPhone = " " }); !errors.Is(err, domain.ErrMissingRecipient) {
		t.Errorf("no phone = %v, want ErrMissingRecipient", err)
	}
}

func TestOverlongAddressFieldsAreRefused(t *testing.T) {
	cases := map[string]func(*addressArgs){
		"label":        func(a *addressArgs) { a.label = strings.Repeat("x", 41) },
		"line1":        func(a *addressArgs) { a.line1 = strings.Repeat("x", 201) },
		"line2":        func(a *addressArgs) { a.line2 = strings.Repeat("x", 201) },
		"recipient":    func(a *addressArgs) { a.recipientName = strings.Repeat("x", 81) },
		"instructions": func(a *addressArgs) { a.instructions = strings.Repeat("x", 301) },
	}
	for name, mutate := range cases {
		if _, err := newAddress(t, mutate); err == nil {
			t.Errorf("an overlong %s was accepted", name)
		}
	}
}

func TestAnAddressNeedsAPin(t *testing.T) {
	if _, err := newAddress(t, func(a *addressArgs) { a.pin = domain.Pin{} }); !errors.Is(err, domain.ErrInvalidPin) {
		t.Errorf("error = %v, want the missing pin refused", err)
	}
}

func TestAnAddressNeedsAnAccount(t *testing.T) {
	if _, err := newAddress(t, func(a *addressArgs) { a.userID = "" }); !errors.Is(err, domain.ErrEmptyUserID) {
		t.Errorf("error = %v, want ErrEmptyUserID", err)
	}
}

// An unplaced address is one the domain can build but the use case refuses to
// save: placing it is a separate step that can fail for reasons the customer
// cannot fix.
func TestAnAddressStartsUnplaced(t *testing.T) {
	address, err := newAddress(t, nil)
	if err != nil {
		t.Fatalf("NewAddress: %v", err)
	}
	if address.Placement.IsPlaced() {
		t.Error("a freshly built address reports itself as placed")
	}

	placed := address.WithPlacement(domain.Placement{
		AreaCode: "DHK-DHM", AreaName: "Dhanmondi", DistrictCode: "DHK", DivisionCode: "DHA",
	})
	if !placed.Placement.IsPlaced() {
		t.Error("a placed address does not report itself as placed")
	}
	if address.Placement.IsPlaced() {
		t.Error("WithPlacement mutated the original")
	}
}

func TestTheDisplayLabelFallsBackToTheStreet(t *testing.T) {
	labelled, _ := newAddress(t, nil)
	if labelled.DisplayLabel() != "Home" {
		t.Errorf("label = %q", labelled.DisplayLabel())
	}
	unlabelled, _ := newAddress(t, func(a *addressArgs) { a.label = "" })
	if unlabelled.DisplayLabel() != "House 12, Road 7" {
		t.Errorf("label = %q, want the street line", unlabelled.DisplayLabel())
	}
}

// The server composes the single line so every surface shows the same string.
// A client that joins the parts itself will eventually join them differently.
func TestTheSingleLineIsComposedByTheServer(t *testing.T) {
	address, _ := newAddress(t, nil)
	placed := address.WithPlacement(domain.Placement{
		AreaCode: "DHK-DHM", AreaName: "Dhanmondi", DistrictCode: "DHK", DivisionCode: "DHA",
	})
	if got := placed.SingleLine(); got != "House 12, Road 7, Dhanmondi, Dhanmondi" {
		t.Errorf("single line = %q", got)
	}

	sparse, _ := newAddress(t, func(a *addressArgs) { a.line2 = "" })
	if got := sparse.SingleLine(); got != "House 12, Road 7" {
		t.Errorf("single line = %q, want no stray commas for empty parts", got)
	}
}

// ---------------------------------------------------------------- defaults

func addressList(t *testing.T, defaults ...bool) []domain.Address {
	t.Helper()
	out := make([]domain.Address, 0, len(defaults))
	for i, isDefault := range defaults {
		address, err := newAddress(t, func(a *addressArgs) { a.id = string(rune('a' + i)) })
		if err != nil {
			t.Fatalf("NewAddress: %v", err)
		}
		address.IsDefault = isDefault
		out = append(out, address)
	}
	return out
}

func defaultCount(addresses []domain.Address) int {
	n := 0
	for _, a := range addresses {
		if a.IsDefault {
			n++
		}
	}
	return n
}

// A user who has any addresses has exactly one default.
func TestExactlyOneAddressIsAlwaysTheDefault(t *testing.T) {
	cases := map[string][]domain.Address{
		"none marked":    addressList(t, false, false, false),
		"one marked":     addressList(t, false, true, false),
		"several marked": addressList(t, true, true, true),
		"single address": addressList(t, false),
	}
	for name, addresses := range cases {
		got := domain.ChooseDefault(addresses)
		if defaultCount(got) != 1 {
			t.Errorf("%s: %d defaults, want exactly 1", name, defaultCount(got))
		}
	}
}

func TestAnExplicitDefaultIsKept(t *testing.T) {
	addresses := addressList(t, false, true, false)
	got := domain.ChooseDefault(addresses)
	if !got[1].IsDefault {
		t.Errorf("the marked address is not the default: %+v", got)
	}
}

// The newest remaining address inherits: someone who deletes their default is
// usually moving on from it, and the most recent is the best guess at where
// they are now.
func TestTheNewestAddressInheritsTheDefault(t *testing.T) {
	addresses := addressList(t, false, false, false)
	got := domain.ChooseDefault(addresses)
	if !got[len(got)-1].IsDefault {
		t.Errorf("the newest address did not inherit: %+v", got)
	}
}

// With several marked, the first wins — deterministically, so two concurrent
// writers converge rather than flapping.
func TestSeveralMarkedDefaultsResolveToTheFirst(t *testing.T) {
	got := domain.ChooseDefault(addressList(t, true, true, true))
	if !got[0].IsDefault || got[1].IsDefault || got[2].IsDefault {
		t.Errorf("defaults = %v, %v, %v", got[0].IsDefault, got[1].IsDefault, got[2].IsDefault)
	}
}

func TestChoosingADefaultFromNothingIsSafe(t *testing.T) {
	if got := domain.ChooseDefault(nil); len(got) != 0 {
		t.Errorf("got %d addresses from an empty list", len(got))
	}
}

func TestChoosingADefaultDoesNotMutateTheInput(t *testing.T) {
	addresses := addressList(t, true, false)
	_ = domain.ChooseDefault(addresses)
	if !addresses[0].IsDefault {
		t.Error("ChooseDefault mutated the caller's slice")
	}
}
