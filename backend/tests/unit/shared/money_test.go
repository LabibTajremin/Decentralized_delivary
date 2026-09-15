package shared

import (
	"errors"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// Money is the type every price, fee and total in the product is built from.
// The properties that matter are that it never loses a poisha, never silently
// mixes currencies, and formats exactly one way — because the server owns
// formatting (2.9) and two spellings of a price is a customer comparing a
// screen with a receipt and seeing two answers.

func TestAnAmountIsExactlyWhatWasPutIn(t *testing.T) {
	amount := money.Taka(124_000)
	if amount.Minor() != 124_000 {
		t.Errorf("Minor() = %d", amount.Minor())
	}
	if amount.Currency() != money.BDT {
		t.Errorf("Currency() = %q", amount.Currency())
	}
}

// TestTheZeroValueIsAUsableZero: an uninitialised Money must not be a currency
// mismatch waiting to happen, because struct fields default to it everywhere.
func TestTheZeroValueIsAUsableZero(t *testing.T) {
	var uninitialised money.Money

	if !uninitialised.IsZero() {
		t.Error("the zero value is not zero")
	}
	if uninitialised.Currency() != money.BDT {
		t.Errorf("Currency() = %q, want the default", uninitialised.Currency())
	}

	sum, err := uninitialised.Add(money.Taka(500))
	if err != nil {
		t.Fatalf("adding to the zero value: %v", err)
	}
	if sum.Minor() != 500 {
		t.Errorf("sum = %d", sum.Minor())
	}
}

func TestOnlyCurrenciesWeHandleAreAccepted(t *testing.T) {
	if _, err := money.New(100, money.BDT); err != nil {
		t.Errorf("BDT was refused: %v", err)
	}
	if _, err := money.New(100, "USD"); !errors.Is(err, money.ErrUnknownCurrency) {
		t.Errorf("error = %v, want ErrUnknownCurrency", err)
	}
}

// TestArithmeticRefusesToMixCurrencies. With one currency in the system today
// this is a guard for tomorrow, and it is cheaper to have it before the second
// currency than to find every call site afterwards.
func TestArithmeticRefusesToMixCurrencies(t *testing.T) {
	taka := money.Taka(100)
	other := money.Zero("USD")

	if _, err := taka.Add(other); !errors.Is(err, money.ErrCurrencyMismatch) {
		t.Errorf("Add: error = %v, want ErrCurrencyMismatch", err)
	}
	if _, err := taka.Sub(other); !errors.Is(err, money.ErrCurrencyMismatch) {
		t.Errorf("Sub: error = %v, want ErrCurrencyMismatch", err)
	}
}

func TestAddingAndSubtracting(t *testing.T) {
	sum, err := money.Taka(1_500).Add(money.Taka(250))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if sum.Minor() != 1_750 {
		t.Errorf("sum = %d", sum.Minor())
	}

	difference, err := money.Taka(1_500).Sub(money.Taka(250))
	if err != nil {
		t.Fatalf("Sub: %v", err)
	}
	if difference.Minor() != 1_250 {
		t.Errorf("difference = %d", difference.Minor())
	}
}

// TestSubtractingPastZeroIsAllowed: a saving, a refund and a discount are all
// differences that can go negative, and refusing here would push the sign
// handling into every caller.
func TestSubtractingPastZeroIsAllowed(t *testing.T) {
	difference, err := money.Taka(100).Sub(money.Taka(250))
	if err != nil {
		t.Fatalf("Sub: %v", err)
	}
	if !difference.IsNegative() || difference.Minor() != -150 {
		t.Errorf("difference = %d", difference.Minor())
	}
}

func TestTimesRepeatsALine(t *testing.T) {
	if got := money.Taka(120).Times(3); got.Minor() != 360 {
		t.Errorf("Times(3) = %d", got.Minor())
	}
	if got := money.Taka(120).Times(0); !got.IsZero() {
		t.Errorf("Times(0) = %d, want zero", got.Minor())
	}
}

func TestANonNegativeAmountCanBeDemanded(t *testing.T) {
	if _, err := money.Taka(100).MustBeNonNegative(); err != nil {
		t.Errorf("a positive amount was refused: %v", err)
	}
	if _, err := money.Taka(0).MustBeNonNegative(); err != nil {
		t.Errorf("zero was refused: %v", err)
	}
	if _, err := money.Taka(-1).MustBeNonNegative(); !errors.Is(err, money.ErrNegative) {
		t.Errorf("error = %v, want ErrNegative", err)
	}
}

func TestComparingAmounts(t *testing.T) {
	cases := []struct {
		a, b int64
		want int
	}{
		{100, 200, -1},
		{200, 100, 1},
		{100, 100, 0},
		{-100, 100, -1},
	}
	for _, tc := range cases {
		if got := money.Taka(tc.a).Compare(money.Taka(tc.b)); got != tc.want {
			t.Errorf("Compare(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

// TestTheDisplayStringIsTheOneSpellingOfAPrice. The server formats money
// because the client must not (2.9), so these strings are the product's actual
// output and worth pinning exactly.
func TestTheDisplayStringIsTheOneSpellingOfAPrice(t *testing.T) {
	cases := map[int64]string{
		0:          "৳ 0",
		5:          "৳ 0.05",
		50:         "৳ 0.50",
		99:         "৳ 0.99",
		100:        "৳ 1",
		12_000:     "৳ 120",
		124_000:    "৳ 1,240",
		124_050:    "৳ 1,240.50",
		100_000_00: "৳ 100,000",
		1_234_567:  "৳ 12,345.67",
		-25_000:    "-৳ 250",
		-5:         "-৳ 0.05",
	}

	for minor, want := range cases {
		if got := money.Taka(minor).Display(); got != want {
			t.Errorf("Taka(%d).Display() = %q, want %q", minor, got, want)
		}
	}
}

// TestStringIsTheSameAsDisplay: there is no second spelling of an amount, so a
// log line and a receipt agree.
func TestStringIsTheSameAsDisplay(t *testing.T) {
	amount := money.Taka(124_050)
	if amount.String() != amount.Display() {
		t.Errorf("String() = %q, Display() = %q", amount.String(), amount.Display())
	}
}

// TestGroupingHandlesEveryDigitCount walks the boundaries where a separator is
// added, which is where an off-by-one in the grouping would show.
func TestGroupingHandlesEveryDigitCount(t *testing.T) {
	cases := map[int64]string{
		100:           "৳ 1",
		1_000:         "৳ 10",
		10_000:        "৳ 100",
		100_000:       "৳ 1,000",
		1_000_000:     "৳ 10,000",
		10_000_000:    "৳ 100,000",
		100_000_000:   "৳ 1,000,000",
		1_000_000_000: "৳ 10,000,000",
	}
	for minor, want := range cases {
		if got := money.Taka(minor).Display(); got != want {
			t.Errorf("Taka(%d).Display() = %q, want %q", minor, got, want)
		}
	}
}

func TestZeroBuildsAnAmountInACurrency(t *testing.T) {
	zero := money.Zero(money.BDT)
	if !zero.IsZero() || zero.Currency() != money.BDT {
		t.Errorf("Zero(BDT) = %+v", zero)
	}
}
