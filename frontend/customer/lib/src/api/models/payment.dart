import 'package:flutter/foundation.dart';
import 'package:goklay_core/goklay_core.dart';


/// What `POST /v1/payments/checkout` answers.
@immutable
class Checkout {
  /// Creates a checkout.
  const Checkout({
    required this.paymentId,
    required this.orderId,
    required this.status,
    required this.statusLabel,
    required this.amount,
    required this.redirectUrl,
  });

  /// Reads the `Checkout` response.
  factory Checkout.fromJson(Map<String, Object?> json) => Checkout(
    paymentId: readString(json, 'payment_id'),
    orderId: readString(json, 'order_id'),
    status: readString(json, 'status'),
    statusLabel: readString(json, 'status_label'),
    amount: Money.fromJson(readObject(json, 'amount')),
    redirectUrl: readString(json, 'redirect_url'),
  );

  /// The payment attempt's id.
  final String paymentId;

  /// The order being paid for.
  final String orderId;

  /// `pending`, `captured`, `failed` or `refunded`.
  final String status;

  /// That state as a sentence, composed by the server.
  final String statusLabel;

  /// What is owed.
  final Money amount;

  /// Where to send the customer to pay. Empty on a resumed checkout whose
  /// attempt never recorded one, which is why the screen checks rather than
  /// launching an empty URL.
  final String redirectUrl;

  /// Whether there is somewhere to send them.
  bool get hasRedirect => redirectUrl.isNotEmpty;
}

/// A payment's current state.
@immutable
class Payment {
  /// Creates a payment.
  const Payment({
    required this.id,
    required this.orderId,
    required this.status,
    required this.statusLabel,
    required this.amount,
    required this.reason,
  });

  /// Reads the `Payment` response.
  factory Payment.fromJson(Map<String, Object?> json) => Payment(
    id: readString(json, 'id'),
    orderId: readString(json, 'order_id'),
    status: readString(json, 'status'),
    statusLabel: readString(json, 'status_label'),
    amount: Money.fromJson(readObject(json, 'amount')),
    reason: readString(json, 'reason'),
  );

  /// The payment's id.
  final String id;

  /// The order it belongs to.
  final String orderId;

  /// `pending`, `captured`, `failed` or `refunded`.
  final String status;

  /// That state as a sentence.
  final String statusLabel;

  /// The amount.
  final Money amount;

  /// Why it failed or was refunded, in the caller's language. Empty
  /// otherwise.
  final String reason;

  /// Whether the money is in.
  bool get isCaptured => status == 'captured';

  /// Whether the customer should be offered another attempt.
  bool get isFailed => status == 'failed';
}
