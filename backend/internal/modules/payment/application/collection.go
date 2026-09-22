package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/domain"
	dispatchx "github.com/rootlogic-lab/delivery/backend/internal/modules/payment/external/dispatch"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// CollectionUseCase is the COD ledger: what a partner is holding, and giving
// it back to the platform.
type CollectionUseCase struct {
	repo     ports.Repository
	dispatch dispatchx.Service
	clock    clock.Clock
	ids      id.Generator
}

// NewCollectionUseCase wires the use case.
func NewCollectionUseCase(repo ports.Repository, dispatch dispatchx.Service, c clock.Clock, ids id.Generator) *CollectionUseCase {
	return &CollectionUseCase{repo: repo, dispatch: dispatch, clock: c, ids: ids}
}

// Record is order's delivery hook: a rider has just handed over a COD order,
// which means they are now carrying its cash.
//
// Idempotent on the order id. The transition that calls this only ever fires
// once in the ordinary run of things — the order's own compare-and-set refuses
// a second `delivered` — but a crash between that write and this call is
// exactly the kind of gap a retry exists to close, and a retry must not book
// the same cash twice.
func (uc *CollectionUseCase) Record(ctx context.Context, orderID, partnerID string, amountMinor int64) error {
	now := uc.clock.Now()
	collection, err := domain.NewCollection(uc.ids.New("COL"), orderID, partnerID, money.Taka(amountMinor), now)
	if err != nil {
		return collectionError(err)
	}
	if err := uc.repo.CreateCollection(ctx, collection); err != nil {
		if errs.CodeOf(err) == "collection_exists" {
			// Already recorded. The ledger is not short a row; the retry just
			// arrived after the original.
			return nil
		}
		return storageError(err)
	}
	return nil
}

// MyLedgerForUser is a signed-in rider asking "what am I carrying" — resolved
// from their own account rather than trusting a partner id from the request,
// so nobody can read another rider's cash position by guessing an id.
func (uc *CollectionUseCase) MyLedgerForUser(ctx context.Context, userID, lang string) (LedgerView, error) {
	partnerID, found, err := uc.dispatch.PartnerOfUser(ctx, userID)
	if err != nil {
		return LedgerView{}, storageError(err)
	}
	if !found {
		return LedgerView{}, notFound()
	}
	return uc.MyLedger(ctx, partnerID, lang)
}

// MyLedger is a partner's own cash position.
func (uc *CollectionUseCase) MyLedger(ctx context.Context, partnerID, lang string) (LedgerView, error) {
	collections, err := uc.repo.ForPartner(ctx, partnerID)
	if err != nil {
		return LedgerView{}, storageError(err)
	}
	return ledgerViewOf(domain.Summarize(partnerID, collections), lang), nil
}

// Reconcile marks a partner's held cash as remitted.
//
// Every id in the batch either belongs to this partner and is still held, or
// none of them move — the repository does this inside one transaction, so a
// partial remittance can never leave the ledger claiming half a handover
// happened when an operator meant all of it.
func (uc *CollectionUseCase) Reconcile(ctx context.Context, partnerID string, collectionIDs []string, reference, lang string) (LedgerView, error) {
	if len(collectionIDs) == 0 {
		return LedgerView{}, errs.New(errs.KindInvalid, "nothing_to_reconcile",
			"Choose at least one collection to remit.")
	}
	if reference == "" {
		return LedgerView{}, collectionError(domain.ErrNoRemittanceRef)
	}

	if err := uc.repo.Remit(ctx, partnerID, collectionIDs, reference, uc.clock.Now()); err != nil {
		if errs.KindOf(err) == errs.KindConflict {
			return LedgerView{}, err
		}
		return LedgerView{}, storageError(err)
	}
	return uc.MyLedger(ctx, partnerID, lang)
}
