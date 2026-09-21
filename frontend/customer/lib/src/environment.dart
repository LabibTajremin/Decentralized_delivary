import 'package:flutter/foundation.dart';

/// Where this build of the app points, and nothing else.
///
/// Deployment settings only. Business rules — fee bands, radii, COD limits —
/// are not here and are not anywhere else in the app either: they live in the
/// config module and arrive in responses (2.9, Appendix B). If something
/// belongs in this class, changing it means shipping a new build; that is the
/// test for whether it belongs here at all.
@immutable
class CustomerEnvironment {
  /// Creates an environment.
  const CustomerEnvironment({required this.apiBaseUrl});

  /// Reads the environment from `--dart-define`s baked in at build time.
  ///
  /// The default points at the local stack, so a developer who runs the app
  /// with no flags reaches the same backend `docker compose up` gives them.
  factory CustomerEnvironment.fromCompileTime() {
    return CustomerEnvironment(
      apiBaseUrl: Uri.parse(
        const String.fromEnvironment(
          'GOKLAY_API_BASE_URL',
          defaultValue: 'http://10.0.2.2:8080',
        ),
      ),
    );
  }

  /// The API root every request is made against.
  final Uri apiBaseUrl;
}
