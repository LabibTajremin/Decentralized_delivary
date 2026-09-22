import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart' show Locale;
import 'package:goklay_core/goklay_core.dart';
import 'package:http/http.dart' as http;

import 'api/endpoints/partner_api.dart';
import 'environment.dart';

/// Everything the partner app is built out of, assembled in one place.
///
/// It is the only one of the three graphs that carries an [OfflineQueue].
/// P17 built the queue for exactly this app: a rider taps "collected" and
/// "delivered" in places with no signal, and those taps can neither be lost
/// nor replayed out of order — the order state machine refuses `delivered`
/// from an order that never reached `picked_up`.
class Dependencies {
  /// Builds the graph.
  Dependencies({
    required this.environment,
    required http.Client httpClient,
    TokenStorage? storage,
    OfflineQueue? queue,
    Locale locale = const Locale('bn'),
  }) : _http = httpClient,
       session = Session(storage ?? InMemoryTokenStorage()),
       outbox = queue ?? OfflineQueue() {
    api = GoklayApiClient(
      baseUrl: environment.apiBaseUrl,
      httpClient: httpClient,
      locale: locale,
      accessToken: () => session.accessToken,
    );
    auth = AuthApi(api);
    account = AccountApi(api);
    partner = PartnerApi(api);
  }

  /// Where this build points.
  final PartnerEnvironment environment;

  /// Who is signed in.
  final Session session;

  /// The taps that have not reached the server yet.
  final OfflineQueue outbox;

  final http.Client _http;

  /// The shared transport.
  late final GoklayApiClient api;

  /// Signing in and out.
  late final AuthApi auth;

  /// The rider's own profile.
  late final AccountApi account;

  /// The rider's work.
  late final PartnerApi partner;

  /// The language the app is running in.
  Locale get locale => api.locale;

  /// Switches language, on the transport as well as the widgets.
  set locale(Locale value) => api.locale = value;

  /// Replays whatever is queued, oldest first.
  Future<FlushReport> flushOutbox() => outbox.flush(api);

  /// Signs out on this device.
  ///
  /// The outbox is cleared as well as the tokens and the cache: one rider's
  /// unsent taps must never be replayed under the next rider's token, which
  /// on a shared handset is somebody else's delivery.
  Future<void> signOut() async {
    outbox.clear();
    await session.signOut();
    await api.cache.clear();
  }

  /// Releases the HTTP client.
  @mustCallSuper
  void dispose() {
    session.dispose();
    _http.close();
  }
}
