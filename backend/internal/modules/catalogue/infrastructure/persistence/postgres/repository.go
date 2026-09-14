// Package postgres stores a shop's catalogue.
//
// Engine-specific SQL is confined to this package (05-architecture.md 2.4).
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// Querier is the read and write surface this repository needs.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// TxBeginner starts a transaction.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Repository stores categories, items and combos.
type Repository struct {
	db Querier
	tx TxBeginner
}

// New builds a repository.
func New(db Querier, tx TxBeginner) *Repository { return &Repository{db: db, tx: tx} }

// NewFromPool is the production constructor.
func NewFromPool(pool *pgxpool.Pool) *Repository { return &Repository{db: pool, tx: pool} }

// ------------------------------------------------------------- categories

// Categories returns a shop's categories in the owner's own order.
func (r *Repository) Categories(ctx context.Context, merchantID string) ([]domain.Category, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, merchant_id, name, sort_order, active
		FROM catalogue_categories WHERE merchant_id = $1
		ORDER BY sort_order, id`, merchantID)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	var out []domain.Category
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(&c.ID, &c.MerchantID, &c.Name, &c.SortOrder, &c.Active); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read categories: %w", err)
	}
	return out, nil
}

// Category returns one category belonging to a shop.
func (r *Repository) Category(ctx context.Context, merchantID, categoryID string) (domain.Category, error) {
	// The merchant id is in the WHERE clause, not checked afterwards. A query
	// that can return another shop's row is one careless call away from letting
	// somebody file an item under a competitor's menu.
	var c domain.Category
	err := r.db.QueryRow(ctx, `
		SELECT id, merchant_id, name, sort_order, active
		FROM catalogue_categories WHERE merchant_id = $1 AND id = $2`,
		merchantID, categoryID).Scan(&c.ID, &c.MerchantID, &c.Name, &c.SortOrder, &c.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Category{}, domain.ErrCategoryNotFound
	}
	if err != nil {
		return domain.Category{}, fmt.Errorf("read category: %w", err)
	}
	return c, nil
}

// SaveCategory writes a category, creating it if absent.
func (r *Repository) SaveCategory(ctx context.Context, c domain.Category) error {
	if _, err := r.db.Exec(ctx, `
		INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE
		    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
		        active = EXCLUDED.active, updated_at = now()`,
		c.ID, c.MerchantID, c.Name, c.SortOrder, c.Active); err != nil {
		return fmt.Errorf("save category: %w", err)
	}
	return nil
}

// DeleteCategory removes a category.
func (r *Repository) DeleteCategory(ctx context.Context, merchantID, categoryID string) error {
	if _, err := r.db.Exec(ctx,
		`DELETE FROM catalogue_categories WHERE merchant_id = $1 AND id = $2`,
		merchantID, categoryID); err != nil {
		return fmt.Errorf("delete category: %w", err)
	}
	return nil
}

// ------------------------------------------------------------------ items

// itemColumns is the projection every item read shares, so one added column
// cannot be forgotten in one of four places.
const itemColumns = `
	id, merchant_id, category_id, merchant_type, name, description, image_url,
	price_minor, stock_tracked, stock_quantity,
	unit, pack_size, brand, generic_name, strength, requires_prescription,
	is_vegetarian, preparation_minutes, active, availability, sort_order`

// scanItem reads one row, option groups excluded.
func scanItem(row pgx.Row) (domain.Item, error) {
	var (
		i            domain.Item
		kind, unit   string
		priceMinor   int64
		availability []byte
	)

	if err := row.Scan(&i.ID, &i.MerchantID, &i.CategoryID, &kind, &i.Name, &i.Description, &i.ImageURL,
		&priceMinor, &i.Stock.Tracked, &i.Stock.Quantity,
		&unit, &i.Attributes.PackSize, &i.Attributes.Brand,
		&i.Attributes.GenericName, &i.Attributes.Strength, &i.Attributes.RequiresPrescription,
		&i.Attributes.IsVegetarian, &i.Attributes.PreparationMinutes,
		&i.Active, &availability, &i.SortOrder); err != nil {
		return domain.Item{}, err
	}

	merchantType, err := domain.ParseMerchantType(kind)
	if err != nil {
		return domain.Item{}, fmt.Errorf("item %s: %w", i.ID, err)
	}
	i.MerchantType = merchantType
	i.Price = money.Taka(priceMinor)

	if unit != "" {
		parsed, unitErr := domain.ParseUnit(unit)
		if unitErr != nil {
			return domain.Item{}, fmt.Errorf("item %s: %w", i.ID, unitErr)
		}
		i.Attributes.Unit = parsed
	}

	var encoded map[string][]string
	if err := json.Unmarshal(availability, &encoded); err != nil {
		return domain.Item{}, fmt.Errorf("item %s availability: %w", i.ID, err)
	}
	decoded, err := domain.DecodeAvailability(encoded)
	if err != nil {
		return domain.Item{}, fmt.Errorf("item %s availability: %w", i.ID, err)
	}
	i.Availability = decoded
	return i, nil
}

// Items returns items matching a filter.
func (r *Repository) Items(ctx context.Context, f ports.ItemFilter) ([]domain.Item, error) {
	// Every clause is a placeholder rather than interpolated text: a search box
	// that concatenated its term into SQL would be an injection on the busiest
	// endpoint in the product.
	clauses := []string{"merchant_id = $1"}
	args := []any{f.MerchantID}

	if f.CategoryID != "" {
		args = append(args, f.CategoryID)
		clauses = append(clauses, fmt.Sprintf("category_id = $%d", len(args)))
	}
	if f.ActiveOnly {
		clauses = append(clauses, "active")
	}
	if search := strings.TrimSpace(f.Search); search != "" {
		args = append(args, "%"+strings.ToLower(search)+"%")
		clauses = append(clauses, fmt.Sprintf("lower(name) LIKE $%d", len(args)))
	}
	args = append(args, f.Limit, f.Offset)

	rows, err := r.db.Query(ctx, fmt.Sprintf(
		`SELECT %s FROM catalogue_items WHERE %s ORDER BY sort_order, id LIMIT $%d OFFSET $%d`,
		itemColumns, strings.Join(clauses, " AND "), len(args)-1, len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	defer rows.Close()

	var found []domain.Item
	for rows.Next() {
		item, scanErr := scanItem(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan item: %w", scanErr)
		}
		found = append(found, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read items: %w", err)
	}
	return r.attachOptions(ctx, found)
}

// Item returns one item belonging to a shop.
func (r *Repository) Item(ctx context.Context, merchantID, itemID string) (domain.Item, error) {
	item, err := scanItem(r.db.QueryRow(ctx,
		`SELECT `+itemColumns+` FROM catalogue_items WHERE merchant_id = $1 AND id = $2`,
		merchantID, itemID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Item{}, domain.ErrItemNotFound
	}
	if err != nil {
		return domain.Item{}, fmt.Errorf("read item: %w", err)
	}

	withOptions, err := r.attachOptions(ctx, []domain.Item{item})
	if err != nil {
		return domain.Item{}, err
	}
	return withOptions[0], nil
}

// ItemsByID returns several items of one shop at once.
func (r *Repository) ItemsByID(ctx context.Context, merchantID string, itemIDs []string) ([]domain.Item, error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}

	rows, err := r.db.Query(ctx,
		`SELECT `+itemColumns+` FROM catalogue_items
		 WHERE merchant_id = $1 AND id = ANY($2) ORDER BY sort_order, id`,
		merchantID, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("read items: %w", err)
	}
	defer rows.Close()

	var found []domain.Item
	for rows.Next() {
		item, scanErr := scanItem(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan item: %w", scanErr)
		}
		found = append(found, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read items: %w", err)
	}
	return r.attachOptions(ctx, found)
}

// attachOptions loads the variant and add-on groups for a set of items.
//
// After the item rows are closed, not inside the loop: pgx holds one connection
// per open result set, and a nested query on the same connection would deadlock
// against the pool under load.
func (r *Repository) attachOptions(ctx context.Context, items []domain.Item) ([]domain.Item, error) {
	if len(items) == 0 {
		return items, nil
	}

	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}

	variants, err := r.variantGroups(ctx, ids)
	if err != nil {
		return nil, err
	}
	addOns, err := r.addOnGroups(ctx, ids)
	if err != nil {
		return nil, err
	}

	for i := range items {
		items[i].VariantGroups = variants[items[i].ID]
		items[i].AddOnGroups = addOns[items[i].ID]
	}
	return items, nil
}

// variantGroups reads every variant group and option for a set of items in two
// queries rather than two per item.
func (r *Repository) variantGroups(ctx context.Context, itemIDs []string) (map[string][]domain.VariantGroup, error) {
	rows, err := r.db.Query(ctx, `
		SELECT g.item_id, g.id, g.name, g.required, g.min_choices, g.max_choices, g.sort_order,
		       o.id, o.name, o.price_delta_minor, o.available
		FROM catalogue_variant_groups g
		LEFT JOIN catalogue_variant_options o ON o.group_id = g.id
		WHERE g.item_id = ANY($1)
		ORDER BY g.item_id, g.sort_order, g.id, o.sort_order, o.id`, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("read variant groups: %w", err)
	}
	defer rows.Close()

	out := make(map[string][]domain.VariantGroup)
	// index remembers where each group sits in its item's slice, so options
	// join onto the group they belong to without a second pass.
	index := make(map[string]int)

	for rows.Next() {
		var (
			itemID, groupID, groupName string
			required                   bool
			minChoices, maxChoices     int
			sortOrder                  int
			optionID, optionName       *string
			priceDelta                 *int64
			available                  *bool
		)
		if err := rows.Scan(&itemID, &groupID, &groupName, &required, &minChoices, &maxChoices, &sortOrder,
			&optionID, &optionName, &priceDelta, &available); err != nil {
			return nil, fmt.Errorf("scan variant group: %w", err)
		}

		position, seen := index[groupID]
		if !seen {
			out[itemID] = append(out[itemID], domain.VariantGroup{
				ID: groupID, Name: groupName, Required: required,
				MinChoices: minChoices, MaxChoices: maxChoices, SortOrder: sortOrder,
			})
			position = len(out[itemID]) - 1
			index[groupID] = position
		}

		// A LEFT JOIN, so a group with no options yields one row of NULLs. That
		// state should not exist — the domain refuses an empty group — but a
		// row written before that rule, or by hand, must not crash a menu.
		if optionID == nil {
			continue
		}
		out[itemID][position].Options = append(out[itemID][position].Options, domain.VariantOption{
			ID: *optionID, Name: *optionName,
			PriceDelta: money.Taka(*priceDelta), Available: *available,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read variant groups: %w", err)
	}
	return out, nil
}

// addOnGroups does the same for extras.
func (r *Repository) addOnGroups(ctx context.Context, itemIDs []string) (map[string][]domain.AddOnGroup, error) {
	rows, err := r.db.Query(ctx, `
		SELECT g.item_id, g.id, g.name, g.min_choices, g.max_choices, g.sort_order,
		       o.id, o.name, o.price_minor, o.available
		FROM catalogue_addon_groups g
		LEFT JOIN catalogue_addon_options o ON o.group_id = g.id
		WHERE g.item_id = ANY($1)
		ORDER BY g.item_id, g.sort_order, g.id, o.sort_order, o.id`, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("read add-on groups: %w", err)
	}
	defer rows.Close()

	out := make(map[string][]domain.AddOnGroup)
	index := make(map[string]int)

	for rows.Next() {
		var (
			itemID, groupID, groupName string
			minChoices, maxChoices     int
			sortOrder                  int
			optionID, optionName       *string
			price                      *int64
			available                  *bool
		)
		if err := rows.Scan(&itemID, &groupID, &groupName, &minChoices, &maxChoices, &sortOrder,
			&optionID, &optionName, &price, &available); err != nil {
			return nil, fmt.Errorf("scan add-on group: %w", err)
		}

		position, seen := index[groupID]
		if !seen {
			out[itemID] = append(out[itemID], domain.AddOnGroup{
				ID: groupID, Name: groupName,
				MinChoices: minChoices, MaxChoices: maxChoices, SortOrder: sortOrder,
			})
			position = len(out[itemID]) - 1
			index[groupID] = position
		}
		if optionID == nil {
			continue
		}
		out[itemID][position].Options = append(out[itemID][position].Options, domain.AddOn{
			ID: *optionID, Name: *optionName,
			Price: money.Taka(*price), Available: *available,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read add-on groups: %w", err)
	}
	return out, nil
}

// SaveItem writes an item and its option groups in one transaction.
func (r *Repository) SaveItem(ctx context.Context, i domain.Item) error {
	return r.inTx(ctx, func(tx pgx.Tx) error { return writeItem(ctx, tx, i) })
}

// SaveItems writes several items in one transaction.
//
// One transaction because a half-applied bulk update is worse than none: a menu
// where the first thirty items moved and the rest did not is a shop selling at
// two price lists, and nobody can tell where the boundary fell.
func (r *Repository) SaveItems(ctx context.Context, items []domain.Item) error {
	return r.inTx(ctx, func(tx pgx.Tx) error {
		for _, item := range items {
			if err := writeItem(ctx, tx, item); err != nil {
				return err
			}
		}
		return nil
	})
}

// writeItem upserts an item row and replaces its option groups.
func writeItem(ctx context.Context, q Querier, i domain.Item) error {
	// Encode returns map[string][]string, whose every value is a string, so
	// there is no type encoding/json can refuse and no error branch to reach.
	availability, _ := json.Marshal(i.Availability.Encode()) //nolint:errchkjson // map[string][]string cannot fail
	if len(i.Availability.Encode()) == 0 {
		availability = []byte(`{}`)
	}

	if _, err := q.Exec(ctx, `
		INSERT INTO catalogue_items (
		    id, merchant_id, category_id, merchant_type, name, description, image_url,
		    price_minor, stock_tracked, stock_quantity,
		    unit, pack_size, brand, generic_name, strength, requires_prescription,
		    is_vegetarian, preparation_minutes, active, availability, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
		        $17, $18, $19, $20, $21)
		ON CONFLICT (id) DO UPDATE
		    SET category_id = EXCLUDED.category_id,
		        name = EXCLUDED.name,
		        description = EXCLUDED.description,
		        image_url = EXCLUDED.image_url,
		        price_minor = EXCLUDED.price_minor,
		        stock_tracked = EXCLUDED.stock_tracked,
		        stock_quantity = EXCLUDED.stock_quantity,
		        unit = EXCLUDED.unit,
		        pack_size = EXCLUDED.pack_size,
		        brand = EXCLUDED.brand,
		        generic_name = EXCLUDED.generic_name,
		        strength = EXCLUDED.strength,
		        requires_prescription = EXCLUDED.requires_prescription,
		        is_vegetarian = EXCLUDED.is_vegetarian,
		        preparation_minutes = EXCLUDED.preparation_minutes,
		        active = EXCLUDED.active,
		        availability = EXCLUDED.availability,
		        sort_order = EXCLUDED.sort_order,
		        updated_at = now()`,
		i.ID, i.MerchantID, i.CategoryID, i.MerchantType.String(), i.Name, i.Description, i.ImageURL,
		i.Price.Minor(), i.Stock.Tracked, i.Stock.Quantity,
		i.Attributes.Unit.String(), i.Attributes.PackSize, i.Attributes.Brand,
		i.Attributes.GenericName, i.Attributes.Strength, i.Attributes.RequiresPrescription,
		i.Attributes.IsVegetarian, i.Attributes.PreparationMinutes,
		i.Active, availability, i.SortOrder); err != nil {
		return fmt.Errorf("save item %s: %w", i.ID, err)
	}

	return writeOptionGroups(ctx, q, i)
}

// writeOptionGroups replaces an item's groups with the set it now carries.
//
// Deleting what is absent rather than only upserting what is present: the domain
// is the authority on which groups an item has, and a group left behind from
// before an edit is a choice a customer would still be offered.
func writeOptionGroups(ctx context.Context, q Querier, i domain.Item) error {
	variantIDs := make([]string, 0, len(i.VariantGroups))
	for _, group := range i.VariantGroups {
		variantIDs = append(variantIDs, group.ID)
	}
	if _, err := q.Exec(ctx,
		`DELETE FROM catalogue_variant_groups WHERE item_id = $1 AND NOT (id = ANY($2))`,
		i.ID, variantIDs); err != nil {
		return fmt.Errorf("prune variant groups: %w", err)
	}

	for _, group := range i.VariantGroups {
		if _, err := q.Exec(ctx, `
			INSERT INTO catalogue_variant_groups (id, item_id, name, required, min_choices, max_choices, sort_order)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO UPDATE
			    SET name = EXCLUDED.name, required = EXCLUDED.required,
			        min_choices = EXCLUDED.min_choices, max_choices = EXCLUDED.max_choices,
			        sort_order = EXCLUDED.sort_order`,
			group.ID, i.ID, group.Name, group.Required,
			group.MinChoices, group.MaxChoices, group.SortOrder); err != nil {
			return fmt.Errorf("save variant group %s: %w", group.ID, err)
		}

		optionIDs := make([]string, 0, len(group.Options))
		for _, option := range group.Options {
			optionIDs = append(optionIDs, option.ID)
		}
		if _, err := q.Exec(ctx,
			`DELETE FROM catalogue_variant_options WHERE group_id = $1 AND NOT (id = ANY($2))`,
			group.ID, optionIDs); err != nil {
			return fmt.Errorf("prune variant options: %w", err)
		}
		for position, option := range group.Options {
			if _, err := q.Exec(ctx, `
				INSERT INTO catalogue_variant_options (id, group_id, name, price_delta_minor, available, sort_order)
				VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (id) DO UPDATE
				    SET name = EXCLUDED.name, price_delta_minor = EXCLUDED.price_delta_minor,
				        available = EXCLUDED.available, sort_order = EXCLUDED.sort_order`,
				option.ID, group.ID, option.Name, option.PriceDelta.Minor(),
				option.Available, position); err != nil {
				return fmt.Errorf("save variant option %s: %w", option.ID, err)
			}
		}
	}

	addOnIDs := make([]string, 0, len(i.AddOnGroups))
	for _, group := range i.AddOnGroups {
		addOnIDs = append(addOnIDs, group.ID)
	}
	if _, err := q.Exec(ctx,
		`DELETE FROM catalogue_addon_groups WHERE item_id = $1 AND NOT (id = ANY($2))`,
		i.ID, addOnIDs); err != nil {
		return fmt.Errorf("prune add-on groups: %w", err)
	}

	for _, group := range i.AddOnGroups {
		if _, err := q.Exec(ctx, `
			INSERT INTO catalogue_addon_groups (id, item_id, name, min_choices, max_choices, sort_order)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (id) DO UPDATE
			    SET name = EXCLUDED.name, min_choices = EXCLUDED.min_choices,
			        max_choices = EXCLUDED.max_choices, sort_order = EXCLUDED.sort_order`,
			group.ID, i.ID, group.Name, group.MinChoices, group.MaxChoices, group.SortOrder); err != nil {
			return fmt.Errorf("save add-on group %s: %w", group.ID, err)
		}

		optionIDs := make([]string, 0, len(group.Options))
		for _, option := range group.Options {
			optionIDs = append(optionIDs, option.ID)
		}
		if _, err := q.Exec(ctx,
			`DELETE FROM catalogue_addon_options WHERE group_id = $1 AND NOT (id = ANY($2))`,
			group.ID, optionIDs); err != nil {
			return fmt.Errorf("prune add-on options: %w", err)
		}
		for position, option := range group.Options {
			if _, err := q.Exec(ctx, `
				INSERT INTO catalogue_addon_options (id, group_id, name, price_minor, available, sort_order)
				VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (id) DO UPDATE
				    SET name = EXCLUDED.name, price_minor = EXCLUDED.price_minor,
				        available = EXCLUDED.available, sort_order = EXCLUDED.sort_order`,
				option.ID, group.ID, option.Name, option.Price.Minor(),
				option.Available, position); err != nil {
				return fmt.Errorf("save add-on option %s: %w", option.ID, err)
			}
		}
	}
	return nil
}

// DeleteItem removes an item and everything hanging off it.
func (r *Repository) DeleteItem(ctx context.Context, merchantID, itemID string) error {
	if _, err := r.db.Exec(ctx,
		`DELETE FROM catalogue_items WHERE merchant_id = $1 AND id = $2`,
		merchantID, itemID); err != nil {
		return fmt.Errorf("delete item: %w", err)
	}
	return nil
}

// CountItemsInCategory reports how many items point at a category.
func (r *Repository) CountItemsInCategory(ctx context.Context, merchantID, categoryID string) (int, error) {
	var n int
	if err := r.db.QueryRow(ctx,
		`SELECT count(*) FROM catalogue_items WHERE merchant_id = $1 AND category_id = $2`,
		merchantID, categoryID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count items in category: %w", err)
	}
	return n, nil
}

// ----------------------------------------------------------------- combos

const comboColumns = `
	id, merchant_id, merchant_type, name, description, image_url,
	price_minor, active, availability, sort_order`

// scanCombo reads one combo row, lines excluded.
func scanCombo(row pgx.Row) (domain.Combo, error) {
	var (
		c            domain.Combo
		kind         string
		priceMinor   int64
		availability []byte
	)
	if err := row.Scan(&c.ID, &c.MerchantID, &kind, &c.Name, &c.Description, &c.ImageURL,
		&priceMinor, &c.Active, &availability, &c.SortOrder); err != nil {
		return domain.Combo{}, err
	}

	merchantType, err := domain.ParseMerchantType(kind)
	if err != nil {
		return domain.Combo{}, fmt.Errorf("combo %s: %w", c.ID, err)
	}
	c.MerchantType = merchantType
	c.Price = money.Taka(priceMinor)

	var encoded map[string][]string
	if err := json.Unmarshal(availability, &encoded); err != nil {
		return domain.Combo{}, fmt.Errorf("combo %s availability: %w", c.ID, err)
	}
	decoded, err := domain.DecodeAvailability(encoded)
	if err != nil {
		return domain.Combo{}, fmt.Errorf("combo %s availability: %w", c.ID, err)
	}
	c.Availability = decoded
	return c, nil
}

// Combos returns a shop's combos in the owner's own order.
func (r *Repository) Combos(ctx context.Context, merchantID string, activeOnly bool) ([]domain.Combo, error) {
	where := "merchant_id = $1"
	if activeOnly {
		where += " AND active"
	}

	rows, err := r.db.Query(ctx,
		`SELECT `+comboColumns+` FROM catalogue_combos WHERE `+where+` ORDER BY sort_order, id`,
		merchantID)
	if err != nil {
		return nil, fmt.Errorf("list combos: %w", err)
	}
	defer rows.Close()

	var found []domain.Combo
	for rows.Next() {
		combo, scanErr := scanCombo(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan combo: %w", scanErr)
		}
		found = append(found, combo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read combos: %w", err)
	}
	return r.attachLines(ctx, found)
}

// Combo returns one combo belonging to a shop.
func (r *Repository) Combo(ctx context.Context, merchantID, comboID string) (domain.Combo, error) {
	combo, err := scanCombo(r.db.QueryRow(ctx,
		`SELECT `+comboColumns+` FROM catalogue_combos WHERE merchant_id = $1 AND id = $2`,
		merchantID, comboID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Combo{}, domain.ErrComboNotFound
	}
	if err != nil {
		return domain.Combo{}, fmt.Errorf("read combo: %w", err)
	}

	withLines, err := r.attachLines(ctx, []domain.Combo{combo})
	if err != nil {
		return domain.Combo{}, err
	}
	return withLines[0], nil
}

// attachLines loads the membership for a set of combos, after their rows are
// closed.
func (r *Repository) attachLines(ctx context.Context, combos []domain.Combo) ([]domain.Combo, error) {
	if len(combos) == 0 {
		return combos, nil
	}

	ids := make([]string, 0, len(combos))
	for _, combo := range combos {
		ids = append(ids, combo.ID)
	}

	rows, err := r.db.Query(ctx, `
		SELECT combo_id, item_id, quantity FROM catalogue_combo_lines
		WHERE combo_id = ANY($1) ORDER BY combo_id, item_id`, ids)
	if err != nil {
		return nil, fmt.Errorf("read combo lines: %w", err)
	}
	defer rows.Close()

	byCombo := make(map[string][]domain.ComboLine, len(combos))
	for rows.Next() {
		var comboID string
		var line domain.ComboLine
		if err := rows.Scan(&comboID, &line.ItemID, &line.Quantity); err != nil {
			return nil, fmt.Errorf("scan combo line: %w", err)
		}
		byCombo[comboID] = append(byCombo[comboID], line)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read combo lines: %w", err)
	}

	for i := range combos {
		combos[i].Lines = byCombo[combos[i].ID]
	}
	return combos, nil
}

// SaveCombo writes a combo and its lines in one transaction.
func (r *Repository) SaveCombo(ctx context.Context, c domain.Combo) error {
	return r.inTx(ctx, func(tx pgx.Tx) error {
		availability, _ := json.Marshal(c.Availability.Encode()) //nolint:errchkjson // map[string][]string cannot fail
		if len(c.Availability.Encode()) == 0 {
			availability = []byte(`{}`)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO catalogue_combos (
			    id, merchant_id, merchant_type, name, description, image_url,
			    price_minor, active, availability, sort_order)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (id) DO UPDATE
			    SET name = EXCLUDED.name, description = EXCLUDED.description,
			        image_url = EXCLUDED.image_url, price_minor = EXCLUDED.price_minor,
			        active = EXCLUDED.active, availability = EXCLUDED.availability,
			        sort_order = EXCLUDED.sort_order, updated_at = now()`,
			c.ID, c.MerchantID, c.MerchantType.String(), c.Name, c.Description, c.ImageURL,
			c.Price.Minor(), c.Active, availability, c.SortOrder); err != nil {
			return fmt.Errorf("save combo: %w", err)
		}

		itemIDs := make([]string, 0, len(c.Lines))
		for _, line := range c.Lines {
			itemIDs = append(itemIDs, line.ItemID)
		}
		if _, err := tx.Exec(ctx,
			`DELETE FROM catalogue_combo_lines WHERE combo_id = $1 AND NOT (item_id = ANY($2))`,
			c.ID, itemIDs); err != nil {
			return fmt.Errorf("prune combo lines: %w", err)
		}

		for _, line := range c.Lines {
			if _, err := tx.Exec(ctx, `
				INSERT INTO catalogue_combo_lines (combo_id, item_id, quantity)
				VALUES ($1, $2, $3)
				ON CONFLICT (combo_id, item_id) DO UPDATE SET quantity = EXCLUDED.quantity`,
				c.ID, line.ItemID, line.Quantity); err != nil {
				return fmt.Errorf("save combo line %s: %w", line.ItemID, err)
			}
		}
		return nil
	})
}

// DeleteCombo removes a combo. Its lines cascade; the items are untouched.
func (r *Repository) DeleteCombo(ctx context.Context, merchantID, comboID string) error {
	if _, err := r.db.Exec(ctx,
		`DELETE FROM catalogue_combos WHERE merchant_id = $1 AND id = $2`,
		merchantID, comboID); err != nil {
		return fmt.Errorf("delete combo: %w", err)
	}
	return nil
}

// inTx runs fn in a transaction, rolling back on any failure.
func (r *Repository) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.tx.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	if err := fn(tx); err != nil {
		// The rollback error is discarded: the original failure is what the
		// caller needs, and reporting a rollback problem instead would hide it.
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
