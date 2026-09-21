package identity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/infrastructure/token"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// Service is what every other module sees. It is deliberately small: other
// modules need to know who is calling and to revoke access, not to issue
// tokens — an interface that let them would put the ability to mint credentials
// in every module in the system.

func newIdentityService(t *testing.T, sessions *fakeSessionStore) (contract.IdentityContract, *token.Signer) {
	t.Helper()
	service, signer, _ := newIdentityServiceWithDirectory(t, sessions)
	return service, signer
}

func newIdentityServiceWithDirectory(t *testing.T, sessions *fakeSessionStore) (contract.IdentityContract, *token.Signer, *fakeDirectory) {
	t.Helper()
	signer, err := token.NewSigner(
		[]byte("a-service-test-signing-key-32-bytes-x"), "goklay-test",
		token.WithClock(func() time.Time { return signInAt }))
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	directory := newDirectory()
	return application.NewService(signer, sessions, directory), signer, directory
}

func TestTheContractVerifiesATokenAndReportsTheCaller(t *testing.T) {
	service, signer := newIdentityService(t, newSessionStore())

	signed, err := signer.Sign(domain.Claims{
		UserID: "usr_1", Role: domain.RoleMerchant, SessionID: "ses_1",
		TokenID: "jti_1", IssuedAt: signInAt, ExpiresAt: signInAt.Add(15 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	principal, err := service.PrincipalFromToken(context.Background(), signed)
	if err != nil {
		t.Fatalf("PrincipalFromToken: %v", err)
	}
	if principal.UserID != "usr_1" || principal.Role != contract.RoleMerchant || principal.SessionID != "ses_1" {
		t.Errorf("principal = %+v", principal)
	}
	if principal.IsZero() {
		t.Error("a verified principal reported itself as empty")
	}
}

func TestTheContractRefusesABadToken(t *testing.T) {
	service, _ := newIdentityService(t, newSessionStore())

	for _, bad := range []string{"", "not-a-token", "a.b.c"} {
		principal, err := service.PrincipalFromToken(context.Background(), bad)
		if errs.KindOf(err) != errs.KindUnauthorized {
			t.Errorf("token %q: error = %v, want unauthorized", bad, err)
		}
		if !principal.IsZero() {
			t.Errorf("token %q returned a principal: %+v", bad, principal)
		}
	}
}

// TestRevokingAUsersSessionsTakesEffectAtOnce is what a suspension or a
// delisting needs: losing permission must not wait for the last token to
// expire.
func TestRevokingAUsersSessionsTakesEffectAtOnce(t *testing.T) {
	sessions := newSessionStore()
	service, _ := newIdentityService(t, sessions)

	session := domain.Session{
		ID: "ses_1", UserID: "usr_1", Role: domain.RoleMerchant,
		CreatedAt: signInAt, LastSeenAt: signInAt, ExpiresAt: signInAt.Add(time.Hour),
	}
	refresh, err := domain.GenerateRefreshToken(&countingReader{})
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	if err := sessions.CreateSession(context.Background(), session, refresh.Hash(), time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := service.RevokeUserSessions(context.Background(), "usr_1"); err != nil {
		t.Fatalf("RevokeUserSessions: %v", err)
	}
	if _, err := sessions.Session(context.Background(), "ses_1"); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Errorf("the session survived revocation: %v", err)
	}
}

func TestTheContractSurfacesAStoreFailure(t *testing.T) {
	sessions := newSessionStore()
	sessions.revokeErr = errStore
	service, _ := newIdentityService(t, sessions)

	if err := service.RevokeUserSessions(context.Background(), "usr_1"); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("error = %v, want unavailable", err)
	}
}

// TestPhoneForOverTheContract is notification's own lookup (P14): the number
// behind an account id, so an undelivered push has somewhere else to go.
func TestPhoneForOverTheContract(t *testing.T) {
	ctx := context.Background()
	service, _, directory := newIdentityServiceWithDirectory(t, newSessionStore())

	phone, err := domain.NewPhone("+8801711000001")
	if err != nil {
		t.Fatalf("NewPhone: %v", err)
	}
	userID, _, err := directory.EnsureUser(ctx, phone, domain.RoleCustomer)
	if err != nil {
		t.Fatalf("EnsureUser: %v", err)
	}

	got, found, err := service.PhoneFor(ctx, userID)
	if err != nil {
		t.Fatalf("PhoneFor: %v", err)
	}
	if !found || got != phone.String() {
		t.Errorf("PhoneFor = %q, %v, want %q, true", got, found, phone.String())
	}
}

func TestPhoneForAnUnknownAccountIsNotFound(t *testing.T) {
	service, _, _ := newIdentityServiceWithDirectory(t, newSessionStore())
	_, found, err := service.PhoneFor(context.Background(), "usr_missing")
	if err != nil || found {
		t.Errorf("PhoneFor(missing) = found=%v, err=%v", found, err)
	}
}

func TestPhoneForSurfacesAStoreFailure(t *testing.T) {
	service, _, directory := newIdentityServiceWithDirectory(t, newSessionStore())
	directory.phoneErr = errStore

	if _, _, err := service.PhoneFor(context.Background(), "usr_1"); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("error = %v, want unavailable", err)
	}
}

func TestTheZeroPrincipalIsRecognisable(t *testing.T) {
	if !(contract.Principal{}).IsZero() {
		t.Error("the zero Principal does not report itself as empty")
	}
	if (contract.Principal{UserID: "usr_1"}).IsZero() {
		t.Error("a principal with a user id reported itself as empty")
	}
}

// A device label is displayed in the active-devices list and stored per
// session, so an unbounded string is both a storage cost and a way to make that
// screen unreadable.
func TestAnOverlongDeviceLabelIsTruncated(t *testing.T) {
	r := newRig()
	if _, err := r.request.Execute(ctx(), testPhone, noPlacement); err != nil {
		t.Fatalf("request: %v", err)
	}
	huge := ""
	for i := 0; i < 500; i++ {
		huge += "x"
	}
	result, err := r.verify.Execute(ctx(), application.VerifyRequest{
		Phone: testPhone, Code: r.sms.codes[0], Role: domain.RoleCustomer, Device: huge,
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	session, err := r.sessions.Session(ctx(), result.Tokens.SessionID)
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	if len(session.Device) != 64 {
		t.Errorf("device label is %d characters, want it capped at 64", len(session.Device))
	}
}

func TestGeneratingARefreshTokenSurfacesAnEntropyFailure(t *testing.T) {
	if _, err := domain.GenerateRefreshToken(emptyReader{}); err == nil {
		t.Error("GenerateRefreshToken must fail rather than return a short token")
	}
}

type emptyReader struct{}

func (emptyReader) Read([]byte) (int, error) { return 0, errors.New("no entropy") }
