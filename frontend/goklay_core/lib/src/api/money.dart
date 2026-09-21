import 'package:flutter/foundation.dart';

/// An amount of money as the server sends it.
///
/// Two representations arrive together and neither is redundant. [minor] is the
/// integer the server computed with — poisha, never a float — and is here so
/// the client can compare, sort and test against an exact value. [display] is
/// the string the server already formatted, in the caller's language, and is
/// the only thing a screen is allowed to paint.
///
/// **This class has no arithmetic and never will.** There is no `operator +`,
/// no `times`, no `sum`. That is the point: every figure a user sees was
/// worked out in Go, where the fee bands, the surcharge and the rounding live
/// and where an admin can change them without an app release (2.9). A client
/// that could add two of these would eventually be asked to, and the number it
/// produced would disagree with the receipt.
///
/// If a screen needs a figure that is not in the response, the fix is a
/// backend change.
@immutable
class Money {
  /// Creates an amount. Normally built by [Money.fromJson] from a response.
  const Money({
    required this.minor,
    required this.currency,
    required this.display,
  });

  /// Reads the `{minor, currency, display}` object the API sends.
  factory Money.fromJson(Map<String, Object?> json) {
    return Money(
      minor: (json['minor'] as num?)?.toInt() ?? 0,
      currency: json['currency'] as String? ?? 'BDT',
      display: json['display'] as String? ?? '',
    );
  }

  /// The amount in minor units, as an integer. For comparison and tests — not
  /// for display, and not for working anything out.
  final int minor;

  /// The ISO currency code. `BDT` throughout this product.
  final String currency;

  /// The formatted string to paint, already in the caller's language.
  final String display;

  @override
  bool operator ==(Object other) =>
      other is Money &&
      other.minor == minor &&
      other.currency == currency &&
      other.display == display;

  @override
  int get hashCode => Object.hash(minor, currency, display);

  @override
  String toString() => 'Money($minor $currency, "$display")';
}
