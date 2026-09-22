import 'package:flutter/foundation.dart';

/// Where this build of the partner app points, and nothing else.
@immutable
class PartnerEnvironment {
  /// Creates an environment.
  const PartnerEnvironment({required this.apiBaseUrl});

  /// Reads the environment from `--dart-define`s baked in at build time.
  factory PartnerEnvironment.fromCompileTime() => PartnerEnvironment(
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
