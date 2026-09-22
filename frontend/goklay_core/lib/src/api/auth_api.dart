import '../models/auth.dart';

import 'api_client.dart';
import 'api_page.dart';

/// The auth calls the customer app makes.
///
/// Signing in is a phone number and a six-digit code, and nothing here decides
/// whether a code is still good, how long to wait before offering "resend", or
/// whether a number has an account. All three are the server's to state, and
/// two of them arrive in [OtpChallenge].
class AuthApi {
  /// Creates the API against [client].
  const AuthApi(this._client);

  final GoklayApiClient _client;

  /// Asks for a code to be sent to [phone].
  Future<OtpChallenge> requestOtp(String phone) async {
    final ApiResponse response = await _client.send(
      'POST',
      '/v1/auth/otp/request',
      body: <String, Object?>{'phone': phone},
    );
    return OtpChallenge.fromJson(response.asObject);
  }

  /// Exchanges [code] for a token pair.
  ///
  /// [role] is the app asking, and it is sent rather than discovered
  /// afterwards: the server refuses a token for a role its endpoint does not
  /// admit (2.7), which is better than handing a rider an empty shop screen
  /// and letting them find out three taps later.
  Future<AuthResult> verifyOtp({
    required String phone,
    required String code,
    required String role,
    String device = '',
  }) async {
    final ApiResponse response = await _client.send(
      'POST',
      '/v1/auth/otp/verify',
      body: <String, Object?>{
        'phone': phone,
        'code': code,
        'role': role,
        if (device.isNotEmpty) 'device': device,
      },
    );
    return AuthResult.fromJson(response.asObject);
  }

  /// Exchanges a refresh token for a new pair.
  Future<AuthResult> refresh(String refreshToken) async {
    final ApiResponse response = await _client.send(
      'POST',
      '/v1/auth/refresh',
      body: <String, Object?>{'refresh_token': refreshToken},
    );
    return AuthResult.fromJson(response.asObject);
  }

  /// Revokes the current session on the server.
  Future<void> logout() => _client.send('POST', '/v1/auth/logout');

  /// Revokes every session on this account.
  Future<void> logoutAll() => _client.send('POST', '/v1/auth/logout-all');

  /// The devices signed in on this account.
  Future<ApiPage<List<DeviceSession>>> sessions() async {
    final ApiResponse response = await _client.get('/v1/auth/sessions');
    final Object? raw = response.asObject['sessions'];
    return ApiPage<List<DeviceSession>>.of(response, <DeviceSession>[
      if (raw is List<Object?>)
        for (final Object? entry in raw)
          if (entry is Map<String, Object?>) DeviceSession.fromJson(entry),
    ]);
  }
}
