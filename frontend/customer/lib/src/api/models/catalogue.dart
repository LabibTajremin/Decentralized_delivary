import 'package:flutter/foundation.dart';
import 'package:goklay_core/goklay_core.dart';

import '../json.dart';

/// One section of a shop's menu.
@immutable
class Category {
  /// Creates a category.
  const Category({required this.id, required this.name, required this.sortOrder});

  /// Reads the `Category` object.
  factory Category.fromJson(Map<String, Object?> json) => Category(
    id: readString(json, 'id'),
    name: readString(json, 'name'),
    sortOrder: readInt(json, 'sort_order'),
  );

  /// The category's id, which an item points at.
  final String id;

  /// Its name, in the caller's language.
  final String name;

  /// Where it sits in the shop's own order. The server sends them sorted;
  /// this is kept so a test can prove the app did not re-sort them.
  final int sortOrder;
}

/// One choice inside a variant or add-on group.
@immutable
class ItemOption {
  /// Creates an option.
  const ItemOption({
    required this.id,
    required this.name,
    required this.price,
    required this.available,
  });

  /// Reads the `Option` object.
  factory ItemOption.fromJson(Map<String, Object?> json) => ItemOption(
    id: readString(json, 'id'),
    name: readString(json, 'name'),
    price: Money.fromJson(readObject(json, 'price')),
    available: readBool(json, 'available'),
  );

  /// The option's id — the only thing the client ever sends back.
  final String id;

  /// What it is called.
  final String name;

  /// What it adds, already formatted. Shown, never added up here: the cart
  /// endpoint returns the line total the server worked out (2.9).
  final Money price;

  /// Whether it can be chosen right now.
  final bool available;
}

/// A set of choices — "Size", "Extras" — with the rules for picking from it.
///
/// [isRequired], [minChoices] and [maxChoices] are the shop's rules, and the
/// server enforces them on `POST /v1/cart/items` regardless of what the app
/// allows. The app uses them to shape the picker; the answer to "is this
/// selection legal" still comes from the server's response.
@immutable
class OptionGroup {
  /// Creates a group.
  const OptionGroup({
    required this.id,
    required this.name,
    required this.isRequired,
    required this.minChoices,
    required this.maxChoices,
    required this.options,
  });

  /// Reads the `OptionGroup` object.
  factory OptionGroup.fromJson(Map<String, Object?> json) => OptionGroup(
    id: readString(json, 'id'),
    name: readString(json, 'name'),
    isRequired: readBool(json, 'required'),
    minChoices: readInt(json, 'min_choices'),
    maxChoices: readInt(json, 'max_choices'),
    options: readList(json, 'options', ItemOption.fromJson),
  );

  /// The group's id, sent alongside each chosen option.
  final String id;

  /// Its heading.
  final String name;

  /// Whether a choice must be made before the item can be added.
  final bool isRequired;

  /// The fewest choices the shop accepts.
  final int minChoices;

  /// The most it accepts. One means a radio group; more means checkboxes.
  final int maxChoices;

  /// The choices.
  final List<ItemOption> options;

  /// Whether this group takes more than one choice.
  bool get isMultiSelect => maxChoices > 1;
}

/// An item as a customer sees it: no shelf count, no visibility switch.
@immutable
class PublicItem {
  /// Creates an item.
  const PublicItem({
    required this.id,
    required this.merchantId,
    required this.categoryId,
    required this.name,
    required this.description,
    required this.imageUrl,
    required this.price,
    required this.orderable,
    required this.unavailableReason,
    required this.unit,
    required this.packSize,
    required this.brand,
    required this.genericName,
    required this.strength,
    required this.requiresPrescription,
    required this.isVegetarian,
    required this.preparationMinutes,
    required this.variantGroups,
    required this.addonGroups,
  });

  /// Reads the `PublicItem` object.
  factory PublicItem.fromJson(Map<String, Object?> json) => PublicItem(
    id: readString(json, 'id'),
    merchantId: readString(json, 'merchant_id'),
    categoryId: readString(json, 'category_id'),
    name: readString(json, 'name'),
    description: readString(json, 'description'),
    imageUrl: readString(json, 'image_url'),
    price: Money.fromJson(readObject(json, 'price')),
    orderable: readBool(json, 'orderable'),
    unavailableReason: readOptionalString(json, 'unavailable_reason'),
    unit: readString(json, 'unit'),
    packSize: readString(json, 'pack_size'),
    brand: readString(json, 'brand'),
    genericName: readString(json, 'generic_name'),
    strength: readString(json, 'strength'),
    requiresPrescription: readBool(json, 'requires_prescription'),
    isVegetarian: readBool(json, 'is_vegetarian'),
    preparationMinutes: readInt(json, 'preparation_minutes'),
    variantGroups: readList(json, 'variant_groups', OptionGroup.fromJson),
    addonGroups: readList(json, 'addon_groups', OptionGroup.fromJson),
  );

