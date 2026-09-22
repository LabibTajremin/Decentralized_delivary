// Package http exposes a shop's catalogue to its owner and to customers.
//
// Two surfaces on one handler. The owner routes are nested under
// /v1/merchants/{merchantId}/catalogue and every one checks that the caller
// owns that shop — the merchant id is in the path because an owner may manage a
// shop, but ownership is verified against the merchant record, never taken on
// trust. The customer routes under /v1/catalogue are read-only and public:
// browsing a menu is what the app does before anyone signs in.
package http

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/platform/httpx"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// Guard wraps a handler in an access requirement. Passed in rather than
// imported, so the catalogue does not depend on identity's transport.
type Guard func(http.Handler) http.Handler

// PrincipalOf reports the authenticated caller's user id.
type PrincipalOf func(*http.Request) (userID string, ok bool)

// Handler serves the catalogue endpoints.
type Handler struct {
	categories *application.CategoryUseCase
	items      *application.ItemUseCase
	options    *application.OptionUseCase
	combos     *application.ComboUseCase
	service    *application.Service
	authed     Guard
	principal  PrincipalOf
}

// NewHandler builds the handler.
//
// A nil guard or principal reader would let any caller rewrite any menu in the
// country, so both are refused at wiring time rather than becoming a hole
// nobody notices.
func NewHandler(
	categories *application.CategoryUseCase,
	items *application.ItemUseCase,
	options *application.OptionUseCase,
	combos *application.ComboUseCase,
	service *application.Service,
	authed Guard,
	principal PrincipalOf,
) *Handler {
	if authed == nil || principal == nil {
		panic("catalogue transport: a guard and a principal reader are required")
	}
	return &Handler{
		categories: categories, items: items, options: options, combos: combos,
		service: service, authed: authed, principal: principal,
	}
}

// ownerRoutes are the shop owner's menu-management endpoints.
func (h *Handler) ownerRoutes() map[string]http.HandlerFunc {
	const base = "/v1/merchants/{merchantId}/catalogue"
	return map[string]http.HandlerFunc{
		"GET " + base + "/categories":                      h.listCategories,
		"POST " + base + "/categories":                     h.createCategory,
		"PUT " + base + "/categories/{categoryId}":         h.updateCategory,
		"POST " + base + "/categories/{categoryId}/active": h.setCategoryActive,
		"DELETE " + base + "/categories/{categoryId}":      h.deleteCategory,

		"GET " + base + "/items":                       h.listItems,
		"POST " + base + "/items":                      h.createItem,
		"GET " + base + "/items/{itemId}":              h.getItem,
		"PUT " + base + "/items/{itemId}":              h.updateItem,
		"DELETE " + base + "/items/{itemId}":           h.deleteItem,
		"POST " + base + "/items/{itemId}/active":      h.setItemActive,
		"PUT " + base + "/items/{itemId}/stock":        h.setItemStock,
		"PUT " + base + "/items/{itemId}/availability": h.setItemAvailability,
		"PUT " + base + "/items/{itemId}/variants":     h.setVariantGroups,
		"PUT " + base + "/items/{itemId}/addons":       h.setAddOnGroups,
		"POST " + base + "/items/bulk":                 h.bulkUpdateItems,

		"GET " + base + "/combos":                        h.listCombos,
		"POST " + base + "/combos":                       h.createCombo,
		"PUT " + base + "/combos/{comboId}":              h.updateCombo,
		"DELETE " + base + "/combos/{comboId}":           h.deleteCombo,
		"POST " + base + "/combos/{comboId}/active":      h.setComboActive,
		"PUT " + base + "/combos/{comboId}/availability": h.setComboAvailability,

		"GET " + base + "/capabilities": h.capabilities,
	}
}

// publicRoutes are what a customer browsing a shop calls.
func (h *Handler) publicRoutes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"GET /v1/catalogue/{merchantId}/menu":             h.menu,
		"GET /v1/catalogue/{merchantId}/items/{itemId}":   h.publicItem,
		"GET /v1/catalogue/{merchantId}/combos/{comboId}": h.publicCombo,
	}
}

// Register mounts the routes, owner routes behind the guard.
func (h *Handler) Register(mux *http.ServeMux) {
	for pattern, handle := range h.ownerRoutes() {
		mux.Handle(pattern, h.authed(handle))
	}
	for pattern, handle := range h.publicRoutes() {
		mux.Handle(pattern, handle)
	}
}

