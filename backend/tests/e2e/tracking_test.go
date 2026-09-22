package e2e

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// startTrackingAPI is startAuthAPI with a short poll interval, so a test
// watching a live stream does not have to wait out the 3s production
// default to see an update.
func startTrackingAPI(t *testing.T) (string, *logTail, func()) {
	t.Helper()
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Fatal("REDIS_URL is not set; the tracking tests need a real Redis")
	}
	dbURL := dbtestSchemaURL(t)
	seedDemoData(t, dbURL)

	tail := newLogTail(t)
	base, stop := startAPIWithOutput(t, buildAPI(t), tail,
		"DATABASE_URL="+dbURL, "REDIS_URL="+redisURL, "TRACKING_STREAM_INTERVAL=50ms")
	return base, tail, stop
}

type trackingFrame struct {
	OrderID     string `json:"order_id"`
	Status      string `json:"status"`
	StatusLabel string `json:"status_label"`
	Live        bool   `json:"live"`
	Partner     *struct {
		ID      string  `json:"id"`
		Name    string  `json:"name"`
		Phone   string  `json:"phone"`
		Vehicle string  `json:"vehicle"`
		Lat     float64 `json:"lat"`
		Lng     float64 `json:"lng"`
	} `json:"partner"`
}

// streamFrames opens the tracking stream and reads up to n "data:" frames,
// or stops early once the stream itself closes (a terminal order sends one
// frame and ends the connection).
func streamFrames(t *testing.T, base, orderID, bearer string, n int) []trackingFrame {
	t.Helper()
	return streamFramesThen(t, base, orderID, bearer, n, nil)
}

// streamFramesThen is streamFrames with a hook that fires once the first frame
// has actually been read.
//
// It exists because the interesting assertion about this stream is that it
// notices a change *mid-connection*, and proving that means changing the order
// while the stream is open. Sleeping for a while and hoping the stream got
// there first is how that test was written, and it is a test that fails on a
// loaded machine for reasons that have nothing to do with the product: if the
// change lands before the first frame, the stream opens on an order that is
// already terminal, sends one frame and closes.
//
// The hook removes the clock from it. The caller is told when the stream is
// demonstrably reading, and only then makes its change.
func streamFramesThen(
	t *testing.T, base, orderID, bearer string, n int, afterFirst func(),
) []trackingFrame {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/v1/track/"+orderID+"?lang=en", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+bearer)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("stream request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stream status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}

	var out []trackingFrame
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() && len(out) < n {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var frame trackingFrame
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &frame); err != nil {
			t.Fatalf("decode frame %q: %v", line, err)
		}
		out = append(out, frame)
		if len(out) == 1 && afterFirst != nil {
			// In a goroutine: the hook's whole purpose is to happen while
			// this loop is still reading, and calling it inline would stop
			// the reading it is meant to race.
			go afterFirst()
		}
	}
	return out
}

// The stream a customer opens while their delivery is on the road shows the
// rider carrying it, real position included, and closes on its own once the
// order is delivered — proven against the real binary, real Postgres, and a
// stream reading concurrently with the delivery actually happening.
func TestACustomerWatchesTheirDeliveryLive(t *testing.T) {
	base, tail, stop := startTrackingAPI(t)
	defer stop()

	customer, owner, shopID, addressID := readyToOrder(t, base, tail)
	rider, _ := onShift(t, base, tail, "Karim", dhanmondiLat, dhanmondiLng)

	orderID := placeOrder(t, base, customer, addressID)
	walkShopToReady(t, base, owner, shopID, orderID)

	var mine jobListPayload
	if status := requestAs(t, http.MethodGet, base+"/v1/partner/jobs?live=true", "",
		rider.AccessToken, &mine); status != http.StatusOK || mine.Total != 1 {
		t.Fatalf("jobs: status = %d, mine = %+v", status, mine)
	}
	job := mine.Jobs[0]

	for _, step := range []string{"/accept", "/collect"} {
		if status := requestAs(t, http.MethodPost, base+"/v1/partner/jobs/"+job.ID+step, "",
			rider.AccessToken, nil); status != http.StatusOK {
			t.Fatalf("%s: status = %d", step, status)
		}
	}

	// The delivery finishes concurrently with the stream reading it, so the
	// test proves the stream notices a change mid-connection rather than
	// only ever seeing a snapshot taken before it opened.
	//
	// It is triggered by the stream's first frame rather than by a timer.
	// With a timer this raced: on a loaded machine the delivery could land
	// before the stream had read anything, the stream would then open on an
	// order that was already terminal, and the test would fail having proven
	// nothing about the product.
	done := make(chan struct{})
	deliver := func() {
		defer close(done)
		if status := requestAs(t, http.MethodPost, base+"/v1/partner/jobs/"+job.ID+"/deliver", "",
			rider.AccessToken, nil); status != http.StatusOK {
			t.Errorf("deliver: status = %d", status)
		}
	}

	frames := streamFramesThen(t, base, orderID, customer.AccessToken, 2, deliver)
	<-done

	if len(frames) < 2 {
		t.Fatalf("frames = %+v, want the live snapshot and the delivered one", frames)
	}
	first := frames[0]
	if first.Status != "picked_up" || !first.Live {
		t.Fatalf("first frame = %+v", first)
	}
	if first.Partner == nil || first.Partner.ID == "" || first.Partner.Name != "Karim" {
		t.Fatalf("first frame carries no rider: %+v", first)
	}
	if first.Partner.Lat != dhanmondiLat || first.Partner.Lng != dhanmondiLng {
		t.Fatalf("partner location = %+v, want %f,%f", first.Partner, dhanmondiLat, dhanmondiLng)
	}

	last := frames[len(frames)-1]
	if last.Status != "delivered" || last.Live {
		t.Fatalf("last frame = %+v, want a terminal delivered frame", last)
	}
}

