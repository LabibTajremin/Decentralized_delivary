package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// These drive a shop owner building a menu and a customer reading it, through
// the real binary against real Postgres. The phase's acceptance criterion —
// that the per-merchant-type schema differences hold — is checked here at the
// only level that proves it: the API a Flutter app will actually call.

// approvedShop registers a shop of a given type, takes it through approval, and
// returns its id along with the owner's and an admin's tokens.
func approvedShop(t *testing.T, base string, tail *logTail, kind string) (string, tokens, tokens) {
	t.Helper()

	owner := signInAs(t, base, tail, uniquePhone(t), "merchant phone", "merchant")
	admin := signInAs(t, base, tail, uniquePhone(t), "admin console", "admin")

	var created merchantPayload
	status := requestAs(t, http.MethodPost, base+"/v1/merchants", fmt.Sprintf(`{
		"name": "Demo %s", "type": %q, "phone": "01712345678",
		"line1": "House 12, Road 7", "lat": 23.7461, "lng": 90.3742
	}`, kind, kind), owner.AccessToken, &created)
	if status != http.StatusCreated {
		t.Fatalf("register %s: status = %d", kind, status)
	}

	documents := map[string][]string{
		"restaurant": {"trade_licence", "national_id", "food_licence"},
		"grocery":    {"trade_licence", "national_id"},
		"pharmacy":   {"trade_licence", "national_id", "drug_licence"},
	}
	for _, document := range documents[kind] {
		if status := requestAs(t, http.MethodPut, base+"/v1/merchants/me/documents",
			`{"kind":"`+document+`","number":"N-1","file_url":"/static/demo/doc.png"}`,
			owner.AccessToken, nil); status != http.StatusOK {
			t.Fatalf("upload %s: status = %d", document, status)
		}
	}
	if status := requestAs(t, http.MethodPost, base+"/v1/merchants/me/submit", "",
		owner.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("submit: status = %d", status)
	}
	if status := requestAs(t, http.MethodPost, base+"/v1/admin/merchants/"+created.ID+"/approve",
		`{}`, admin.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("approve: status = %d", status)
	}

	return created.ID, owner, admin
}

// catalogueBase is the owner's menu-management prefix for a shop.
func catalogueBase(base, merchantID string) string {
	return base + "/v1/merchants/" + merchantID + "/catalogue"
}

type idPayload struct {
	ID string `json:"id"`
}

// addCategory creates a menu section and returns its id.
func addCategory(t *testing.T, base, merchantID, token, name string) string {
	t.Helper()
	var created idPayload
	status := requestAs(t, http.MethodPost, catalogueBase(base, merchantID)+"/categories",
		fmt.Sprintf(`{"name":%q}`, name), token, &created)
	if status != http.StatusCreated {
		t.Fatalf("create category %q: status = %d", name, status)
	}
	return created.ID
}

