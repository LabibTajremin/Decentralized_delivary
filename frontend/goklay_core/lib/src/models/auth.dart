import 'package:flutter/foundation.dart';

import '../session/token_storage.dart';
import '../api/json.dart';

/// What `POST /v1/auth/otp/request` answers.
///
/// It says nothing about whether the number has an account. That is
/// deliberate on the server's side — answering would let anyone enumerate
/// which numbers are registered — so the screen shows the same thing either
/// way, and the countdown comes from here rather than from a constant in the
/// app.
@immutable
class OtpChallenge {
  /// Creates a challenge.
  const OtpChallenge({required this.expiresIn, required this.resendAfter});

  /// Reads the response.
  factory OtpChallenge.fromJson(Map<String, Object?> json) => OtpChallenge(
    expiresIn: Duration(seconds: readInt(json, 'expires_in')),
    resendAfter: Duration(seconds: readInt(json, 'resend_after')),
  );

  /// How long the code is good for. The server's number, counted down here.
  final Duration expiresIn;

  /// How long before "resend" becomes available.
  final Duration resendAfter;
}

/// What `POST /v1/auth/otp/verify` and `POST /v1/auth/refresh` answer.
@immutable
class AuthResult {
  /// Creates a result.
  const AuthResult({
    required this.tokens,
    required this.role,
    required this.userId,
    required this.isNewUser,
  });

  /// Reads the `TokenPair` response.
  factory AuthResult.fromJson(Map<String, Object?> json) => AuthResult(
    tokens: StoredTokens.fromJson(json),
    role: readString(json, 'role'),
    userId: readString(json, 'user_id'),
    isNewUser: readBool(json, 'new_user'),
  );

  /// The pair to persist.
  final StoredTokens tokens;

  /// Which app this account belongs in: `customer`, `merchant`, `partner` or
  /// `admin`. The customer app refuses anything else rather than showing a
  /// rider an empty home screen.
  final String role;

  /// The account's id.
  final String userId;

  /// Whether this sign-in created the account, so the app can send a first-
  /// time user to fill in their name.
  final bool isNewUser;

  /// The customer app's role.
  static const String customerRole = 'customer';

  /// The merchant app's role.
  static const String merchantRole = 'merchant';

  /// The partner app's role.
  static const String partnerRole = 'partner';

  /// Whether this account may use the app that asked for [wanted].
  bool isFor(String wanted) => role == wanted;
}

/// One device signed in on this account.
///
/// It carries no token and no hash by design: the list exists so somebody can
/// recognise a device and revoke it, which needs a label and a time, not a
/// credential.
@immutable
class DeviceSession {
  /// Creates a device session.
  const DeviceSession({
    required this.sessionId,
    required this.device,
    required this.lastSeenAt,
    required this.isCurrent,
  });

  /// Reads the `DeviceSession` object.
  factory DeviceSession.fromJson(Map<String, Object?> json) => DeviceSession(
    sessionId: readString(json, 'session_id'),
    device: readString(json, 'device'),
    lastSeenAt: DateTime.tryParse(readString(json, 'last_seen_at')),
    isCurrent: readBool(json, 'current'),
  );

  /// The session's id.
  final String sessionId;

  /// What the device called itself.
  final String device;

  /// When it last made a request.
  final DateTime? lastSeenAt;

  /// Whether this is the handset reading the list, so the screen can say so.
  final bool isCurrent;
}
