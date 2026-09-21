import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_customer/src/app_scope.dart';
import 'package:goklay_customer/src/dependencies.dart';
import 'package:goklay_customer/src/environment.dart';
import 'package:goklay_customer/src/l10n/customer_strings.dart';
import 'package:goklay_customer/src/session/token_storage.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

/// One recorded request, so a test can assert on the path, the query and the
/// body the app actually sent.
class SentRequest {
  /// Records a request.
  SentRequest(this.method, this.url, this.body, this.headers);

  /// The HTTP method.
  final String method;

  /// Where it went, query string and all.
  final Uri url;

  /// The request body, decoded as JSON, or null.
  final Object? body;

  /// The headers as sent.
  final Map<String, String> headers;
}

/// A backend that answers from a table of canned responses.
///
/// Keyed by `'<METHOD> <path>'`. A request for a path the table does not have
/// fails the test loudly rather than returning an empty body — a screen that
/// silently painted nothing because a test forgot a route is the sort of green
/// that hides a bug.
class FakeBackend {
  /// Creates a backend over [routes].
  FakeBackend(this.routes);

  /// The canned answers.
  final Map<String, Object? Function(SentRequest request)> routes;

  /// Statuses to answer with, by the same key. Anything absent is 200.
  final Map<String, int> statuses = <String, int>{};

  /// Every request the app made, in order.
  final List<SentRequest> sent = <SentRequest>[];

  /// The client to hand to [Dependencies].
  http.Client get client => MockClient((http.Request request) async {
    final String key = '${request.method} ${request.url.path}';
    final SentRequest recorded = SentRequest(
      request.method,
      request.url,
      request.body.isEmpty ? null : jsonDecode(request.body) as Object?,
      request.headers,
    );
    sent.add(recorded);
    final Object? Function(SentRequest)? handler = routes[key];
    if (handler == null) {
      fail('FakeBackend has no route for "$key"');
    }
    final Object? body = handler(recorded);
    return http.Response.bytes(
      utf8.encode(body == null ? '' : jsonEncode(body)),
      statuses[key] ?? 200,
      headers: <String, String>{
        'content-type': 'application/json; charset=utf-8',
      },
    );
  });

  /// The requests that went to [path].
  List<SentRequest> to(String path) => <SentRequest>[
    for (final SentRequest request in sent)
      if (request.url.path == path) request,
  ];
}

/// The standard error envelope, for a route that should fail.
Map<String, Object?> errorBody(String code, String message) =>
    <String, Object?>{
      'error': <String, Object?>{'code': code, 'message': message},
    };

/// Token storage holding a signed-in pair, or nothing.
TokenStorage harnessStorage({bool signedIn = true}) => InMemoryTokenStorage(
  signedIn
      ? const StoredTokens(accessToken: 'access', refreshToken: 'refresh')
      : null,
);

/// A dependency graph over [backend], signed in unless told otherwise.
///
/// It restores the session before returning, because the app does: a screen
/// that made a request before storage had answered would send no bearer token
/// and get a 401 the real app never sees.
Future<Dependencies> harnessDependencies(
  FakeBackend backend, {
  bool signedIn = true,
  Locale locale = const Locale('bn'),
}) async {
  final Dependencies dependencies = Dependencies(
    environment: CustomerEnvironment(apiBaseUrl: Uri.parse('http://api.test')),
    httpClient: backend.client,
    locale: locale,
    storage: harnessStorage(signedIn: signedIn),
  );
  await dependencies.session.restore();
  return dependencies;
}

/// Scrolls [finder] into view, then taps it.
///
/// A test window is 800x600 logical pixels, and Flutter builds no semantics
/// node for a widget that is off-screen — so a `bySemanticsLabel` finder comes
/// back empty for a button that exists but is below the fold. Scrolling to it
/// first is what a customer does anyway. Pass a widget finder, not a semantics
/// one, for the same reason.
Future<void> revealAndTap(WidgetTester tester, Finder finder) async {
  if (finder.evaluate().isEmpty) {
    // Not merely off-screen: a ListView builds only what fits, so it is not in
    // the tree at all yet. Scroll until it is — through the list's own
    // scrollable, because every TextField on a form has one of its own and
    // `find.byType(Scrollable).first` would pick one of those.
    final Finder list = find.byType(ListView);
    await tester.scrollUntilVisible(
      finder,
      200,
      scrollable: list.evaluate().isEmpty
          ? find.byType(Scrollable).first
          : find
                .descendant(of: list.first, matching: find.byType(Scrollable))
                .first,
    );
  }
  await tester.ensureVisible(finder);
  await tester.pumpAndSettle();
  await tester.tap(finder);
  await tester.pumpAndSettle();
}

/// Taps the app bar's back button.
///
/// Not `tester.pageBack()`: that looks for Cupertino's back button, and these
/// screens are Material ones with a localised tooltip.
Future<void> goBack(WidgetTester tester) async {
  await tester.tap(find.byType(BackButton).last);
  await tester.pumpAndSettle();
}

/// Pumps [child] inside everything a screen expects: the scope, the theme and
/// both localisation tables.
Future<void> pumpScreen(
  WidgetTester tester,
  Widget child, {
  required Dependencies dependencies,
  Locale locale = const Locale('bn'),
}) async {
  await tester.pumpWidget(
    AppScope(
      dependencies: dependencies,
      child: MaterialApp(
        theme: GoklayTheme.light(),
        locale: locale,
        localizationsDelegates: const <LocalizationsDelegate<Object>>[
          GoklayLocalizations.delegate,
          CustomerLocalizations.delegate,
          GlobalMaterialLocalizations.delegate,
          GlobalWidgetsLocalizations.delegate,
          GlobalCupertinoLocalizations.delegate,
        ],
        supportedLocales: GoklayLocalizations.supportedLocales,
        home: child,
      ),
    ),
  );
  // Not pumpAndSettle: a screen that is still loading is showing a spinner,
  // and a spinner never settles. Two pumps is enough for the localisation
  // delegates, which resolve on a microtask.
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 50));
}
