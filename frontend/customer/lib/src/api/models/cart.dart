import 'package:flutter/foundation.dart';
import 'package:goklay_core/goklay_core.dart';


/// One chosen add-on or variant on a cart line.
@immutable
class CartOption {
  /// Creates an option.
  const CartOption({required this.name, required this.price});

  /// Reads the `CartOption` object.
  factory CartOption.fromJson(Map<String, Object?> json) => CartOption(
    name: readString(json, 'name'),
    price: Money.fromJson(readObject(json, 'price')),
  );

  /// What it is called.
  final String name;

  /// What it added, already formatted.
  final Money price;
}

/// One line in the cart.
///
/// [issue] and [issueText] are why this matters: a cart is revalidated against
/// the live shop on every read, so a price can change or an item can go out of
/// stock between adding and checking out. The server decides that and writes
/// the sentence; the app shows it and disables the line.
@immutable
class CartLine {
  /// Creates a cart line.
  const CartLine({
    required this.id,
    required this.name,
    required this.options,
    required this.quantity,
    required this.note,
    required this.lineTotal,
    required this.issue,
    required this.issueText,
    required this.orderable,
  });

  /// Reads the `CartLine` object.
  factory CartLine.fromJson(Map<String, Object?> json) => CartLine(
    id: readString(json, 'id'),
    name: readString(json, 'name'),
    options: readList(json, 'options', CartOption.fromJson),
    quantity: readInt(json, 'quantity'),
    note: readString(json, 'note'),
    lineTotal: Money.fromJson(readObject(json, 'line_total')),
    issue: readOptionalString(json, 'issue'),
    issueText: readString(json, 'issue_text'),
    orderable: readBool(json, 'orderable'),
  );

  /// The line's id, for quantity changes and removal.
  final String id;

  /// The item or combo name.
  final String name;

  /// The chosen options.
  final List<CartOption> options;

  /// How many.
  final int quantity;

  /// The customer's note to the shop.
  final String note;

  /// What this line costs in total, already worked out.
  final Money lineTotal;

  /// A machine code for what is wrong with this line, or null.
  final String? issue;

  /// The sentence explaining it, composed by the server.
  final String issueText;

  /// Whether this line may be ordered as it stands.
  final bool orderable;

  /// Whether anything is wrong with this line.
  bool get hasIssue => issue != null;
}

/// What the cart costs.
@immutable
class CartPricing {
  /// Creates a pricing summary.
  const CartPricing({
    required this.total,
    required this.freeDelivery,
    required this.awayFromFreeDelivery,
    required this.expanded,
    required this.rows,
    required this.notice,
  });

  /// Reads the `CartPricing` object.
  factory CartPricing.fromJson(Map<String, Object?> json) {
    final Map<String, Object?> away = readObject(
      json,
      'away_from_free_delivery',
    );
    return CartPricing(
      total: Money.fromJson(readObject(json, 'total')),
      freeDelivery: readBool(json, 'free_delivery'),
      awayFromFreeDelivery: away.isEmpty ? null : Money.fromJson(away),
      expanded: readBool(json, 'expanded'),
      rows: readList(json, 'rows', ReceiptRow.fromJson),
      notice: readString(json, 'notice'),
    );
  }

  /// What the customer pays.
  final Money total;

  /// Whether delivery came out free.
  final bool freeDelivery;

  /// How much more would earn free delivery, or null when it already is, or
  /// when the shop has no such offer. Computed by pricing, not here.
  final Money? awayFromFreeDelivery;

  /// Whether the fee includes D2's expansion surcharge.
  final bool expanded;

  /// The receipt, in the order it prints.
  final List<ReceiptRow> rows;

  /// A line about the pricing — the free-delivery nudge, the surcharge
  /// explanation — composed by the server.
  final String notice;
}

/// The cart, as revalidated against the shop on every read.
@immutable
class Cart {
  /// Creates a cart.
  const Cart({
    required this.id,
    required this.merchantId,
    required this.merchantName,
    required this.merchantLogoUrl,
    required this.addressId,
    required this.lines,
    required this.count,
    required this.orderable,
    required this.blocker,
    required this.blockerText,
    required this.pricing,
  });

  /// Reads the `Cart` response.
  factory Cart.fromJson(Map<String, Object?> json) => Cart(
    id: readString(json, 'id'),
    merchantId: readString(json, 'merchant_id'),
    merchantName: readString(json, 'merchant_name'),
    merchantLogoUrl: readString(json, 'merchant_logo_url'),
    addressId: readOptionalString(json, 'address_id'),
    lines: readList(json, 'lines', CartLine.fromJson),
    count: readInt(json, 'count'),
    orderable: readBool(json, 'orderable'),
    blocker: readOptionalString(json, 'blocker'),
    blockerText: readString(json, 'blocker_text'),
    pricing: CartPricing.fromJson(readObject(json, 'pricing')),
  );

  /// The cart's id.
  final String id;

  /// The shop it is from. A cart holds one shop's items only (P09).
  final String merchantId;

  /// That shop's name.
  final String merchantName;

  /// That shop's logo.
  final String merchantLogoUrl;

  /// The delivery address it is bound to, or null when none is chosen yet.
  final String? addressId;

  /// The lines.
  final List<CartLine> lines;

  /// How many items in total.
  final int count;

  /// **The checkout button reads this and nothing else.**
  ///
  /// Whether the cart may be ordered is a server decision that folds in the
  /// minimum order, the shop's hours, its holiday mode, stock, the delivery
  /// address and D3's division rule. An app that tried to infer it would get a
  /// different answer from the one checkout enforces.
  final bool orderable;

  /// A machine code for why not — `empty`, `no_address`, `shop_closed` — or
  /// null. For branching, never for printing.
  final String? blocker;

  /// The sentence to show when [orderable] is false, in the caller's
  /// language.
  final String blockerText;

  /// What it costs.
  final CartPricing pricing;

  /// Whether the cart has nothing in it.
  bool get isEmpty => lines.isEmpty;
}
