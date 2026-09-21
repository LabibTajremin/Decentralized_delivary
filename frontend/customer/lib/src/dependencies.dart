import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart' show Locale;
import 'package:goklay_core/goklay_core.dart';
import 'package:http/http.dart' as http;

import 'api/endpoints/account_api.dart';
import 'api/endpoints/auth_api.dart';
import 'api/endpoints/cart_api.dart';
import 'api/endpoints/catalogue_api.dart';
import 'api/endpoints/discovery_api.dart';
import 'api/endpoints/notification_api.dart';
import 'api/endpoints/order_api.dart';
import 'api/endpoints/payment_api.dart';
import 'api/endpoints/support_api.dart';
import 'api/endpoints/tracking_api.dart';
import 'environment.dart';
import 'session/session.dart';
import 'session/token_storage.dart';

/// Everything the app is built out of, assembled in one place.
///
/// There is no service locator and no injection package. A single object
/// created at startup and handed down the widget tree is enough for an app
/// this shape, and it has the property a locator does not: a test constructs
/// one with a fake HTTP client and gets the whole app wired to it, with no
/// global to reset between tests.
class Dependencies {
  /// Builds the graph.
  ///
  /// [httpClient] is injected so tests never open a socket, and [storage] so
  /// they never touch the device keychain.
  Dependencies({
    required this.environment,
    required http.Client httpClient,
    TokenStorage? storage,
    Locale locale = const Locale('bn'),
  }) : _http = httpClient,
       session = Session(storage ?? InMemoryTokenStorage()) {
    api = GoklayApiClient(
      baseUrl: environment.apiBaseUrl,
      httpClient: httpClient,
      locale: locale,
      accessToken: () => session.accessToken,
    );
    auth = AuthApi(api);
    account = AccountApi(api);
    discovery = DiscoveryApi(api);
    catalogue = CatalogueApi(api);
    cart = CartApi(api);
    orders = OrderApi(api);
    payments = PaymentApi(api);
    support = SupportApi(api);
    notifications = NotificationApi(api);
    tracking = TrackingApi(
      baseUrl: environment.apiBaseUrl,
      httpClient: httpClient,
      accessToken: () => session.accessToken,
      language: () => GoklayLocalizations.languageQueryValue(api.locale),
    );
  }

  /// Where this build points.
  final CustomerEnvironment environment;

  /// Who is signed in.
  final Session session;

  final http.Client _http;

  /// The shared transport. Exposed because sign-out clears its cache.
  late final GoklayApiClient api;

  /// Signing in and out.
  late final AuthApi auth;

  /// Profile and addresses.
  late final AccountApi account;

  /// Finding shops.
  late final DiscoveryApi discovery;

  /// Menus and items.
  late final CatalogueApi catalogue;

  /// The cart.
  late final CartApi cart;

  /// Orders.
  late final OrderApi orders;

  /// Payments.
  late final PaymentApi payments;

  /// Reviews and support.
  late final SupportApi support;

  /// Notification history.
  late final NotificationApi notifications;

  /// The live delivery stream.
  late final TrackingApi tracking;

  /// The language the app is running in.
  Locale get locale => api.locale;

  /// Switches language. Set on the transport rather than held separately, so
  /// there is one answer to "what language is this app in" and requests and
  /// widgets cannot drift apart.
  set locale(Locale value) => api.locale = value;

  /// Signs out everywhere on this device.
  ///
  /// The cache is cleared as well as the tokens. A cached cart or order left
  /// behind would be served to whoever signs in next, which on a shared phone
  /// is somebody else's receipt.
  Future<void> signOut() async {
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
