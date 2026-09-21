import 'package:flutter/foundation.dart';

/// The pair the backend hands out at sign-in.
@immutable
class StoredTokens {
  /// Creates a token pair.
  const StoredTokens({required this.accessToken, required this.refreshToken});

  /// Reads the `TokenPair` the auth endpoints return.
  factory StoredTokens.fromJson(Map<String, Object?> json) => StoredTokens(
    accessToken: json['access_token'] as String? ?? '',
    refreshToken: json['refresh_token'] as String? ?? '',
  );

  /// The short-lived bearer token sent on every request.
  final String accessToken;

  /// The long-lived token exchanged for a new pair.
  final String refreshToken;

  /// Whether both halves are present.
  bool get isComplete => accessToken.isNotEmpty && refreshToken.isNotEmpty;

  @override
  bool operator ==(Object other) =>
      other is StoredTokens &&
      other.accessToken == accessToken &&
      other.refreshToken == refreshToken;

  @override
  int get hashCode => Object.hash(accessToken, refreshToken);
}

/// Where the token pair lives between launches.
///
/// A port rather than a concrete class because what is correct here is
/// platform-specific and likely to change: a refresh token is a bearer
/// credential, so it belongs in the Keystore or the Keychain rather than in a
/// preferences file. Keeping the seam means changing that is one class, not
/// every screen.
abstract class TokenStorage {
  /// The stored pair, or null when nobody is signed in on this device.
  Future<StoredTokens?> read();

  /// Persists [tokens], replacing whatever was there.
  Future<void> write(StoredTokens tokens);

  /// Removes the pair. Called on sign-out and whenever the server tells us
  /// the refresh token is no longer good.
  Future<void> clear();
}

/// Storage that lasts as long as the process.
///
/// What the tests use. It is also what the app falls back to if secure
/// storage is unavailable on a device, which costs the user a sign-in on next
/// launch rather than an unusable app.
class InMemoryTokenStorage implements TokenStorage {
  /// Creates empty storage, or storage already holding [initial].
  InMemoryTokenStorage([this._tokens]);

  StoredTokens? _tokens;

  @override
  Future<StoredTokens?> read() async => _tokens;

  @override
  Future<void> write(StoredTokens tokens) async => _tokens = tokens;

  @override
  Future<void> clear() async => _tokens = null;
}
