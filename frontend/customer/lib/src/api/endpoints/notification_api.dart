import 'package:goklay_core/goklay_core.dart';

import '../models/notification.dart';

/// The customer's notification history, and this device's push registration.
class NotificationApi {
  /// Creates the API against [client].
  const NotificationApi(this._client);

  final GoklayApiClient _client;

  /// What the server has sent this customer.
  ///
  /// This endpoint answers a bare array rather than an envelope, which is why
  /// it reads [ApiResponse.data] instead of [ApiResponse.asObject].
  Future<ApiPage<List<AppNotification>>> list({int limit = 20}) async {
    final ApiResponse response = await _client.get(
      '/v1/me/notifications',
      query: <String, String>{'limit': '$limit'},
    );
    final Object? raw = response.data;
    return ApiPage<List<AppNotification>>.of(response, <AppNotification>[
      if (raw is List<Object?>)
        for (final Object? entry in raw)
          if (entry is Map<String, Object?>) AppNotification.fromJson(entry),
    ]);
  }

  /// Registers this device for push.
  Future<void> registerDevice({
    required String platform,
    required String token,
  }) => _client.send(
    'POST',
    '/v1/me/device',
    body: <String, Object?>{'platform': platform, 'token': token},
  );
}
