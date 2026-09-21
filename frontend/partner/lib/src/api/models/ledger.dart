import 'package:flutter/foundation.dart';
import 'package:goklay_core/goklay_core.dart';

/// One cash-on-delivery order's worth of cash.
@immutable
class Collection {
  /// Creates a collection.
  const Collection({
    required this.id,
    required this.orderId,
    required this.amount,
    required this.status,
    required this.statusLabel,
  });

  /// Reads the `Collection` object.
  factory Collection.fromJson(Map<String, Object?> json) => Collection(
    id: readString(json, 'id'),
    orderId: readString(json, 'order_id'),
    amount: Money.fromJson(readObject(json, 'amount')),
    status: readString(json, 'status'),
    statusLabel: readString(json, 'status_label'),
  );

  /// The collection's id.
  final String id;

  /// The order it came from.
  final String orderId;

  /// How much.
  final Money amount;

  /// `held` or `remitted`.
  final String status;

  /// That state as a sentence.
  final String statusLabel;

  /// Whether the rider still has it.
  bool get isHeld => status == 'held';
}

/// The rider's cash position.
///
/// **Both totals are the server's.** They are recomputed from the collections
/// rather than stored, and the app does not add up [held] to check: a rider
/// and an office disagreeing about how much cash is in a bag is exactly the
/// kind of argument a second calculation causes (2.9).
@immutable
class Ledger {
  /// Creates a ledger.
  const Ledger({
    required this.partnerId,
    required this.outstanding,
    required this.remitted,
    required this.held,
  });

  /// Reads the `Ledger` response.
  factory Ledger.fromJson(Map<String, Object?> json) => Ledger(
    partnerId: readString(json, 'partner_id'),
    outstanding: Money.fromJson(readObject(json, 'outstanding')),
    remitted: Money.fromJson(readObject(json, 'remitted')),
    held: readList(json, 'held', Collection.fromJson),
  );

  /// Whose ledger.
  final String partnerId;

  /// What is still owed.
  final Money outstanding;

  /// What has already been handed over.
  final Money remitted;

  /// The held collections, oldest first — what a "please remit" screen lists.
  final List<Collection> held;

  /// Whether there is nothing to hand over.
  bool get isSettled => held.isEmpty;
}
