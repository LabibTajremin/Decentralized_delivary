# Tracking

> P14 (half of it — see also `notification.md`). A live view of one
> delivery: the order's own status, and while a rider is carrying it, where
> they are. Depends on P11 (order) and P12 (dispatch).

## No storage of its own

Order already has the status and the history (`OrderContract.Order` —
its own doc comment says as much: "Tracking needs the status and the
history"). Dispatch already has where the rider is
(`DispatchContract.JobForOrder`, whose `Partner` carries the last reported
location). A third copy in a tracking table would only be a second place
for the two to disagree, so this module owns nothing — `infrastructure/`
does not exist, the same as discovery's (P08).

Both seams are bare interfaces with no adapter, structurally satisfied by
`orderapp.Service` and `dispatchapp.Service` directly — the target's own
method already returns exactly what tracking needs, the pattern
established in P13 (`payment/external/dispatch`) and applied wherever a
target's method signature already matches what a consumer needs.

## The stream

`GET /v1/track/{orderId}` is Server-Sent Events, not a poll a client
repeats — one frame on connect, then one more each time something worth
showing changes, until the order reaches a status with nothing left to
watch. `domain.Snapshot.Changed` decides "worth showing": the status
moved, a rider appeared or left, or the rider they're already watching
moved. A tick where nothing changed sends nothing, because a stream that
redrew a screen on every poll interval regardless of whether anything
moved would be a battery cost with no information in it.

`TRACKING_STREAM_INTERVAL` (default 3s, `.env.example`) is a deploy-time
knob, not an Appendix B business rule — it trades server load against how
quickly a screen updates, not anything a division's operations team would
tune.

**The first frame answers the ownership question the same way a JSON
endpoint would.** A caller who may not watch this order gets a 404 before
anything is upgraded to a stream — never a stream that opens and then
refuses partway through. Only the order's own customer, or the partner
currently assigned to it (`DispatchContract.PartnerOfUser`, the same
lookup P13's COD ledger uses), may open one; anyone else is refused as
not-found, the same non-disclosure every other caller-scoped read in this
system uses, so a stream cannot be used to probe whether an order id
exists.

`SnapshotUseCase` carries both the scoped path (`For`, used by the
transport) and an unscoped one (`Unscoped`, used only by
`TrackingContract`) — the ownership check belongs to the boundary an end
user actually reaches, not to a method a trusted in-process caller uses.

## A real bug the httpx middleware chain hid

`httpx.Logging` wraps every response in `statusRecorder`, which embeds
`http.ResponseWriter` as an **interface field**. Go's method promotion
through an embedded interface promotes only that interface's own methods —
`Header`, `Write`, `WriteHeader` — never `Flush`, which isn't part of
`http.ResponseWriter` at all. Before this phase, nothing in the system had
ever needed to flush a partial response, so nobody had hit it: a stream's
`w.(http.Flusher)` type assertion would have failed the moment `Logging`
sat in front of the handler, in every environment, always — not a race,
not an edge case, simply broken. `statusRecorder` now has its own `Flush`
that forwards to the underlying writer when it can, with a no-op fallback
otherwise, covered directly in `backend/tests/unit/httpx` rather than only
being provable through an SSE test elsewhere.

## Everything a screen shows is composed here

Status labels are composed server-side, Bengali-first (1.4) with
`?lang=en`, mirroring order's own wording exactly (`application/view.go`'s
`statusLabel`) — duplicated rather than imported, the same as every other
module that shows an order's status, since 2.5 lets tracking depend on
order's *contract*, which carries the raw status primitive, not a
pre-composed label.

## What tracking does not do

It never writes to an order or a job — `SnapshotUseCase` has no write path
at all. An outage in either seam is a failure the stream simply ends on
(`if err != nil { return }` inside the poll loop) rather than a retry
storm; the client's own reconnect is the recovery, the same as any other
SSE consumer.
