import 'package:flutter/foundation.dart';

/// Where this build of the merchant app points, and nothing else.
///
/// Deployment settings only. Business rules — which documents a pharmacy
/// needs, what a shop may sell, when it counts as open — are not here and are
/// not anywhere else in the app either: they arrive in responses (2.9).
@immutable
class MerchantEnvironment {
  /// Creates an environment.
  const MerchantEnvironment({required this.apiBaseUrl});

  /// Reads the environment from `--dart-define`s baked in at build time.
  factory MerchantEnvironment.fromCompileTime() => MerchantEnvironment(
    apiBaseUrl: Uri.parse(
      const String.fromEnvironment(
        'GOKLAY_API_BASE_URL',
        defaultValue: 'http://10.0.2.2:8080',
      ),
    ),
  );

  /// The API root every request is made against.
  final Uri apiBaseUrl;
}