  /// The item's id.
  final String id;

  /// The shop it belongs to.
  final String merchantId;

  /// The section it sits in.
  final String categoryId;

  /// Its name.
  final String name;

  /// The shop's description of it.
  final String description;

  /// A pre-sized image URL, or empty.
  final String imageUrl;

  /// The price, already formatted.
  final Money price;

  /// **Whether "add to cart" is live.**
  ///
  /// The server folds together the item's own switch, its stock and its
  /// schedule. An app that inferred this from a stock number would disagree
  /// with the cart endpoint, which is the one that actually decides.
  final bool orderable;

  /// A machine code — `out_of_stock`, `not_available_now`, `unavailable` —
  /// present only when [orderable] is false. For branching, not printing.
  final String? unavailableReason;

  /// How it is sold: `piece`, `kg`, `strip`. Empty for prepared food.
  final String unit;

  /// The pack size as written on the pack.
  final String packSize;

  /// The brand, for grocery and pharmacy.
  final String brand;

  /// The generic drug name, for pharmacy.
  final String genericName;

  /// The dose, for pharmacy.
  final String strength;

  /// Whether the shop needs a prescription before it will dispense this.
  final bool requiresPrescription;

  /// Whether it is vegetarian, for food.
  final bool isVegetarian;

  /// How long the shop says it takes to prepare.
  final int preparationMinutes;

  /// Choices that change the item — size, cut, dose.
  final List<OptionGroup> variantGroups;

  /// Choices that add to it.
  final List<OptionGroup> addonGroups;

  /// Every group a picker has to show, variants before add-ons.
  List<OptionGroup> get allGroups => <OptionGroup>[
    ...variantGroups,
    ...addonGroups,
  ];
}

/// One member of a bundle.
@immutable
class ComboLine {
  /// Creates a combo line.
  const ComboLine({
    required this.itemId,
    required this.name,
    required this.quantity,
  });

  /// Reads the `ComboLine` object.
  factory ComboLine.fromJson(Map<String, Object?> json) => ComboLine(
    itemId: readString(json, 'item_id'),
    name: readString(json, 'name'),
    quantity: readInt(json, 'quantity'),
  );

  /// The item this line is.
  final String itemId;

  /// Its name.
  final String name;

  /// How many of it the bundle contains.
  final int quantity;
}

/// A bundle sold at one price.
@immutable
class Combo {
  /// Creates a combo.
  const Combo({
    required this.id,
    required this.merchantId,
    required this.name,
    required this.description,
    required this.imageUrl,
    required this.price,
    required this.savings,
    required this.lines,
    required this.orderable,
  });

  /// Reads the `Combo` object.
  factory Combo.fromJson(Map<String, Object?> json) {
    final Map<String, Object?> savings = readObject(json, 'savings');
    return Combo(
      id: readString(json, 'id'),
      merchantId: readString(json, 'merchant_id'),
      name: readString(json, 'name'),
      description: readString(json, 'description'),
      imageUrl: readString(json, 'image_url'),
      price: Money.fromJson(readObject(json, 'price')),
      savings: savings.isEmpty ? null : Money.fromJson(savings),
      lines: readList(json, 'lines', ComboLine.fromJson),
      orderable: readBool(json, 'orderable'),
    );
  }

  /// The combo's id.
  final String id;

  /// The shop it belongs to.
  final String merchantId;

  /// Its name.
  final String name;

  /// The shop's description.
  final String description;

  /// A pre-sized image URL, or empty.
  final String imageUrl;

  /// The bundle price.
  final Money price;

  /// What it saves against buying the parts — **computed by the server**, and
  /// null when it did not send one. The app does not subtract two totals to
  /// find out (2.9).
  final Money? savings;

  /// What is in it.
  final List<ComboLine> lines;

  /// Whether every member item is available.
  final bool orderable;
}

/// A shop's whole catalogue as a customer sees it.
@immutable
class Menu {
  /// Creates a menu.
  const Menu({
    required this.merchantId,
    required this.categories,
    required this.items,
    required this.combos,
  });

  /// Reads the `Menu` response.
  factory Menu.fromJson(Map<String, Object?> json) => Menu(
    merchantId: readString(json, 'merchant_id'),
    categories: readList(json, 'categories', Category.fromJson),
    items: readList(json, 'items', PublicItem.fromJson),
    combos: readList(json, 'combos', Combo.fromJson),
  );

  /// The shop.
  final String merchantId;

  /// Its sections, in the order the shop put them.
  final List<Category> categories;

  /// Every item.
  final List<PublicItem> items;

  /// Every bundle.
  final List<Combo> combos;

  /// The items in [category], in the order they arrived.
  List<PublicItem> itemsIn(Category category) => <PublicItem>[
    for (final PublicItem item in items)
      if (item.categoryId == category.id) item,
  ];

  /// Whether the shop has nothing to sell.
  bool get isEmpty => items.isEmpty && combos.isEmpty;
}
