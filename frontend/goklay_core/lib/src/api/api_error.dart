import 'package:flutter/foundation.dart';

/// A failure from the API, in the envelope every endpoint uses.
///
/// The backend answers every error as `{"error": {code, message, details}}`.
/// [message] is already written for a person, in the caller's language, so a
/// screen shows it directly rather than mapping [code] to a sentence of its
/// own — a client-side map goes stale the moment the backend adds a code.
///
/// [code] is for the client to *branch* on, not to print: retry on
/// `config_unavailable`, send the user to sign in on `unauthenticated`.
@immutable
class ApiError implements Exception {
  /// Creates an error.
  const ApiError({
    required this.statusCode,
    required this.code,
    required this.message,
    this.details = const <String, String>{},
  });

  /// Reads the error envelope out of a decoded response body.
  ///
  /// A failure that arrives without a usable envelope — a gateway's HTML error
  /// page, a truncated body — still has to become something a screen can show,
  /// so it falls back to [unreadable] rather than throwing while handling a
  /// throw.
  factory ApiError.fromResponse(int statusCode, Object? decodedBody) {
    if (decodedBody is! Map<String, Object?>) {
      return ApiError.unreadable(statusCode);
    }
    final Object? envelope = decodedBody['error'];
    if (envelope is! Map<String, Object?>) {
      return ApiError.unreadable(statusCode);
    }
    final Object? rawDetails = envelope['details'];
    return ApiError(
      statusCode: statusCode,
      code: envelope['code'] as String? ?? _unknownCode,
      message: envelope['message'] as String? ?? '',
      details: rawDetails is Map<String, Object?>
          ? rawDetails.map(
              (String k, Object? v) => MapEntry<String, String>(k, '$v'),
            )
          : const <String, String>{},
    );
  }

  /// A failure whose body could not be read as the standard envelope.
  ///
  /// [message] is left empty on purpose: there is nothing trustworthy to show,
  /// so the caller falls back to `GoklayStrings.unexpectedError`, which is
  /// translated, rather than printing a gateway's English.
  factory ApiError.unreadable(int statusCode) =>
      ApiError(statusCode: statusCode, code: _unknownCode, message: '');

  /// The failure raised when a request never reached the server at all.
  factory ApiError.offline() =>
      const ApiError(statusCode: 0, code: offlineCode, message: '');

  static const String _unknownCode = 'unknown_error';

  /// The code used when the device could not reach the server.
  static const String offlineCode = 'offline';

  /// The HTTP status, or 0 when the request never got a response.
  final int statusCode;

  /// The backend's stable machine code. Branch on this; do not print it.
  final String code;

  /// The server's own sentence for the user. Empty when there was none.
  final String message;

  /// Field-level detail, such as the maximum a value may take.
  final Map<String, String> details;

  /// Whether this failure was the network rather than the server.
  bool get isOffline => code == offlineCode;

  /// Whether the caller should send the user to sign in again.
  bool get isUnauthenticated => statusCode == 401;

  @override
  String toString() => 'ApiError($statusCode $code)';
}