// TestAnOwnerBuildsAMenuAndACustomerReadsIt, end to end.
func TestAnOwnerBuildsAMenuAndACustomerReadsIt(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	merchantID, owner, _ := approvedShop(t, base, tail, "restaurant")
	shopBase := catalogueBase(base, merchantID)
	categoryID := addCategory(t, base, merchantID, owner.AccessToken, "কাচ্চি ও বিরিয়ানি")

	var kacchi idPayload
	status := requestAs(t, http.MethodPost, shopBase+"/items", `{
		"category_id":"`+categoryID+`","name":"কাচ্চি বিরিয়ানি",
		"description":"Slow cooked mutton kacchi.","price_minor":35000,
		"preparation_minutes":40,"sort_order":1
	}`, owner.AccessToken, &kacchi)
	if status != http.StatusCreated {
		t.Fatalf("create item: status = %d", status)
	}

	var borhani idPayload
	if status := requestAs(t, http.MethodPost, shopBase+"/items", `{
		"category_id":"`+categoryID+`","name":"বোরহানি","price_minor":6000,"sort_order":2
	}`, owner.AccessToken, &borhani); status != http.StatusCreated {
		t.Fatalf("create second item: status = %d", status)
	}

	// Sizes and extras.
	if status := requestAs(t, http.MethodPut, shopBase+"/items/"+kacchi.ID+"/variants", `{"groups":[{
		"name":"সাইজ","required":true,"min_choices":1,"max_choices":1,
		"options":[{"name":"হাফ","price_minor":-10000},{"name":"ফুল","price_minor":0}]
	}]}`, owner.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("variants: status = %d", status)
	}
	if status := requestAs(t, http.MethodPut, shopBase+"/items/"+kacchi.ID+"/addons", `{"groups":[{
		"name":"এক্সট্রা","min_choices":0,"max_choices":2,
		"options":[{"name":"ডিম","price_minor":3000}]
	}]}`, owner.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("add-ons: status = %d", status)
	}

	// A combo over the two.
	var combo idPayload
	if status := requestAs(t, http.MethodPost, shopBase+"/combos", `{
		"name":"কাচ্চি মিল","price_minor":38000,
		"lines":[{"item_id":"`+kacchi.ID+`","quantity":1},{"item_id":"`+borhani.ID+`","quantity":1}]
	}`, owner.AccessToken, &combo); status != http.StatusCreated {
		t.Fatalf("create combo: status = %d", status)
	}

	// The customer reads the menu, with no account.
	var menu struct {
		MerchantID string `json:"merchant_id"`
		Categories []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"categories"`
		Items []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Price struct {
				Minor   int64  `json:"minor"`
				Display string `json:"display"`
			} `json:"price"`
			Orderable     bool `json:"orderable"`
			VariantGroups []struct {
				Name     string `json:"name"`
				Required bool   `json:"required"`
				Options  []struct {
					Name  string `json:"name"`
					Price struct {
						Display string `json:"display"`
					} `json:"price"`
				} `json:"options"`
			} `json:"variant_groups"`
			AddOnGroups []struct {
				Name string `json:"name"`
			} `json:"addon_groups"`
		} `json:"items"`
		Combos []struct {
			ID      string `json:"id"`
			Savings struct {
				Minor   int64  `json:"minor"`
				Display string `json:"display"`
			} `json:"savings"`
			Orderable bool `json:"orderable"`
			Lines     []struct {
				Name string `json:"name"`
			} `json:"lines"`
		} `json:"combos"`
	}
	if status := getJSONAs(t, base+"/v1/catalogue/"+merchantID+"/menu", "", &menu); status != http.StatusOK {
		t.Fatalf("menu: status = %d", status)
	}

	if menu.MerchantID != merchantID || len(menu.Categories) != 1 {
		t.Errorf("menu = %+v", menu)
	}
	if len(menu.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(menu.Items))
	}

	var dish = menu.Items[0]
	if dish.Name != "কাচ্চি বিরিয়ানি" {
		t.Errorf("the first item = %q", dish.Name)
	}
	if dish.Price.Minor != 35_000 || dish.Price.Display != "৳ 350" {
		t.Errorf("price = %+v; the server formats money, not the client", dish.Price)
	}
	if !dish.Orderable {
		t.Error("the dish is not orderable")
	}
	if len(dish.VariantGroups) != 1 || !dish.VariantGroups[0].Required {
		t.Fatalf("variant groups = %+v", dish.VariantGroups)
	}
	if got := dish.VariantGroups[0].Options[0].Price.Display; got != "-৳ 100" {
		t.Errorf("the half-size delta = %q, want a formatted discount", got)
	}
	if len(dish.AddOnGroups) != 1 {
		t.Errorf("add-on groups = %+v", dish.AddOnGroups)
	}

	if len(menu.Combos) != 1 {
		t.Fatalf("combos = %d", len(menu.Combos))
	}
	if menu.Combos[0].Savings.Minor != 3_000 || menu.Combos[0].Savings.Display != "৳ 30" {
		t.Errorf("savings = %+v; the server works this out, not the client", menu.Combos[0].Savings)
	}
	if !menu.Combos[0].Orderable || len(menu.Combos[0].Lines) != 2 {
		t.Errorf("combo = %+v", menu.Combos[0])
	}
	if menu.Combos[0].Lines[0].Name == "" {
		t.Error("the combo's lines carry no member names")
	}
}

// TestThePerShopTypeRulesHoldOverHTTP is the phase's acceptance criterion at
// the API. Each case is a field that belongs to one kind of shop and is refused
// on another.
func TestThePerShopTypeRulesHoldOverHTTP(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	cases := []struct {
		name    string
		kind    string
		payload string
		code    string
	}{
		{
			"a grocery item must say how it is sold", "grocery",
			`"name":"চাল","price_minor":7500`, "unit_required",
		},
		{
			"a restaurant dish is not sold by unit", "restaurant",
			`"name":"কাচ্চি","price_minor":35000,"unit":"kg"`, "unit_not_allowed",
		},
		{
			"only a pharmacy item may need a prescription", "grocery",
			`"name":"চাল","price_minor":7500,"unit":"kg","requires_prescription":true`,
			"prescription_not_allowed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			merchantID, owner, _ := approvedShop(t, base, tail, tc.kind)
			categoryID := addCategory(t, base, merchantID, owner.AccessToken, "Section")

			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			status := requestAs(t, http.MethodPost, catalogueBase(base, merchantID)+"/items",
				`{"category_id":"`+categoryID+`",`+tc.payload+`}`, owner.AccessToken, &body)

			if status != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", status)
			}
			if body.Error.Code != tc.code {
				t.Errorf("code = %q, want %q", body.Error.Code, tc.code)
			}
		})
	}
}

// TestCapabilitiesAreServedSoTheAppNeverDecidesThem (2.9).
func TestCapabilitiesAreServedSoTheAppNeverDecidesThem(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	cases := map[string]struct {
		addOns, tracksStock, prescriptions bool
	}{
		"restaurant": {true, false, false},
		"grocery":    {false, true, false},
		"pharmacy":   {false, true, true},
	}

	for kind, want := range cases {
		t.Run(kind, func(t *testing.T) {
			merchantID, owner, _ := approvedShop(t, base, tail, kind)

			var body struct {
				MerchantType  string   `json:"merchant_type"`
				Variants      bool     `json:"variants"`
				AddOns        bool     `json:"addons"`
				TracksStock   bool     `json:"tracks_stock"`
				Prescriptions bool     `json:"prescriptions"`
				Units         []string `json:"units"`
			}
			if status := getJSONAs(t, catalogueBase(base, merchantID)+"/capabilities",
				owner.AccessToken, &body); status != http.StatusOK {
				t.Fatalf("status = %d", status)
			}

			if body.MerchantType != kind || !body.Variants {
				t.Errorf("capabilities = %+v", body)
			}
			if body.AddOns != want.addOns || body.TracksStock != want.tracksStock ||
				body.Prescriptions != want.prescriptions {
				t.Errorf("capabilities = %+v, want %+v", body, want)
			}
			if want.tracksStock && len(body.Units) == 0 {
				t.Error("a shop that sells by unit was offered none")
			}
		})
	}
}

// TestAGroceryManagesItsShelfAndACustomerSeesOnlyTheAnswer.
func TestAGroceryManagesItsShelfAndACustomerSeesOnlyTheAnswer(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	merchantID, owner, _ := approvedShop(t, base, tail, "grocery")
	shopBase := catalogueBase(base, merchantID)
	categoryID := addCategory(t, base, merchantID, owner.AccessToken, "চাল")

	var rice idPayload
	if status := requestAs(t, http.MethodPost, shopBase+"/items", `{
		"category_id":"`+categoryID+`","name":"মিনিকেট চাল","price_minor":7500,
		"unit":"kg","pack_size":"1 kg","brand":"Pran","stock_quantity":40
	}`, owner.AccessToken, &rice); status != http.StatusCreated {
		t.Fatalf("create item: status = %d", status)
	}

	// The customer sees that it can be bought, and not how many are left.
	var public map[string]any
	if status := getJSONAs(t, base+"/v1/catalogue/"+merchantID+"/items/"+rice.ID, "", &public); status != http.StatusOK {
		t.Fatalf("public item: status = %d", status)
	}
	if public["orderable"] != true {
		t.Errorf("orderable = %v", public["orderable"])
	}
	if public["unit"] != "kg" {
		t.Errorf("unit = %v", public["unit"])
	}
	for _, field := range []string{"stock_quantity", "stock_tracked", "active"} {
		if _, present := public[field]; present {
			t.Errorf("the customer's view carries %q", field)
		}
	}

	// It sells out.
	if status := requestAs(t, http.MethodPut, shopBase+"/items/"+rice.ID+"/stock",
		`{"quantity":0}`, owner.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("set stock: status = %d", status)
	}

	public = nil
	if status := getJSONAs(t, base+"/v1/catalogue/"+merchantID+"/items/"+rice.ID, "", &public); status != http.StatusOK {
		t.Fatalf("public item: status = %d", status)
	}
	if public["orderable"] != false {
		t.Error("a sold-out item is still orderable")
	}
	if public["unavailable_reason"] != "out_of_stock" {
		t.Errorf("reason = %v, want out_of_stock", public["unavailable_reason"])
	}
}

// TestABulkRePricingIsAppliedInOneGo.
func TestABulkRePricingIsAppliedInOneGo(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	merchantID, owner, _ := approvedShop(t, base, tail, "grocery")
	shopBase := catalogueBase(base, merchantID)
	categoryID := addCategory(t, base, merchantID, owner.AccessToken, "চাল")

	var first, second idPayload
	for _, spec := range []struct {
		into *idPayload
		name string
	}{{&first, "মিনিকেট"}, {&second, "আতপ"}} {
		if status := requestAs(t, http.MethodPost, shopBase+"/items", fmt.Sprintf(`{
			"category_id":%q,"name":%q,"price_minor":7500,"unit":"kg","stock_quantity":10
		}`, categoryID, spec.name), owner.AccessToken, spec.into); status != http.StatusCreated {
			t.Fatalf("create %s: status = %d", spec.name, status)
		}
	}

	var updated struct {
		Items []struct {
			ID    string `json:"id"`
			Price struct {
				Minor int64 `json:"minor"`
			} `json:"price"`
		} `json:"items"`
	}
	status := requestAs(t, http.MethodPost, shopBase+"/items/bulk", `{"changes":[
		{"item_id":"`+first.ID+`","price_minor":9000},
		{"item_id":"`+second.ID+`","price_minor":9500}
	]}`, owner.AccessToken, &updated)
	if status != http.StatusOK {
		t.Fatalf("bulk: status = %d", status)
	}
	if len(updated.Items) != 2 {
		t.Fatalf("items = %d", len(updated.Items))
	}

	// An unknown id refuses the whole request, leaving both prices as they were.
	status = requestAs(t, http.MethodPost, shopBase+"/items/bulk", `{"changes":[
		{"item_id":"`+first.ID+`","price_minor":11000},
		{"item_id":"itm_nope","price_minor":11000}
	]}`, owner.AccessToken, nil)
	if status != http.StatusNotFound {
		t.Errorf("status = %d, want 404", status)
	}

	var check struct {
		Price struct {
			Minor int64 `json:"minor"`
		} `json:"price"`
	}
	if status := getJSONAs(t, base+"/v1/catalogue/"+merchantID+"/items/"+first.ID, "", &check); status != http.StatusOK {
		t.Fatalf("read back: status = %d", status)
	}
	if check.Price.Minor != 9_000 {
		t.Errorf("price = %d, want the refused batch to have changed nothing", check.Price.Minor)
	}
}

// TestOneShopOwnerCannotTouchAnothersMenu. The merchant id is in the path, so
// this is the check that stops any signed-in account rewriting any menu.
func TestOneShopOwnerCannotTouchAnothersMenu(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	merchantID, owner, _ := approvedShop(t, base, tail, "restaurant")
	categoryID := addCategory(t, base, merchantID, owner.AccessToken, "Biryani")

	intruder := signInAs(t, base, tail, uniquePhone(t), "another phone", "merchant")
	shopBase := catalogueBase(base, merchantID)

	requests := []struct{ method, path, body string }{
		{http.MethodPost, shopBase + "/categories", `{"name":"Mine now"}`},
		{http.MethodPut, shopBase + "/categories/" + categoryID, `{"name":"Renamed"}`},
		{http.MethodDelete, shopBase + "/categories/" + categoryID, ""},
		{http.MethodPost, shopBase + "/items", `{"category_id":"` + categoryID + `","name":"X","price_minor":1}`},
		{http.MethodGet, shopBase + "/capabilities", ""},
	}

	for _, req := range requests {
		status := requestAs(t, req.method, req.path, req.body, intruder.AccessToken, nil)
		if status != http.StatusForbidden {
			t.Errorf("%s %s: status = %d, want 403", req.method, req.path, status)
		}
	}
}

// TestBrowsingAMenuNeedsNoAccount: requiring a token would put a sign-up wall
// in front of the thing customers came for.
func TestBrowsingAMenuNeedsNoAccount(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	merchantID, owner, _ := approvedShop(t, base, tail, "restaurant")
	categoryID := addCategory(t, base, merchantID, owner.AccessToken, "Biryani")

	var item idPayload
	if status := requestAs(t, http.MethodPost, catalogueBase(base, merchantID)+"/items",
		`{"category_id":"`+categoryID+`","name":"Kacchi","price_minor":35000}`,
		owner.AccessToken, &item); status != http.StatusCreated {
		t.Fatalf("create item: status = %d", status)
	}

	for _, target := range []string{
		"/v1/catalogue/" + merchantID + "/menu",
		"/v1/catalogue/" + merchantID + "/items/" + item.ID,
	} {
		if status := getJSONAs(t, base+target, "", nil); status != http.StatusOK {
			t.Errorf("%s with no token: status = %d, want 200", target, status)
		}
	}
}

// TestAHiddenSectionTakesItsItemsOffTheMenu, so an owner hiding a seasonal menu
// does not have to hide each dish too.
func TestAHiddenSectionTakesItsItemsOffTheMenu(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	merchantID, owner, _ := approvedShop(t, base, tail, "restaurant")
	shopBase := catalogueBase(base, merchantID)
	categoryID := addCategory(t, base, merchantID, owner.AccessToken, "শীতের বিশেষ")

	if status := requestAs(t, http.MethodPost, shopBase+"/items",
		`{"category_id":"`+categoryID+`","name":"হালিম","price_minor":20000}`,
		owner.AccessToken, nil); status != http.StatusCreated {
		t.Fatalf("create item: status = %d", status)
	}

	var menu struct {
		Items []idPayload `json:"items"`
	}
	if status := getJSONAs(t, base+"/v1/catalogue/"+merchantID+"/menu", "", &menu); status != http.StatusOK {
		t.Fatalf("menu: status = %d", status)
	}
	if len(menu.Items) != 1 {
		t.Fatalf("items = %d, want the dish on the menu", len(menu.Items))
	}

	if status := requestAs(t, http.MethodPost, shopBase+"/categories/"+categoryID+"/active",
		`{"active":false}`, owner.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("hide section: status = %d", status)
	}

	menu.Items = nil
	if status := getJSONAs(t, base+"/v1/catalogue/"+merchantID+"/menu", "", &menu); status != http.StatusOK {
		t.Fatalf("menu: status = %d", status)
	}
	if len(menu.Items) != 0 {
		t.Errorf("items = %d, want none from a hidden section", len(menu.Items))
	}
}

// TestASectionWithFoodInItCannotBeDeletedOverHTTP, with a message telling the
// owner what to do rather than a constraint violation.
func TestASectionWithFoodInItCannotBeDeletedOverHTTP(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	merchantID, owner, _ := approvedShop(t, base, tail, "restaurant")
	shopBase := catalogueBase(base, merchantID)
	categoryID := addCategory(t, base, merchantID, owner.AccessToken, "Biryani")

	var item idPayload
	if status := requestAs(t, http.MethodPost, shopBase+"/items",
		`{"category_id":"`+categoryID+`","name":"Kacchi","price_minor":35000}`,
		owner.AccessToken, &item); status != http.StatusCreated {
		t.Fatalf("create item: status = %d", status)
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	status := requestAs(t, http.MethodDelete, shopBase+"/categories/"+categoryID, "",
		owner.AccessToken, &body)
	if status != http.StatusConflict {
		t.Fatalf("status = %d, want 409", status)
	}
	if body.Error.Code != "category_not_empty" {
		t.Errorf("code = %q, want category_not_empty", body.Error.Code)
	}

	// Emptied, it goes.
	if status := requestAs(t, http.MethodDelete, shopBase+"/items/"+item.ID, "",
		owner.AccessToken, nil); status != http.StatusNoContent {
		t.Fatalf("delete item: status = %d", status)
	}
	if status := requestAs(t, http.MethodDelete, shopBase+"/categories/"+categoryID, "",
		owner.AccessToken, nil); status != http.StatusNoContent {
		t.Errorf("delete section: status = %d, want 204", status)
	}
}

// TestAnItemOnASchedulseIsOffTheMenuOutsideIt, decided by the server against
// its own clock rather than the client's.
func TestAnItemOnAScheduleIsOffTheMenuOutsideIt(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	merchantID, owner, _ := approvedShop(t, base, tail, "restaurant")
	shopBase := catalogueBase(base, merchantID)
	categoryID := addCategory(t, base, merchantID, owner.AccessToken, "সকালের নাস্তা")

	var paratha idPayload
	if status := requestAs(t, http.MethodPost, shopBase+"/items",
		`{"category_id":"`+categoryID+`","name":"পরোটা","price_minor":3000}`,
		owner.AccessToken, &paratha); status != http.StatusCreated {
		t.Fatalf("create item: status = %d", status)
	}

	// A window that has already closed on every day of the week, so the answer
	// does not depend on when this test runs.
	if status := requestAs(t, http.MethodPut, shopBase+"/items/"+paratha.ID+"/availability", `{"days":{
		"0":["00:00-00:01"],"1":["00:00-00:01"],"2":["00:00-00:01"],"3":["00:00-00:01"],
		"4":["00:00-00:01"],"5":["00:00-00:01"],"6":["00:00-00:01"]
	}}`, owner.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("availability: status = %d", status)
	}

	var public map[string]any
	if status := getJSONAs(t, base+"/v1/catalogue/"+merchantID+"/items/"+paratha.ID, "", &public); status != http.StatusOK {
		t.Fatalf("public item: status = %d", status)
	}
	if public["orderable"] != false {
		t.Error("an item outside its window is orderable")
	}
	if public["unavailable_reason"] != "not_available_now" {
		t.Errorf("reason = %v, want not_available_now", public["unavailable_reason"])
	}

	// Back to always available, and it returns.
	if status := requestAs(t, http.MethodPut, shopBase+"/items/"+paratha.ID+"/availability",
		`{"always":true}`, owner.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("availability: status = %d", status)
	}
	public = nil
	if status := getJSONAs(t, base+"/v1/catalogue/"+merchantID+"/items/"+paratha.ID, "", &public); status != http.StatusOK {
		t.Fatalf("public item: status = %d", status)
	}
	if public["orderable"] != true {
		t.Error("the item did not come back")
	}
}

// TestTheDemoShopsHaveRealMenus. The merchant seed approves fourteen shops and
// the geo seed puts them on the map; a demo that opens one and finds nothing to
// buy is the opposite of what a demo is for. This also checks the three seeded
// menus have the shape their shop type calls for, which is the acceptance
// criterion showing up in the data rather than only in the rules.
func TestTheDemoShopsHaveRealMenus(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()
	admin := signInAs(t, base, tail, uniquePhone(t), "admin console", "admin")

	var shops struct {
		Merchants []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"merchants"`
	}
	if status := getJSONAs(t, base+"/v1/admin/merchants?status=approved&limit=100",
		admin.AccessToken, &shops); status != http.StatusOK {
		t.Fatalf("list shops: status = %d", status)
	}
	if len(shops.Merchants) == 0 {
		t.Fatal("no approved demo shops")
	}

	seenTypes := map[string]bool{}
	// A pharmacy stocks medicines and also plain health goods — a hand
	// sanitiser has no generic drug name and needs no prescription — so these
	// are asserted across the shop's menu rather than on every item.
	sawGenericName, sawPrescription := false, false

	for _, shop := range shops.Merchants {
		var menu struct {
			Categories []idPayload `json:"categories"`
			Items      []struct {
				ID                   string `json:"id"`
				Unit                 string `json:"unit"`
				GenericName          string `json:"generic_name"`
				RequiresPrescription bool   `json:"requires_prescription"`
				PreparationMinutes   int    `json:"preparation_minutes"`
				Orderable            bool   `json:"orderable"`
				Price                struct {
					Display string `json:"display"`
				} `json:"price"`
			} `json:"items"`
		}
		if status := getJSONAs(t, base+"/v1/catalogue/"+shop.ID+"/menu", "", &menu); status != http.StatusOK {
			t.Errorf("%s: menu status = %d", shop.ID, status)
			continue
		}

		if len(menu.Categories) == 0 || len(menu.Items) == 0 {
			t.Errorf("%s (%s) has an empty menu", shop.ID, shop.Type)
			continue
		}
		seenTypes[shop.Type] = true

		for _, item := range menu.Items {
			if item.Price.Display == "" {
				t.Errorf("%s: an item has no formatted price", shop.ID)
			}
			if !item.Orderable {
				t.Errorf("%s: %s is not orderable in a fresh demo", shop.ID, item.ID)
			}

			switch shop.Type {
			case "restaurant":
				if item.Unit != "" {
					t.Errorf("%s: a restaurant dish carries a unit %q", shop.ID, item.Unit)
				}
			case "grocery":
				if item.Unit == "" {
					t.Errorf("%s: a grocery item carries no unit", shop.ID)
				}
				if item.RequiresPrescription {
					t.Errorf("%s: a grocery item needs a prescription", shop.ID)
				}
			case "pharmacy":
				if item.Unit == "" {
					t.Errorf("%s: a pharmacy item carries no unit", shop.ID)
				}
				if item.GenericName != "" {
					sawGenericName = true
				}
				if item.RequiresPrescription {
					sawPrescription = true
				}
			}
		}
	}

	for _, kind := range []string{"restaurant", "grocery", "pharmacy"} {
		if !seenTypes[kind] {
			t.Errorf("the demo has no %s with a menu", kind)
		}
	}
	if !sawGenericName {
		t.Error("no pharmacy item carries a generic name")
	}
	if !sawPrescription {
		t.Error("no pharmacy item requires a prescription; the flag would go untested in a demo")
	}
}