// A stranger cannot watch somebody else's delivery — the stream answers 404,
// not a body full of another customer's rider.
func TestAStrangerCannotWatchSomeoneElsesDelivery(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	customer, owner, shopID, addressID := readyToOrder(t, base, tail)
	orderID := placeOrder(t, base, customer, addressID)
	walkShopToReady(t, base, owner, shopID, orderID)

	stranger := signInAs(t, base, tail, uniquePhone(t), "stranger's phone", "customer")

	req, err := http.NewRequest(http.MethodGet, base+"/v1/track/"+orderID, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+stranger.AccessToken)
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// The customer is told when their delivery is picked up and delivered, and
// can read both back from their own notification history — over the real
// binary, with no device ever registered, so both went out over the SMS
// fallback the log-only sender proves by writing to the server's own log.
func TestTheCustomerIsNotifiedAndCanReadItBack(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	customer, owner, shopID, addressID := readyToOrder(t, base, tail)
	rider, _ := onShift(t, base, tail, "Rafi", dhanmondiLat, dhanmondiLng)

	orderID := placeOrder(t, base, customer, addressID)
	walkShopToReady(t, base, owner, shopID, orderID)

	var mine jobListPayload
	if status := requestAs(t, http.MethodGet, base+"/v1/partner/jobs?live=true", "",
		rider.AccessToken, &mine); status != http.StatusOK || mine.Total != 1 {
		t.Fatalf("jobs: status = %d, mine = %+v", status, mine)
	}
	job := mine.Jobs[0]

	for _, step := range []string{"/accept", "/collect", "/deliver"} {
		if status := requestAs(t, http.MethodPost, base+"/v1/partner/jobs/"+job.ID+step, "",
			rider.AccessToken, nil); status != http.StatusOK {
			t.Fatalf("%s: status = %d", step, status)
		}
	}

	var order orderResponse
	if status := requestAs(t, http.MethodGet, base+"/v1/orders/"+orderID, "",
		customer.AccessToken, &order); status != http.StatusOK {
		t.Fatalf("read order: status = %d", status)
	}

	var history []struct {
		Title   string `json:"title"`
		Body    string `json:"body"`
		Channel string `json:"channel"`
		Status  string `json:"status"`
	}
	if status := requestAs(t, http.MethodGet, base+"/v1/me/notifications", "",
		customer.AccessToken, &history); status != http.StatusOK {
		t.Fatalf("notifications: status = %d", status)
	}
	if len(history) != 2 {
		t.Fatalf("history = %+v, want the pickup and delivery notices", history)
	}
	for _, n := range history {
		if n.Status != "sent" || n.Channel != "sms" {
			t.Fatalf("notification = %+v, want an SMS fallback send since no device was ever registered", n)
		}
		if !strings.Contains(n.Body, order.Code) {
			t.Fatalf("notification body %q does not name the order's own code %q", n.Body, order.Code)
		}
	}

	// The SMS fallback is the log-only sender outside production — the same
	// log a developer reads an OTP from — so the message the customer would
	// have received is provably the one this test just read back. Polled,
	// like lastCode: the child process's stdout reaches this buffer through
	// an OS pipe, a moment after the HTTP response that triggered it.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if strings.Contains(tail.snapshot(), order.Code) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the SMS fallback was not written to the log within 5s; server output was:\n%s", tail.snapshot())
		}
		time.Sleep(20 * time.Millisecond)
	}
}