// Patterns returns every route this module serves, sorted.
func Patterns() []string {
	empty := &Handler{}
	out := make([]string, 0, 32)
	for pattern := range empty.ownerRoutes() {
		out = append(out, pattern)
	}
	for pattern := range empty.publicRoutes() {
		out = append(out, pattern)
	}
	sort.Strings(out)
	return out
}

// caller returns the authenticated user id.
func (h *Handler) caller(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, ok := h.principal(r)
	if !ok || userID == "" {
		httpx.WriteError(w, errs.New(errs.KindUnauthorized, "not_authenticated", "Please sign in again."))
		return "", false
	}
	return userID, true
}

func merchantID(r *http.Request) string { return r.PathValue("merchantId") }

// ----------------------------------------------------------- wire shapes

type categoryResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	Active    bool   `json:"active"`
}

func renderCategory(c domain.Category) categoryResponse {
	return categoryResponse{ID: c.ID, Name: c.Name, SortOrder: c.SortOrder, Active: c.Active}
}

type moneyResponse struct {
	Minor    int64  `json:"minor"`
	Currency string `json:"currency"`
	Display  string `json:"display"`
}

func renderMoney(m contract.Money) moneyResponse {
	return moneyResponse{Minor: m.Minor, Currency: m.Currency, Display: m.Display}
}

type optionResponse struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Price     moneyResponse `json:"price"`
	Available bool          `json:"available"`
}

type optionGroupResponse struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Required   bool             `json:"required"`
	MinChoices int              `json:"min_choices"`
	MaxChoices int              `json:"max_choices"`
	Options    []optionResponse `json:"options"`
}

func renderGroups(groups []contract.OptionGroup) []optionGroupResponse {
	out := make([]optionGroupResponse, 0, len(groups))
	for _, group := range groups {
		options := make([]optionResponse, 0, len(group.Options))
		for _, option := range group.Options {
			options = append(options, optionResponse{
				ID: option.ID, Name: option.Name,
				Price: renderMoney(option.Price), Available: option.Available,
			})
		}
		out = append(out, optionGroupResponse{
			ID: group.ID, Name: group.Name, Required: group.Required,
			MinChoices: group.MinChoices, MaxChoices: group.MaxChoices, Options: options,
		})
	}
	return out
}

// itemResponse is the wire form of an item.
//
// It goes through the contract conversion so the item the app shows and the item
// a consuming module receives are assembled by the same code, and cannot drift
// apart. The owner's view adds the fields only an owner needs.
type itemResponse struct {
	ID          string        `json:"id"`
	MerchantID  string        `json:"merchant_id"`
	CategoryID  string        `json:"category_id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	ImageURL    string        `json:"image_url,omitempty"`
	Price       moneyResponse `json:"price"`

	Orderable         bool   `json:"orderable"`
	UnavailableReason string `json:"unavailable_reason,omitempty"`

	Unit                 string `json:"unit,omitempty"`
	PackSize             string `json:"pack_size,omitempty"`
	Brand                string `json:"brand,omitempty"`
	GenericName          string `json:"generic_name,omitempty"`
	Strength             string `json:"strength,omitempty"`
	RequiresPrescription bool   `json:"requires_prescription,omitempty"`
	IsVegetarian         bool   `json:"is_vegetarian,omitempty"`
	PreparationMinutes   int    `json:"preparation_minutes,omitempty"`

	VariantGroups []optionGroupResponse `json:"variant_groups"`
	AddOnGroups   []optionGroupResponse `json:"addon_groups"`

	// Owner-only. A customer has no use for a shelf count, and publishing one
	// tells a competitor exactly how a shop is doing.
	Active        bool                `json:"active,omitempty"`
	StockTracked  bool                `json:"stock_tracked,omitempty"`
	StockQuantity *int                `json:"stock_quantity,omitempty"`
	Availability  map[string][]string `json:"availability,omitempty"`
	SortOrder     int                 `json:"sort_order,omitempty"`
}

// renderItem builds the customer's view.
func renderItem(c contract.Item) itemResponse {
	return itemResponse{
		ID: c.ID, MerchantID: c.MerchantID, CategoryID: c.CategoryID,
		Name: c.Name, Description: c.Description, ImageURL: c.ImageURL,
		Price:     renderMoney(c.Price),
		Orderable: c.Orderable, UnavailableReason: c.UnavailableReason,
		Unit: c.Unit, PackSize: c.PackSize, Brand: c.Brand,
		GenericName: c.GenericName, Strength: c.Strength,
		RequiresPrescription: c.RequiresPrescription,
		IsVegetarian:         c.IsVegetarian, PreparationMinutes: c.PreparationMinutes,
		VariantGroups: renderGroups(c.VariantGroups),
		AddOnGroups:   renderGroups(c.AddOnGroups),
	}
}

// renderOwnerItem adds the fields only the owner sees.
func (h *Handler) renderOwnerItem(i domain.Item) itemResponse {
	out := renderItem(application.ToContractItem(i, h.items.Now()))
	out.Active = i.Active
	out.StockTracked = i.Stock.Tracked
	if i.Stock.Tracked {
		quantity := i.Stock.Quantity
		out.StockQuantity = &quantity
	}
	out.Availability = i.Availability.Encode()
	out.SortOrder = i.SortOrder
	return out
}

type comboLineResponse struct {
	ItemID   string `json:"item_id"`
	Name     string `json:"name,omitempty"`
	Quantity int    `json:"quantity"`
}

type comboResponse struct {
	ID          string        `json:"id"`
	MerchantID  string        `json:"merchant_id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	ImageURL    string        `json:"image_url,omitempty"`
	Price       moneyResponse `json:"price"`
	// Savings is absent on the owner's own view: it is what a bundle saves a
	// customer against its parts, which needs the member items loaded, and the
	// owner's screen is about what they configured.
	Savings      *moneyResponse      `json:"savings,omitempty"`
	Lines        []comboLineResponse `json:"lines"`
	Orderable    bool                `json:"orderable"`
	Active       bool                `json:"active,omitempty"`
	Availability map[string][]string `json:"availability,omitempty"`
	SortOrder    int                 `json:"sort_order,omitempty"`
}