// TestTheDemoFlagshipShowsTheWholeRestaurantShape — a required size, optional
// extras, and a combo priced below its parts — so the app has something real to
// render on every control it supports.
func TestTheDemoFlagshipShowsTheWholeRestaurantShape(t *testing.T) {
	base, _, stop := startAuthAPI(t)
	defer stop()

	var menu struct {
		Items []struct {
			Name          string `json:"name"`
			VariantGroups []struct {
				Required bool `json:"required"`
				Options  []struct {
					Price struct {
						Minor int64 `json:"minor"`
					} `json:"price"`
				} `json:"options"`
			} `json:"variant_groups"`
			AddOnGroups []struct {
				Options []idPayload `json:"options"`
			} `json:"addon_groups"`
		} `json:"items"`
		Combos []struct {
			Savings struct {
				Minor   int64  `json:"minor"`
				Display string `json:"display"`
			} `json:"savings"`
			Lines []idPayload `json:"lines"`
		} `json:"combos"`
	}
	if status := getJSONAs(t, base+"/v1/catalogue/MER-DEMO-0001/menu", "", &menu); status != http.StatusOK {
		t.Fatalf("menu: status = %d", status)
	}

	var withOptions int
	var sawDiscountVariant bool
	for _, item := range menu.Items {
		if len(item.VariantGroups) == 0 {
			continue
		}
		withOptions++
		if !item.VariantGroups[0].Required {
			t.Error("the size group is not required; a kacchi has no price until one is picked")
		}
		for _, option := range item.VariantGroups[0].Options {
			if option.Price.Minor < 0 {
				sawDiscountVariant = true
			}
		}
		if len(item.AddOnGroups) == 0 || len(item.AddOnGroups[0].Options) == 0 {
			t.Error("the flagship dish has no add-ons")
		}
	}
	if withOptions == 0 {
		t.Error("no item on the flagship menu has variants")
	}
	if !sawDiscountVariant {
		t.Error("no variant costs less than the base price; the half size should")
	}

	if len(menu.Combos) != 1 {
		t.Fatalf("combos = %d, want the seeded meal deal", len(menu.Combos))
	}
	if menu.Combos[0].Savings.Minor <= 0 {
		t.Errorf("savings = %+v, want a combo cheaper than its parts", menu.Combos[0].Savings)
	}
	if len(menu.Combos[0].Lines) != 2 {
		t.Errorf("lines = %d, want two", len(menu.Combos[0].Lines))
	}
}
