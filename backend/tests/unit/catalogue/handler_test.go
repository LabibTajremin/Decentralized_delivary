package catalogue

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/domain"
	cathttp "github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/transport/http"
)

// The owner routes carry a merchant id in the path, so ownership is verified
// against the merchant record on every one. The customer routes are public
// because browsing a menu is what the app does before anyone signs in — and
// they must not leak a shelf count while doing it.

type rig struct {
	*harness
	mux       *http.ServeMux
	principal string
}

func newRig(t *testing.T, kind domain.MerchantType) *rig {
	t.Helper()
	r := &rig{harness: newHarness(t, kind), principal: ownerID}

	mux := http.NewServeMux()
	cathttp.NewHandler(
		r.categories, r.items, r.options, r.combos, r.service,
		func(next http.Handler) http.Handler { return next },
		func(*http.Request) (string, bool) { return r.principal, r.principal != "" },
	).Register(mux)
	r.mux = mux
	return r
}

func (r *rig) do(method, target, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.mux.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, rec.Body)
	}
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeBody(t, rec, &body)
	return body.Error.Code
}

const base = "/v1/merchants/" + shopID + "/catalogue"

// createCategory adds a section through the API and returns its id.
func (r *rig) createCategory(t *testing.T, name string) string {
	t.Helper()
	rec := r.do(http.MethodPost, base+"/categories", `{"name":"`+name+`"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create category: status = %d, body = %s", rec.Code, rec.Body)
	}
	var body struct {
		ID string `json:"id"`
	}
	decodeBody(t, rec, &body)
	return body.ID
}

// createItem adds an item through the API and returns its id.
func (r *rig) createItem(t *testing.T, categoryID, payload string) string {
	t.Helper()
	rec := r.do(http.MethodPost, base+"/items",
		`{"category_id":"`+categoryID+`",`+payload+`}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create item: status = %d, body = %s", rec.Code, rec.Body)
	}
	var body struct {
		ID string `json:"id"`
	}
	decodeBody(t, rec, &body)
	return body.ID
}

// ------------------------------------------------------------------ wiring

// TestTheHandlerRefusesToStartWithoutItsGuards: a nil guard would let any
// caller rewrite any menu, so it fails at wiring time.
func TestTheHandlerRefusesToStartWithoutItsGuards(t *testing.T) {
	pass := func(next http.Handler) http.Handler { return next }
	principal := func(*http.Request) (string, bool) { return "u", true }

	cases := map[string]func(){
		"no guard": func() {
			cathttp.NewHandler(nil, nil, nil, nil, nil, nil, principal)
		},
		"no principal reader": func() {
			cathttp.NewHandler(nil, nil, nil, nil, nil, cathttp.Guard(pass), nil)
		},
	}

	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("the handler was built without its guard")
				}
			}()
			build()
		})
	}
}

func TestPatternsMatchWhatIsMounted(t *testing.T) {
	patterns := cathttp.Patterns()
	if len(patterns) == 0 {
		t.Fatal("the module reports no routes")
	}
	for i := 1; i < len(patterns); i++ {
		if patterns[i-1] >= patterns[i] {
			t.Errorf("Patterns() is not sorted: %q before %q", patterns[i-1], patterns[i])
		}
	}

	r := newRig(t, domain.Restaurant)
	for _, pattern := range patterns {
		method, path, found := strings.Cut(pattern, " ")
		if !found {
			t.Fatalf("pattern %q is not \"METHOD /path\"", pattern)
		}
		for placeholder, value := range map[string]string{
			"{merchantId}": shopID, "{categoryId}": "cat_1",
			"{itemId}": "itm_1", "{comboId}": "cmb_1",
		} {
			path = strings.ReplaceAll(path, placeholder, value)
		}
		if rec := r.do(method, path, `{}`); rec.Code == http.StatusNotFound && rec.Body.Len() == 0 {
			t.Errorf("%s is reported but nothing is mounted at it", pattern)
		}
	}
}

