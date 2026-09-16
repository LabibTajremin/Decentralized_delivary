package domain

import "sort"

// ALG-08 — the partner's job feed. A bounded priority queue over the jobs
// inside their radius, refreshed whenever they report a new location.
//
// The bound is the point as much as the ordering. A partner on a low-end phone
// in a busy area could otherwise be handed three hundred jobs, which is a list
// nobody scrolls and a payload nobody on 2G downloads (2.9).

// FeedLimit is how many jobs a partner is shown at once.
//
// Twenty is a screen and a half. A partner who works through them gets a fresh
// list; one who does not was never going to reach number two hundred.
const FeedLimit = 20

// FeedEntry is one job as the partner's list shows it.
type FeedEntry struct {
	Job Job
	// DistanceToPickupM is how far the partner is from the counter. What the
	// ordering turns on, and what the rider actually cares about.
	DistanceToPickupM float64
}

// BuildFeed orders the jobs a partner should see, best first, bounded.
//
// The caller has already narrowed to jobs inside the partner's radius — that is
// a spatial query and belongs to geo. What happens here is D4's preference
// filter and the ordering. O(n log n) on what came back, which is bounded by
// the radius query's own limit.
// The ordering is by distance to the pickup, not by the assignment score. The
// assignment score exists to choose between *partners* for one job; inside one
// partner's feed its load and acceptance-rate terms are the same on every
// entry, so reusing it would be an elaborate way of sorting by distance.
func BuildFeed(partner Partner, entries []FeedEntry) []FeedEntry {
	kept := make([]FeedEntry, 0, len(entries))
	for _, entry := range entries {
		// D4: a partner is shown only the distances they chose.
		if !partner.Preference.Accepts(entry.Job.Band) {
			continue
		}
		// A job already in somebody's hands is not on offer, and one this
		// partner has been offered is on their current-job screen instead.
		if entry.Job.Status != JobWaiting {
			continue
		}
		kept = append(kept, entry)
	}

	// Ties break on the job id so a partner refreshing twice sees the same
	// list in the same order, rather than two jobs swapping places under their
	// thumb.
	sort.SliceStable(kept, func(i, j int) bool {
		if kept[i].DistanceToPickupM != kept[j].DistanceToPickupM {
			return kept[i].DistanceToPickupM < kept[j].DistanceToPickupM
		}
		return kept[i].Job.ID < kept[j].Job.ID
	})

	if len(kept) > FeedLimit {
		kept = kept[:FeedLimit]
	}
	return kept
}
