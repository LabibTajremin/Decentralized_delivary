package application

// statusLabel composes an order's status for a screen, in the caller's
// language (2.9, 1.4 Bengali-first).
//
// Duplicated from order's own statusLabel rather than imported: 2.5 lets
// tracking depend on order's contract, not its application package, and a
// contract carries the raw status primitive rather than a pre-composed
// label — see order/contract's own Order.Status. Every module that shows an
// order's status composes it in its own vocabulary for the same reason
// dispatch and payment each compose their own; the wording here is kept
// identical to order's on purpose, not by a shared dependency.
func statusLabel(status, lang string) string {
	bengali := lang != "en"
	switch status {
	case "pending_payment":
		if bengali {
			return "পেমেন্টের অপেক্ষায়"
		}
		return "Waiting for payment"
	case "placed":
		if bengali {
			return "দোকানে পাঠানো হয়েছে"
		}
		return "Sent to the shop"
	case "accepted":
		if bengali {
			return "দোকান অর্ডার নিয়েছে"
		}
		return "The shop has accepted"
	case "preparing":
		if bengali {
			return "তৈরি হচ্ছে"
		}
		return "Being prepared"
	case "ready":
		if bengali {
			return "রাইডারের অপেক্ষায়"
		}
		return "Waiting for a rider"
	case "picked_up":
		if bengali {
			return "পথে আছে"
		}
		return "On the way"
	case "delivered":
		if bengali {
			return "পৌঁছে দেওয়া হয়েছে"
		}
		return "Delivered"
	case "cancelled":
		if bengali {
			return "বাতিল করা হয়েছে"
		}
		return "Cancelled"
	case "rejected":
		if bengali {
			return "দোকান নিতে পারেনি"
		}
		return "The shop could not take it"
	default:
		if bengali {
			return "পৌঁছে দেওয়া যায়নি"
		}
		return "Could not be delivered"
	}
}
