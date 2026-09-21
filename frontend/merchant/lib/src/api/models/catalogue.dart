import 'package:flutter/foundation.dart';
import 'package:goklay_core/goklay_core.dart';

/// What a catalogue of this shop type may contain.
///
/// **This is the whole of the app's per-type behaviour, and it is served.**
/// Whether a grocery counts a shelf, whether a restaurant may offer add-ons,
/// which units of sale to offer — all of it arrives here, so the app never
/// branches on `type == 'pharmacy'` and never goes stale when the rule does
/// (2.9).
@immutable
class CatalogueCapabilities {
  /// Creates the capabilities.
  const CatalogueCapabilities({
    required this.merchantType,
    required this.variants,
    required this.addons,
    required this.combos,
    required this.tracksStock,
    required this.requiresUnit,
    required this.prescriptions,
    required this.units,
  });

  /// Reads the `CatalogueCapabilities` response.
  factory CatalogueCapabilities.fromJson(Map<String, Object?> json) {
    final Object? units = json['units'];
    return CatalogueCapabilities(
      merchantType: readString(json, 'merchant_type'),
      variants: readBool(json, 'variants'),
      addons: readBool(json, 'addons'),
      combos: readBool(json, 'combos'),
      tracksStock: readBool(json, 'tracks_stock'),
      requiresUnit: readBool(json, 'requires_unit'),
      prescriptions: readBool(json, 'prescriptions'),
      units: <String>[
        if (units is List<Object?>)
          for (final Object? unit in units)
            if (unit is String) unit,
      ],
    );
  }

  /// Which shop type these are for.
  final String merchantType;

  /// Whether items may have variant groups.
  final bool variants;

  /// Whether items may have add-on groups. Restaurants only.
  final bool addons;

  /// Whether the shop may sell bundles.
  final bool combos;

  /// Whether the shop counts a shelf. Grocery and pharmacy.
  final bool tracksStock;

  /// Whether an item must state a unit of sale.
  final bool requiresUnit;

  /// Whether an item may be marked prescription-only. Pharmacy.
  final bool prescriptions;

  /// The units of sale to offer. Empty when this type has none.
  final List<String> units;
}

/// A section of the shop's own menu.
@immutable
class OwnerCategory {
  /// Creates a category.
  const OwnerCategory({
    required this.id,
    required this.name,
    required this.sortOrder,
    required this.active,
  });

  /// Reads the `Category` object.
  factory OwnerCategory.fromJson(Map<String, Object?> json) => OwnerCategory(
    id: readString(json, 'id'),
    name: readString(json, 'name'),
    sortOrder: readInt(json, 'sort_order'),
    active: readBool(json, 'active'),
  );

  /// The section's id.
  final String id;

  /// Its name.
  final String name;

  /// Where it sits in the shop's own order.
  final int sortOrder;

  /// Whether it is on the menu. A hidden section takes its items with it,
  /// which is the server's rule and not one the app re-applies.
  final bool active;
}

/// An item, as its owner sees it: with the shelf count and the switch.
@immutable
class OwnerItem {
  /// Creates an item.
  const OwnerItem({
    required this.id,
    required this.categoryId,
    required this.name,
    required this.description,
    required this.price,
    required this.orderable,
    required this.unavailableReason,
    required this.active,
    required this.stockTracked,
    required this.stockQuantity,
    required this.unit,
    required this.packSize,
    required this.brand,
    required this.requiresPrescription,
  });

  /// Reads the `Item` object.
  factory OwnerItem.fromJson(Map<String, Object?> json) => OwnerItem(
    id: readString(json, 'id'),
    categoryId: readString(json, 'category_id'),
    name: readString(json, 'name'),
    description: readString(json, 'description'),
    price: Money.fromJson(readObject(json, 'price')),
    orderable: readBool(json, 'orderable'),
    unavailableReason: readOptionalString(json, 'unavailable_reason'),
    active: readBool(json, 'active'),
    stockTracked: readBool(json, 'stock_tracked'),
    stockQuantity: json['stock_quantity'] == null
        ? null
        : readInt(json, 'stock_quantity'),
    unit: readString(json, 'unit'),
    packSize: readString(json, 'pack_size'),
    brand: readString(json, 'brand'),
    requiresPrescription: readBool(json, 'requires_prescription'),
  );

  /// The item's id.
  final String id;

  /// The section it sits in.
  final String categoryId;

  /// Its name.
  final String name;

  /// The shop's description.
  final String description;

  /// What it sells for.
  final Money price;

  /// **Whether a customer can order it right now**, folding together the
  /// switch, the shelf and the schedule. The owner's screen shows this rather
  /// than working it out from [active] and [stockQuantity], because the
  /// customer's app is shown exactly this flag.
  final bool orderable;

  /// Why not, as a code. Null when it is orderable.
  final String? unavailableReason;

  /// The owner's own on/off switch.
  final bool active;

  /// Whether this shop counts a shelf for this item.
  final bool stockTracked;

  /// How many are on the shelf, or null when nothing is counted.
  final int? stockQuantity;

  /// How it is sold.
  final String unit;

  /// The pack size as written on the pack.
  final String packSize;

  /// The brand.
  final String brand;

  /// Whether the shop needs a prescription before dispensing it.
  final bool requiresPrescription;
}

/// A bundle, as its owner sees it.
@immutable
class OwnerCombo {
  /// Creates a combo.
  const OwnerCombo({
    required this.id,
    required this.name,
    required this.price,
    required this.orderable,
    required this.active,
    required this.lineNames,
  });

  /// Reads the `Combo` object.
  factory OwnerCombo.fromJson(Map<String, Object?> json) {
    final Object? lines = json['lines'];
    return OwnerCombo(
      id: readString(json, 'id'),
      name: readString(json, 'name'),
      price: Money.fromJson(readObject(json, 'price')),
      orderable: readBool(json, 'orderable'),
      active: readBool(json, 'active'),
      lineNames: <String>[
        if (lines is List<Object?>)
          for (final Object? line in lines)
            if (line is Map<String, Object?>) readString(line, 'name'),
      ],
    );
  }

  /// The combo's id.
  final String id;

  /// Its name.
  final String name;

  /// The bundle price.
  final Money price;

  /// Whether every member item is available. The server decides; the owner's
  /// screen does not check the members itself.
  final bool orderable;

  /// The owner's own switch.
  final bool active;

  /// What is in it, by name.
  final List<String> lineNames;
}