func renderCombo(c contract.Combo) comboResponse {
	lines := make([]comboLineResponse, 0, len(c.Lines))
	for _, line := range c.Lines {
		lines = append(lines, comboLineResponse{
			ItemID: line.ItemID, Name: line.Name, Quantity: line.Quantity,
		})
	}
	savings := renderMoney(c.Savings)
	return comboResponse{
		ID: c.ID, MerchantID: c.MerchantID, Name: c.Name,
		Description: c.Description, ImageURL: c.ImageURL,
		Price: renderMoney(c.Price), Savings: &savings,
		Lines: lines, Orderable: c.Orderable,
	}
}

// renderOwnerCombo shows the owner what they configured, without needing the
// member items loaded to compute a saving they did not ask for.
func renderOwnerCombo(c domain.Combo) comboResponse {
	lines := make([]comboLineResponse, 0, len(c.Lines))
	for _, line := range c.Lines {
		lines = append(lines, comboLineResponse{ItemID: line.ItemID, Quantity: line.Quantity})
	}
	return comboResponse{
		ID: c.ID, MerchantID: c.MerchantID, Name: c.Name,
		Description: c.Description, ImageURL: c.ImageURL,
		Price: renderMoney(application.ToContractMoney(c.Price)),
		Lines: lines, Active: c.Active,
		Availability: c.Availability.Encode(), SortOrder: c.SortOrder,
	}
}

// -------------------------------------------------------------- categories

// GET /v1/merchants/{merchantId}/catalogue/categories
func (h *Handler) listCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.categories.List(r.Context(), merchantID(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	out := make([]categoryResponse, 0, len(categories))
	for _, category := range categories {
		out = append(out, renderCategory(category))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"categories": out})
}

type categoryBody struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

// POST /v1/merchants/{merchantId}/catalogue/categories
func (h *Handler) createCategory(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body categoryBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	category, err := h.categories.Create(r.Context(), userID, merchantID(r),
		application.CategoryRequest{Name: body.Name, SortOrder: body.SortOrder})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, renderCategory(category))
}

// PUT /v1/merchants/{merchantId}/catalogue/categories/{categoryId}
func (h *Handler) updateCategory(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body categoryBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	category, err := h.categories.Update(r.Context(), userID, merchantID(r), r.PathValue("categoryId"),
		application.CategoryRequest{Name: body.Name, SortOrder: body.SortOrder})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, renderCategory(category))
}

type activeBody struct {
	Active bool `json:"active"`
}

// POST /v1/merchants/{merchantId}/catalogue/categories/{categoryId}/active
func (h *Handler) setCategoryActive(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body activeBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	category, err := h.categories.SetActive(r.Context(), userID, merchantID(r),
		r.PathValue("categoryId"), body.Active)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, renderCategory(category))
}

