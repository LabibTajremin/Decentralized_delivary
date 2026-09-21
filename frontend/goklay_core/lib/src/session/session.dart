import 'package:flutter/foundation.dart';

import 'token_storage.dart';

/// Whether anyone is signed in on this device, and who.
///
/// It holds the token pair and nothing else. It does not call the API: the
/// auth screens do that and hand the result here, so that "what the tokens
/// are" and "how they were obtained" stay separable — which is what lets the
/// OTP flow be tested without a session and the session be tested without a
/// network.
class Session extends ChangeNotifier {
  /// Creates a session backed by [storage].
  ///
  /// Positional because Dart has no private named parameter, and the storage
  /// is the only thing a session is made of.
  Session(this._storage);

  final TokenStorage _storage;
  StoredTokens? _tokens;
  bool _isRestored = false;

  /// The bearer token for the next request, or null when signed out.
  ///
  /// Read through a callback by the API client on every request, so a refresh
  /// takes effect immediately.
  String? get accessToken => _tokens?.accessToken;

  /// The refresh token, for the one endpoint that exchanges it.
  String? get refreshToken => _tokens?.refreshToken;

  /// Whether somebody is signed in.
  bool get isSignedIn => _tokens != null;

  /// Whether [restore] has finished.
  ///
  /// The splash screen waits on this rather than guessing: showing the
  /// sign-in screen to somebody who is already signed in, for the one frame
  /// before storage answers, is the kind of flicker that reads as a bug.
  bool get isRestored => _isRestored;

  /// Loads whatever was stored on the last run.
  ///
  /// A partially written pair — one half present, the other lost — is treated
  /// as signed out and cleared, because a request with an access token and no
  /// way to refresh it fails silently an hour later.
  Future<void> restore() async {
    final StoredTokens? stored = await _storage.read();
    if (stored != null && !stored.isComplete) {
      await _storage.clear();
      _tokens = null;
    } else {
      _tokens = stored;
    }
    _isRestored = true;
    notifyListeners();
  }

  /// Takes a freshly issued pair and persists it.
  Future<void> adopt(StoredTokens tokens) async {
    await _storage.write(tokens);
    _tokens = tokens;
    notifyListeners();
  }

  /// Forgets the pair on this device.
  Future<void> signOut() async {
    await _storage.clear();
    _tokens = null;
    notifyListeners();
  }
}
