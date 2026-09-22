import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart' show Locale;
import 'package:goklay_core/goklay_core.dart';
import 'package:http/http.dart' as http;

import 'api/endpoints/catalogue_api.dart';
import 'api/endpoints/merchant_api.dart';
import 'api/endpoints/order_api.dart';
import 'environment.dart';

/// Everything the merchant app is built out of, assembled in one place.
///
/// The same shape as the customer app's, for the same reason: a single object
/// created at startup and handed down the tree, so a test constructs one over
/// a fake HTTP client and gets the whole app wired to it with no global to
/// reset afterwards.
class Dependencies {
  /// Builds the graph.
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
    merchant = MerchantApi(api);
    catalogue = MerchantCatalogueApi(api);
    orders = MerchantOrderApi(api);
  }

  /// Where this build points.
  final MerchantEnvironment environment;

  /// Who is signed in.
  final Session session;

  final http.Client _http;

  /// The shared transport.
  late final GoklayApiClient api;

  /// Signing in and out.
  late final AuthApi auth;

  /// The owner's own profile.
  late final AccountApi account;

  /// The shop.
  late final MerchantApi merchant;

  /// Its catalogue.
  late final MerchantCatalogueApi catalogue;

  /// Its order board.
  late final MerchantOrderApi orders;

  /// The language the app is running in.
  Locale get locale => api.locale;

  /// Switches language, on the transport as well as the widgets — most of
  /// what an owner reads is composed by the server.
  set locale(Locale value) => api.locale = value;

  /// Signs out on this device, clearing the cached screens too: a shop's
  /// order board left in the cache would be served to whoever signs in next.
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