// TestEveryOwnerRouteRefusesACallerWithNoIdentity. The guard is faked open
// here, so this checks the handler's own check.
func TestEveryOwnerRouteRefusesACallerWithNoIdentity(t *testing.T) {
	r := newRig(t, domain.Restaurant)
	r.principal = ""

	requests := [][2]string{
		{http.MethodPost, base + "/categories"},
		{http.MethodPut, base + "/categories/cat_1"},
		{http.MethodPost, base + "/categories/cat_1/active"},
		{http.MethodDelete, base + "/categories/cat_1"},
		{http.MethodPost, base + "/items"},
		{http.MethodPut, base + "/items/itm_1"},
		{http.MethodDelete, base + "/items/itm_1"},
		{http.MethodPost, base + "/items/itm_1/active"},
		{http.MethodPut, base + "/items/itm_1/stock"},
		{http.MethodPut, base + "/items/itm_1/availability"},
		{http.MethodPut, base + "/items/itm_1/variants"},
		{http.MethodPut, base + "/items/itm_1/addons"},
		{http.MethodPost, base + "/items/bulk"},
		{http.MethodPost, base + "/combos"},
		{http.MethodPut, base + "/combos/cmb_1"},
		{http.MethodDelete, base + "/combos/cmb_1"},
		{http.MethodPost, base + "/combos/cmb_1/active"},
		{http.MethodPut, base + "/combos/cmb_1/availability"},
		{http.MethodGet, base + "/capabilities"},
	}

	for _, req := range requests {
		rec := r.do(req[0], req[1], `{}`)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: status = %d, want 401", req[0], req[1], rec.Code)
		}
		if got := errorCode(t, rec); got != "not_authenticated" {
			t.Errorf("%s %s: code = %q", req[0], req[1], got)
		}
	}
}

// TestTheCustomerRoutesNeedNoAccount: requiring a token to read a menu would
// put a sign-up wall in front of the thing customers came for.
func TestTheCustomerRoutesNeedNoAccount(t *testing.T) {
	r := newRig(t, domain.Restaurant)
	categoryID := r.createCategory(t, "Biryani")
	itemID := r.createItem(t, categoryID, `"name":"Kacchi","price_minor":35000`)
	r.principal = ""

	if rec := r.do(http.MethodGet, "/v1/catalogue/"+shopID+"/menu", ""); rec.Code != http.StatusOK {
		t.Errorf("menu: status = %d, want 200", rec.Code)
	}
	if rec := r.do(http.MethodGet, "/v1/catalogue/"+shopID+"/items/"+itemID, ""); rec.Code != http.StatusOK {
		t.Errorf("item: status = %d, want 200", rec.Code)
	}
}

// ------------------------------------------------------- the owner's menu

