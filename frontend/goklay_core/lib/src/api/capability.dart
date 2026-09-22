import 'package:flutter/foundation.dart';

/// Whether the user may do something, and what to tell them when they may not.
///
/// The server decides. `can_cancel`, `can_checkout`, `can_expand_radius` all
/// arrive shaped like this, with the reason already written in the caller's
/// language (2.9). The client renders the flag and the sentence; it never
/// works out the condition behind them.
///
/// A screen that disables a button by checking a status string, a timestamp or
/// an amount has re-implemented a backend rule and will drift from it. It
/// checks [allowed] instead.
@immutable
class Capability {
  /// Creates a capability. Normally built by [Capability.fromJson].
  const Capability({required this.allowed, this.reason, this.text});

  /// Reads the `{allowed, reason, text}` object the API sends.
  factory Capability.fromJson(Map<String, Object?> json) {
    return Capability(
      allowed: json['allowed'] as bool? ?? false,
      reason: json['reason'] as String?,
      text: json['text'] as String?,
    );
  }

  /// A capability that is permitted and needs no explanation.
  static const Capability yes = Capability(allowed: true);

  /// Whether the action is permitted.
  final bool allowed;

  /// A stable machine code for why not, for logging and tests. Never shown.
  final String? reason;

  /// The sentence to show the user when [allowed] is false, composed by the
  /// server in their language.
  final String? text;

  @override
  bool operator ==(Object other) =>
      other is Capability &&
      other.allowed == allowed &&
      other.reason == reason &&
      other.text == text;

  @override
  int get hashCode => Object.hash(allowed, reason, text);

  @override
  String toString() => 'Capability(allowed: $allowed, reason: $reason)';
}
