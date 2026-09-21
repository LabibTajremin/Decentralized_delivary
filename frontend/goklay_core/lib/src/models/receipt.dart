import 'package:flutter/foundation.dart';

import '../api/json.dart';
import '../api/money.dart';

/// One labelled row of a receipt.
///
/// The server sends the breakdown already computed and labelled, in order.
/// A client prints the rows. It does not know which rows exist, what
/// `expansion_surcharge` means, or how any figure was arrived at — which is
/// what keeps three apps' idea of a bill from ever disagreeing with the one
/// the customer is charged (2.9).
///
/// Shared rather than per-app because a cart, an order and a shop's own copy
/// of that order all carry the same rows, and three parsers would eventually
/// be three slightly different parsers.
@immutable
class ReceiptRow {
  /// Creates a receipt row.
  const ReceiptRow({
    required this.key,
    required this.label,
    required this.amount,
  });

  /// Reads the `ReceiptRow` object.
  factory ReceiptRow.fromJson(Map<String, Object?> json) => ReceiptRow(
    key: readString(json, 'key'),
    label: readString(json, 'label'),
    amount: Money.fromJson(readObject(json, 'amount')),
  );

  /// A stable code — `subtotal`, `delivery`, `expansion_surcharge` — for
  /// styling a row differently. Never printed.
  final String key;

  /// The label to print, in the caller's language.
  final String label;

  /// The amount to print.
  final Money amount;
}