// DELETE /v1/merchants/{merchantId}/catalogue/categories/{categoryId}
func (h *Handler) deleteCategory(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	if err := h.categories.Delete(r.Context(), userID, merchantID(r), r.PathValue("categoryId")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

// ------------------------------------------------------------------- items

type itemBody struct {
	CategoryID  string `json:"category_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
	PriceMinor  int64  `json:"price_minor"`
	SortOrder   int    `json:"sort_order"`

	IsVegetarian         bool   `json:"is_vegetarian"`
	PreparationMinutes   int    `json:"preparation_minutes"`
	Unit                 string `json:"unit"`
	PackSize             string `json:"pack_size"`
	Brand                string `json:"brand"`
	GenericName          string `json:"generic_name"`
	Strength             string `json:"strength"`
	RequiresPrescription bool   `json:"requires_prescription"`

	// A pointer, so "leave the shelf count alone" and "set it to zero" are
	// different requests. Without that, an owner fixing a typo in a name would
	// take their whole inventory offline.
	StockQuantity *int `json:"stock_quantity"`
}

func (b itemBody) toRequest() application.ItemRequest {
	return application.ItemRequest{
		CategoryID: b.CategoryID, Name: b.Name, Description: b.Description,
		ImageURL: b.ImageURL, PriceMinor: b.PriceMinor, SortOrder: b.SortOrder,
		IsVegetarian: b.IsVegetarian, PreparationMinutes: b.PreparationMinutes,
		Unit: b.Unit, PackSize: b.PackSize, Brand: b.Brand,
		GenericName: b.GenericName, Strength: b.Strength,
		RequiresPrescription: b.RequiresPrescription,
		StockQuantity:        b.StockQuantity,
	}
}

// GET /v1/merchants/{merchantId}/catalogue/items
func (h *Handler) listItems(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	items, err := h.items.List(r.Context(), merchantID(r), application.ListRequest{
		CategoryID: query.Get("category"),
		ActiveOnly: query.Get("active") == "true",
		Search:     query.Get("q"),
		Limit:      intParam(query.Get("limit")),
		Offset:     intParam(query.Get("offset")),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	out := make([]itemResponse, 0, len(items))
	for _, item := range items {
		out = append(out, h.renderOwnerItem(item))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

// POST /v1/merchants/{merchantId}/catalogue/items
func (h *Handler) createItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body itemBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	item, err := h.items.Create(r.Context(), userID, merchantID(r), body.toRequest())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, h.renderOwnerItem(item))
}

// GET /v1/merchants/{merchantId}/catalogue/items/{itemId}
func (h *Handler) getItem(w http.ResponseWriter, r *http.Request) {
	item, err := h.items.Get(r.Context(), merchantID(r), r.PathValue("itemId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, h.renderOwnerItem(item))
}

// PUT /v1/merchants/{merchantId}/catalogue/items/{itemId}
func (h *Handler) updateItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body itemBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	item, err := h.items.Update(r.Context(), userID, merchantID(r), r.PathValue("itemId"), body.toRequest())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, h.renderOwnerItem(item))
}

// DELETE /v1/merchants/{merchantId}/catalogue/items/{itemId}
func (h *Handler) deleteItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	if err := h.items.Delete(r.Context(), userID, merchantID(r), r.PathValue("itemId")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

// POST /v1/merchants/{merchantId}/catalogue/items/{itemId}/active
func (h *Handler) setItemActive(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body activeBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	item, err := h.items.SetActive(r.Context(), userID, merchantID(r), r.PathValue("itemId"), body.Active)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, h.renderOwnerItem(item))
}

type stockBody struct {
	Quantity int `json:"quantity"`
}

// PUT /v1/merchants/{merchantId}/catalogue/items/{itemId}/stock
func (h *Handler) setItemStock(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body stockBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	item, err := h.items.SetStock(r.Context(), userID, merchantID(r), r.PathValue("itemId"), body.Quantity)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, h.renderOwnerItem(item))
}

type availabilityBody struct {
	Always bool                `json:"always"`
	Days   map[string][]string `json:"days"`
}

// PUT /v1/merchants/{merchantId}/catalogue/items/{itemId}/availability
func (h *Handler) setItemAvailability(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body availabilityBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	item, err := h.items.SetAvailability(r.Context(), userID, merchantID(r), r.PathValue("itemId"),
		application.AvailabilityRequest{Always: body.Always, Days: body.Days})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, h.renderOwnerItem(item))
}

type optionBody struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PriceMinor int64  `json:"price_minor"`
	Available  *bool  `json:"available"`
}

type groupBody struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Required   bool         `json:"required"`
	MinChoices int          `json:"min_choices"`
	MaxChoices int          `json:"max_choices"`
	SortOrder  int          `json:"sort_order"`
	Options    []optionBody `json:"options"`
}

type groupsBody struct {
	Groups []groupBody `json:"groups"`
}

func (b groupsBody) toRequests() []application.GroupRequest {
	out := make([]application.GroupRequest, 0, len(b.Groups))
	for _, group := range b.Groups {
		options := make([]application.OptionRequest, 0, len(group.Options))
		for _, option := range group.Options {
			options = append(options, application.OptionRequest{
				ID: option.ID, Name: option.Name,
				PriceMinor: option.PriceMinor, Available: option.Available,
			})
		}
		out = append(out, application.GroupRequest{
			ID: group.ID, Name: group.Name, Required: group.Required,
			MinChoices: group.MinChoices, MaxChoices: group.MaxChoices,
			SortOrder: group.SortOrder, Options: options,
		})
	}
	return out
}

// PUT /v1/merchants/{merchantId}/catalogue/items/{itemId}/variants
func (h *Handler) setVariantGroups(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body groupsBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	item, err := h.options.SetVariantGroups(r.Context(), userID, merchantID(r),
		r.PathValue("itemId"), body.toRequests())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, h.renderOwnerItem(item))
}

// PUT /v1/merchants/{merchantId}/catalogue/items/{itemId}/addons
func (h *Handler) setAddOnGroups(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body groupsBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	item, err := h.options.SetAddOnGroups(r.Context(), userID, merchantID(r),
		r.PathValue("itemId"), body.toRequests())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, h.renderOwnerItem(item))
}

type bulkChangeBody struct {
	ItemID string `json:"item_id"`
	// Pointers, so a bulk re-pricing does not have to send stock counts back —
	// and one that forgot would not zero a shop's inventory.
	PriceMinor *int64 `json:"price_minor"`
	Stock      *int   `json:"stock_quantity"`
	Active     *bool  `json:"active"`
}

type bulkBody struct {
	Changes []bulkChangeBody `json:"changes"`
}

// POST /v1/merchants/{merchantId}/catalogue/items/bulk
func (h *Handler) bulkUpdateItems(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body bulkBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}

	changes := make([]application.BulkChange, 0, len(body.Changes))
	for _, change := range body.Changes {
		changes = append(changes, application.BulkChange{
			ItemID: change.ItemID, PriceMinor: change.PriceMinor,
			Stock: change.Stock, Active: change.Active,
		})
	}

	items, err := h.items.BulkUpdate(r.Context(), userID, merchantID(r), changes)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	out := make([]itemResponse, 0, len(items))
	for _, item := range items {
		out = append(out, h.renderOwnerItem(item))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

// ------------------------------------------------------------------ combos

type comboLineBody struct {
	ItemID   string `json:"item_id"`
	Quantity int    `json:"quantity"`
}

type comboBody struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	ImageURL    string          `json:"image_url"`
	PriceMinor  int64           `json:"price_minor"`
	SortOrder   int             `json:"sort_order"`
	Lines       []comboLineBody `json:"lines"`
}

func (b comboBody) toRequest() application.ComboRequest {
	lines := make([]application.ComboLineRequest, 0, len(b.Lines))
	for _, line := range b.Lines {
		lines = append(lines, application.ComboLineRequest{ItemID: line.ItemID, Quantity: line.Quantity})
	}
	return application.ComboRequest{
		Name: b.Name, Description: b.Description, ImageURL: b.ImageURL,
		PriceMinor: b.PriceMinor, SortOrder: b.SortOrder, Lines: lines,
	}
}

// GET /v1/merchants/{merchantId}/catalogue/combos
func (h *Handler) listCombos(w http.ResponseWriter, r *http.Request) {
	combos, err := h.combos.List(r.Context(), merchantID(r), r.URL.Query().Get("active") == "true")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	out := make([]comboResponse, 0, len(combos))
	for _, combo := range combos {
		out = append(out, renderOwnerCombo(combo))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"combos": out})
}

// POST /v1/merchants/{merchantId}/catalogue/combos
func (h *Handler) createCombo(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body comboBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	combo, err := h.combos.Create(r.Context(), userID, merchantID(r), body.toRequest())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, renderOwnerCombo(combo))
}

// PUT /v1/merchants/{merchantId}/catalogue/combos/{comboId}
func (h *Handler) updateCombo(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body comboBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	combo, err := h.combos.Update(r.Context(), userID, merchantID(r), r.PathValue("comboId"), body.toRequest())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, renderOwnerCombo(combo))
}

// DELETE /v1/merchants/{merchantId}/catalogue/combos/{comboId}
func (h *Handler) deleteCombo(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	if err := h.combos.Delete(r.Context(), userID, merchantID(r), r.PathValue("comboId")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

// POST /v1/merchants/{merchantId}/catalogue/combos/{comboId}/active
func (h *Handler) setComboActive(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body activeBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	combo, err := h.combos.SetActive(r.Context(), userID, merchantID(r), r.PathValue("comboId"), body.Active)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, renderOwnerCombo(combo))
}

// PUT /v1/merchants/{merchantId}/catalogue/combos/{comboId}/availability
func (h *Handler) setComboAvailability(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body availabilityBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	combo, err := h.combos.SetAvailability(r.Context(), userID, merchantID(r), r.PathValue("comboId"),
		application.AvailabilityRequest{Always: body.Always, Days: body.Days})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, renderOwnerCombo(combo))
}

// capabilitiesResponse tells the merchant app which fields to show.
//
// Served rather than hard-coded, for the same reason the merchant module serves
// its document requirements: a Flutter build that decided for itself that a
// pharmacy has no add-ons would be a second copy of the rule, updated on a
// different schedule (2.9).
type capabilitiesResponse struct {
	MerchantType  string   `json:"merchant_type"`
	Variants      bool     `json:"variants"`
	AddOns        bool     `json:"addons"`
	Combos        bool     `json:"combos"`
	TracksStock   bool     `json:"tracks_stock"`
	RequiresUnit  bool     `json:"requires_unit"`
	Prescriptions bool     `json:"prescriptions"`
	Units         []string `json:"units"`
}

// GET /v1/merchants/{merchantId}/catalogue/capabilities
func (h *Handler) capabilities(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	kind, err := h.categories.MerchantTypeOf(r.Context(), userID, merchantID(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	capabilities := domain.CapabilitiesFor(kind)
	units := make([]string, 0, len(domain.AllUnits()))
	if capabilities.RequiresUnit {
		for _, unit := range domain.AllUnits() {
			units = append(units, unit.String())
		}
	}
	httpx.WriteJSON(w, http.StatusOK, capabilitiesResponse{
		MerchantType:  kind.String(),
		Variants:      capabilities.Variants,
		AddOns:        capabilities.AddOns,
		Combos:        capabilities.Combos,
		TracksStock:   capabilities.TracksStock,
		RequiresUnit:  capabilities.RequiresUnit,
		Prescriptions: capabilities.Prescriptions,
		Units:         units,
	})
}

// ----------------------------------------------------- the customer's view

// GET /v1/catalogue/{merchantId}/menu
func (h *Handler) menu(w http.ResponseWriter, r *http.Request) {
	menu, err := h.service.Menu(r.Context(), merchantID(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	categories := make([]categoryResponse, 0, len(menu.Categories))
	for _, category := range menu.Categories {
		categories = append(categories, categoryResponse{
			ID: category.ID, Name: category.Name, SortOrder: category.SortOrder, Active: true,
		})
	}
	items := make([]itemResponse, 0, len(menu.Items))
	for _, item := range menu.Items {
		items = append(items, renderItem(item))
	}
	combos := make([]comboResponse, 0, len(menu.Combos))
	for _, combo := range menu.Combos {
		combos = append(combos, renderCombo(combo))
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"merchant_id": menu.MerchantID,
		"categories":  categories,
		"items":       items,
		"combos":      combos,
	})
}

// GET /v1/catalogue/{merchantId}/items/{itemId}
func (h *Handler) publicItem(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.Item(r.Context(), merchantID(r), r.PathValue("itemId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, renderItem(item))
}

// GET /v1/catalogue/{merchantId}/combos/{comboId}
func (h *Handler) publicCombo(w http.ResponseWriter, r *http.Request) {
	combo, err := h.service.Combo(r.Context(), merchantID(r), r.PathValue("comboId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, renderCombo(combo))
}

// intParam reads a query parameter as a count, treating anything unreadable as
// absent. A bad limit falls back to the default page rather than refusing the
// request: the use case clamps it anyway, and a 400 for "limit=abc" is a
// support call about a page that will not load.
func intParam(raw string) int {
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return n
}