func TestAnOwnerBuildsAMenuOverHTTP(t *testing.T) {
	r := newRig(t, domain.Restaurant)
	categoryID := r.createCategory(t, "Biryani")

	rec := r.do(http.MethodPost, base+"/items", `{
		"category_id":"`+categoryID+`","name":"Kacchi","description":"Slow cooked.",
		"price_minor":35000,"preparation_minutes":40,"sort_order":1
	}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}

	var item struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Price struct {
			Minor   int64  `json:"minor"`
			Display string `json:"display"`
		} `json:"price"`
		Orderable          bool   `json:"orderable"`
		Active             bool   `json:"active"`
		StockTracked       bool   `json:"stock_tracked"`
		StockQuantity      *int   `json:"stock_quantity"`
		PreparationMinutes int    `json:"preparation_minutes"`
		UnavailableReason  string `json:"unavailable_reason"`
	}
	decodeBody(t, rec, &item)

	if item.Price.Minor != 35_000 || item.Price.Display != "৳ 350" {
		t.Errorf("price = %+v", item.Price)
	}
	if !item.Orderable || !item.Active {
		t.Errorf("a new restaurant dish = orderable %v, active %v", item.Orderable, item.Active)
	}
	if item.StockTracked || item.StockQuantity != nil {
		t.Errorf("a restaurant dish carries a shelf count: tracked %v, quantity %v",
			item.StockTracked, item.StockQuantity)
	}
	if item.PreparationMinutes != 40 {
		t.Errorf("preparation = %d", item.PreparationMinutes)
	}
	if item.UnavailableReason != "" {
		t.Errorf("reason = %q, want none", item.UnavailableReason)
	}
}

// TestAGrocerySeesItsShelfCountAndACustomerDoesNot.
func TestAGrocerySeesItsShelfCountAndACustomerDoesNot(t *testing.T) {
	r := newRig(t, domain.Grocery)
	categoryID := r.createCategory(t, "Rice")
	itemID := r.createItem(t, categoryID, `"name":"Chal","price_minor":7500,"unit":"kg","stock_quantity":40`)

	rec := r.do(http.MethodGet, base+"/items/"+itemID, "")
	var owner struct {
		StockTracked  bool `json:"stock_tracked"`
		StockQuantity *int `json:"stock_quantity"`
	}
	decodeBody(t, rec, &owner)
	if !owner.StockTracked || owner.StockQuantity == nil || *owner.StockQuantity != 40 {
		t.Errorf("the owner's view = %+v", owner)
	}

	rec = r.do(http.MethodGet, "/v1/catalogue/"+shopID+"/items/"+itemID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var public map[string]any
	decodeBody(t, rec, &public)
	for _, field := range []string{"stock_quantity", "stock_tracked", "active", "availability", "sort_order"} {
		if _, present := public[field]; present {
			t.Errorf("the customer's view carries %q", field)
		}
	}
	if public["orderable"] != true {
		t.Errorf("orderable = %v", public["orderable"])
	}
}

func TestTheOwnerRoutesRefuseABodyThatIsNotJSON(t *testing.T) {
	r := newRig(t, domain.Restaurant)

	requests := [][2]string{
		{http.MethodPost, base + "/categories"},
		{http.MethodPut, base + "/categories/cat_1"},
		{http.MethodPost, base + "/categories/cat_1/active"},
		{http.MethodPost, base + "/items"},
		{http.MethodPut, base + "/items/itm_1"},
		{http.MethodPost, base + "/items/itm_1/active"},
		{http.MethodPut, base + "/items/itm_1/stock"},
		{http.MethodPut, base + "/items/itm_1/availability"},
		{http.MethodPut, base + "/items/itm_1/variants"},
		{http.MethodPut, base + "/items/itm_1/addons"},
		{http.MethodPost, base + "/items/bulk"},
		{http.MethodPost, base + "/combos"},
		{http.MethodPut, base + "/combos/cmb_1"},
		{http.MethodPost, base + "/combos/cmb_1/active"},
		{http.MethodPut, base + "/combos/cmb_1/availability"},
	}
	for _, req := range requests {
		if rec := r.do(req[0], req[1], `{`); rec.Code != http.StatusBadRequest {
			t.Errorf("%s %s: status = %d, want 400", req[0], req[1], rec.Code)
		}
	}
}

func TestCategoriesOverHTTP(t *testing.T) {
	r := newRig(t, domain.Restaurant)
	categoryID := r.createCategory(t, "Biryani")

	rec := r.do(http.MethodGet, base+"/categories", "")
	var list struct {
		Categories []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Active bool   `json:"active"`
		} `json:"categories"`
	}
	decodeBody(t, rec, &list)
	if len(list.Categories) != 1 || list.Categories[0].Name != "Biryani" {
		t.Errorf("categories = %+v", list.Categories)
	}

	if rec := r.do(http.MethodPut, base+"/categories/"+categoryID,
		`{"name":"Rice dishes","sort_order":2}`); rec.Code != http.StatusOK {
		t.Errorf("rename: status = %d, body = %s", rec.Code, rec.Body)
	}
	if rec := r.do(http.MethodPost, base+"/categories/"+categoryID+"/active",
		`{"active":false}`); rec.Code != http.StatusOK {
		t.Errorf("hide: status = %d, body = %s", rec.Code, rec.Body)
	}
	if rec := r.do(http.MethodDelete, base+"/categories/"+categoryID, ""); rec.Code != http.StatusNoContent {
		t.Errorf("delete: status = %d, want 204", rec.Code)
	}
}

func TestItemsOverHTTP(t *testing.T) {
	r := newRig(t, domain.Restaurant)
	categoryID := r.createCategory(t, "Biryani")
	itemID := r.createItem(t, categoryID, `"name":"Kacchi","price_minor":35000`)

	rec := r.do(http.MethodGet, base+"/items?active=true&limit=10&offset=0&q=kacchi&category="+categoryID, "")
	var list struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	decodeBody(t, rec, &list)
	if len(list.Items) != 1 || list.Items[0].ID != itemID {
		t.Errorf("items = %+v", list.Items)
	}

	if rec := r.do(http.MethodPut, base+"/items/"+itemID,
		`{"category_id":"`+categoryID+`","name":"Kacchi biryani","price_minor":36000}`); rec.Code != http.StatusOK {
		t.Errorf("update: status = %d, body = %s", rec.Code, rec.Body)
	}
	if rec := r.do(http.MethodPost, base+"/items/"+itemID+"/active", `{"active":false}`); rec.Code != http.StatusOK {
		t.Errorf("hide: status = %d, body = %s", rec.Code, rec.Body)
	}
	if rec := r.do(http.MethodPut, base+"/items/"+itemID+"/availability",
		`{"days":{"1":["07:00-11:00"]}}`); rec.Code != http.StatusOK {
		t.Errorf("availability: status = %d, body = %s", rec.Code, rec.Body)
	}
	if rec := r.do(http.MethodDelete, base+"/items/"+itemID, ""); rec.Code != http.StatusNoContent {
		t.Errorf("delete: status = %d, want 204", rec.Code)
	}
}

// TestAnUnreadablePageSizeFallsBackToTheDefault: a 400 for "limit=abc" is a
// support call about a page that will not load.
func TestAnUnreadablePageSizeFallsBackToTheDefault(t *testing.T) {
	r := newRig(t, domain.Restaurant)
	if rec := r.do(http.MethodGet, base+"/items?limit=abc&offset=xyz", ""); rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestStockOverHTTP(t *testing.T) {
	r := newRig(t, domain.Grocery)
	categoryID := r.createCategory(t, "Rice")
	itemID := r.createItem(t, categoryID, `"name":"Chal","price_minor":7500,"unit":"kg"`)

	rec := r.do(http.MethodPut, base+"/items/"+itemID+"/stock", `{"quantity":25}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var item struct {
		StockQuantity *int `json:"stock_quantity"`
		Orderable     bool `json:"orderable"`
	}
	decodeBody(t, rec, &item)
	if item.StockQuantity == nil || *item.StockQuantity != 25 || !item.Orderable {
		t.Errorf("item = %+v", item)
	}

	if rec := r.do(http.MethodPut, base+"/items/"+itemID+"/stock", `{"quantity":-1}`); rec.Code != http.StatusBadRequest {
		t.Errorf("negative: status = %d, want 400", rec.Code)
	}
}

func TestOptionGroupsOverHTTP(t *testing.T) {
	r := newRig(t, domain.Restaurant)
	categoryID := r.createCategory(t, "Pizza")
	itemID := r.createItem(t, categoryID, `"name":"Margherita","price_minor":55000`)

	rec := r.do(http.MethodPut, base+"/items/"+itemID+"/variants", `{"groups":[{
		"name":"Size","required":true,"min_choices":1,"max_choices":1,"sort_order":1,
		"options":[{"name":"Regular","price_minor":0},{"name":"Large","price_minor":15000}]
	}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("variants: status = %d, body = %s", rec.Code, rec.Body)
	}

	var withVariants struct {
		VariantGroups []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Required bool   `json:"required"`
			Options  []struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Price struct {
					Display string `json:"display"`
				} `json:"price"`
				Available bool `json:"available"`
			} `json:"options"`
		} `json:"variant_groups"`
	}
	decodeBody(t, rec, &withVariants)
	if len(withVariants.VariantGroups) != 1 || len(withVariants.VariantGroups[0].Options) != 2 {
		t.Fatalf("groups = %+v", withVariants.VariantGroups)
	}
	if withVariants.VariantGroups[0].Options[1].Price.Display != "৳ 150" {
		t.Errorf("large = %+v", withVariants.VariantGroups[0].Options[1])
	}
	if !withVariants.VariantGroups[0].Options[0].Available {
		t.Error("a new option is unavailable")
	}

	rec = r.do(http.MethodPut, base+"/items/"+itemID+"/addons", `{"groups":[{
		"name":"Extras","min_choices":0,"max_choices":2,
		"options":[{"name":"Olives","price_minor":2000}]
	}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("add-ons: status = %d, body = %s", rec.Code, rec.Body)
	}
}

// TestAGroceryIsRefusedAddOnsOverHTTP, with a message about what to do.
func TestAGroceryIsRefusedAddOnsOverHTTP(t *testing.T) {
	r := newRig(t, domain.Grocery)
	categoryID := r.createCategory(t, "Rice")
	itemID := r.createItem(t, categoryID, `"name":"Chal","price_minor":7500,"unit":"kg"`)

	rec := r.do(http.MethodPut, base+"/items/"+itemID+"/addons", `{"groups":[{
		"name":"Extras","max_choices":1,"options":[{"name":"Gift wrap","price_minor":1000}]
	}]}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if got := errorCode(t, rec); got != "addons_not_allowed" {
		t.Errorf("code = %q, want addons_not_allowed", got)
	}
}

func TestBulkUpdateOverHTTP(t *testing.T) {
	r := newRig(t, domain.Grocery)
	categoryID := r.createCategory(t, "Rice")
	first := r.createItem(t, categoryID, `"name":"Chal","price_minor":7500,"unit":"kg"`)
	second := r.createItem(t, categoryID, `"name":"Atap chal","price_minor":8000,"unit":"kg"`)

	rec := r.do(http.MethodPost, base+"/items/bulk", `{"changes":[
		{"item_id":"`+first+`","price_minor":9000,"stock_quantity":60},
		{"item_id":"`+second+`","active":false}
	]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var body struct {
		Items []struct {
			ID    string `json:"id"`
			Price struct {
				Minor int64 `json:"minor"`
			} `json:"price"`
			Active bool `json:"active"`
		} `json:"items"`
	}
	decodeBody(t, rec, &body)
	if len(body.Items) != 2 || body.Items[0].Price.Minor != 9_000 {
		t.Errorf("items = %+v", body.Items)
	}

	rec = r.do(http.MethodPost, base+"/items/bulk", `{"changes":[{"item_id":"itm_nope","active":true}]}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown item: status = %d, want 404", rec.Code)
	}
}

func TestCombosOverHTTP(t *testing.T) {
	r := newRig(t, domain.Restaurant)
	categoryID := r.createCategory(t, "Biryani")
	first := r.createItem(t, categoryID, `"name":"Kacchi","price_minor":35000`)
	second := r.createItem(t, categoryID, `"name":"Borhani","price_minor":6000`)

	rec := r.do(http.MethodPost, base+"/combos", `{
		"name":"Kacchi meal","price_minor":38000,
		"lines":[{"item_id":"`+first+`","quantity":1},{"item_id":"`+second+`","quantity":1}]
	}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var created struct {
		ID      string          `json:"id"`
		Savings json.RawMessage `json:"savings"`
		Lines   []struct {
			ItemID string `json:"item_id"`
		} `json:"lines"`
	}
	decodeBody(t, rec, &created)
	if len(created.Lines) != 2 {
		t.Errorf("lines = %+v", created.Lines)
	}
	// The owner's view omits the saving: it is what a customer saves, and needs
	// the member items loaded.
	if created.Savings != nil {
		t.Errorf("the owner's view carries a saving: %s", created.Savings)
	}

	rec = r.do(http.MethodGet, base+"/combos?active=true", "")
	var list struct {
		Combos []struct {
			ID string `json:"id"`
		} `json:"combos"`
	}
	decodeBody(t, rec, &list)
	if len(list.Combos) != 1 {
		t.Errorf("combos = %+v", list.Combos)
	}

	if rec := r.do(http.MethodPut, base+"/combos/"+created.ID, `{
		"name":"Kacchi meal deal","price_minor":39000,
		"lines":[{"item_id":"`+first+`","quantity":1},{"item_id":"`+second+`","quantity":2}]
	}`); rec.Code != http.StatusOK {
		t.Errorf("update: status = %d, body = %s", rec.Code, rec.Body)
	}
	if rec := r.do(http.MethodPost, base+"/combos/"+created.ID+"/active", `{"active":false}`); rec.Code != http.StatusOK {
		t.Errorf("hide: status = %d, body = %s", rec.Code, rec.Body)
	}
	if rec := r.do(http.MethodPut, base+"/combos/"+created.ID+"/availability",
		`{"days":{"1":["12:00-15:00"]}}`); rec.Code != http.StatusOK {
		t.Errorf("availability: status = %d, body = %s", rec.Code, rec.Body)
	}
	if rec := r.do(http.MethodDelete, base+"/combos/"+created.ID, ""); rec.Code != http.StatusNoContent {
		t.Errorf("delete: status = %d, want 204", rec.Code)
	}
}

// ------------------------------------------------------ the customer's view

func TestTheMenuIsServedInOneCall(t *testing.T) {
	r := newRig(t, domain.Restaurant)
	categoryID := r.createCategory(t, "Biryani")
	first := r.createItem(t, categoryID, `"name":"Kacchi","price_minor":35000`)
	second := r.createItem(t, categoryID, `"name":"Borhani","price_minor":6000`)

	if rec := r.do(http.MethodPost, base+"/combos", `{
		"name":"Kacchi meal","price_minor":38000,
		"lines":[{"item_id":"`+first+`","quantity":1},{"item_id":"`+second+`","quantity":1}]
	}`); rec.Code != http.StatusCreated {
		t.Fatalf("create combo: status = %d, body = %s", rec.Code, rec.Body)
	}

	rec := r.do(http.MethodGet, "/v1/catalogue/"+shopID+"/menu", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}

	var menu struct {
		MerchantID string `json:"merchant_id"`
		Categories []struct {
			ID string `json:"id"`
		} `json:"categories"`
		Items  []map[string]any `json:"items"`
		Combos []struct {
			ID      string `json:"id"`
			Savings struct {
				Minor   int64  `json:"minor"`
				Display string `json:"display"`
			} `json:"savings"`
			Orderable bool `json:"orderable"`
		} `json:"combos"`
	}
	decodeBody(t, rec, &menu)

	if menu.MerchantID != shopID || len(menu.Categories) != 1 || len(menu.Items) != 2 {
		t.Errorf("menu = %+v", menu)
	}
	if len(menu.Combos) != 1 {
		t.Fatalf("combos = %+v", menu.Combos)
	}
	if menu.Combos[0].Savings.Minor != 3_000 || menu.Combos[0].Savings.Display != "৳ 30" {
		t.Errorf("savings = %+v, want the server's own arithmetic", menu.Combos[0].Savings)
	}
	if !menu.Combos[0].Orderable {
		t.Error("the combo is not orderable")
	}
}

func TestTheCustomerCombosRouteIsServed(t *testing.T) {
	r := newRig(t, domain.Restaurant)
	categoryID := r.createCategory(t, "Biryani")
	first := r.createItem(t, categoryID, `"name":"Kacchi","price_minor":35000`)
	second := r.createItem(t, categoryID, `"name":"Borhani","price_minor":6000`)

	rec := r.do(http.MethodPost, base+"/combos", `{
		"name":"Kacchi meal","price_minor":38000,
		"lines":[{"item_id":"`+first+`","quantity":1},{"item_id":"`+second+`","quantity":1}]
	}`)
	var created struct {
		ID string `json:"id"`
	}
	decodeBody(t, rec, &created)

	rec = r.do(http.MethodGet, "/v1/catalogue/"+shopID+"/combos/"+created.ID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var combo struct {
		Savings struct {
			Display string `json:"display"`
		} `json:"savings"`
		Lines []struct {
			Name string `json:"name"`
		} `json:"lines"`
	}
	decodeBody(t, rec, &combo)
	if combo.Savings.Display != "৳ 30" {
		t.Errorf("savings = %+v", combo.Savings)
	}
	if len(combo.Lines) != 2 || combo.Lines[0].Name == "" {
		t.Errorf("lines = %+v, want the member names filled in", combo.Lines)
	}

	if rec := r.do(http.MethodGet, "/v1/catalogue/"+shopID+"/combos/cmb_nope", ""); rec.Code != http.StatusNotFound {
		t.Errorf("unknown combo: status = %d, want 404", rec.Code)
	}
	if rec := r.do(http.MethodGet, "/v1/catalogue/"+shopID+"/items/itm_nope", ""); rec.Code != http.StatusNotFound {
		t.Errorf("unknown item: status = %d, want 404", rec.Code)
	}
}

// TestTheMenuReportsAStoreThatIsDown rather than an empty menu, which would
// read to a customer as a shop with nothing to sell.
func TestTheMenuReportsAStoreThatIsDown(t *testing.T) {
	r := newRig(t, domain.Restaurant)
	r.repo.categoriesErr = errStore

	if rec := r.do(http.MethodGet, "/v1/catalogue/"+shopID+"/menu", ""); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

// -------------------------------------------------------------- capabilities

// TestCapabilitiesAreServedPerShopType, so the merchant app renders the right
// form rather than deciding the rules itself (2.9).
func TestCapabilitiesAreServedPerShopType(t *testing.T) {
	cases := map[domain.MerchantType]struct {
		addOns, tracksStock, requiresUnit, prescriptions bool
		units                                            int
	}{
		domain.Restaurant: {true, false, false, false, 0},
		domain.Grocery:    {false, true, true, false, 8},
		domain.Pharmacy:   {false, true, true, true, 8},
	}

	for kind, want := range cases {
		t.Run(kind.String(), func(t *testing.T) {
			r := newRig(t, kind)
			rec := r.do(http.MethodGet, base+"/capabilities", "")
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
			}

			var body struct {
				MerchantType  string   `json:"merchant_type"`
				Variants      bool     `json:"variants"`
				AddOns        bool     `json:"addons"`
				TracksStock   bool     `json:"tracks_stock"`
				RequiresUnit  bool     `json:"requires_unit"`
				Prescriptions bool     `json:"prescriptions"`
				Units         []string `json:"units"`
			}
			decodeBody(t, rec, &body)

			if body.MerchantType != kind.String() {
				t.Errorf("type = %q", body.MerchantType)
			}
			if !body.Variants {
				t.Error("variants are off; every shop type has them")
			}
			if body.AddOns != want.addOns || body.TracksStock != want.tracksStock ||
				body.RequiresUnit != want.requiresUnit || body.Prescriptions != want.prescriptions {
				t.Errorf("capabilities = %+v, want %+v", body, want)
			}
			if len(body.Units) != want.units {
				t.Errorf("units = %d, want %d", len(body.Units), want.units)
			}
		})
	}
}

// TestCapabilitiesAreOwnerOnly: it is the shop's own configuration surface.
func TestCapabilitiesAreOwnerOnly(t *testing.T) {
	r := newRig(t, domain.Restaurant)
	r.principal = "usr_someone_else"

	if rec := r.do(http.MethodGet, base+"/capabilities", ""); rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

// TestTheOwnerListingsReportAStoreThatIsDown rather than an empty menu, which
// an owner would read as their catalogue having vanished.
func TestTheOwnerListingsReportAStoreThatIsDown(t *testing.T) {
	cases := map[string]struct {
		broken func(*memoryRepo)
		target string
	}{
		"categories": {func(r *memoryRepo) { r.categoriesErr = errStore }, base + "/categories"},
		"items":      {func(r *memoryRepo) { r.itemsErr = errStore }, base + "/items"},
		"combos":     {func(r *memoryRepo) { r.combosErr = errStore }, base + "/combos"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := newRig(t, domain.Restaurant)
			tc.broken(r.repo)

			if rec := r.do(http.MethodGet, tc.target, ""); rec.Code != http.StatusServiceUnavailable {
				t.Errorf("status = %d, want 503", rec.Code)
			}
		})
	}
}
