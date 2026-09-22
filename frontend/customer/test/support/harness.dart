import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_customer/src/app_scope.dart';
import 'package:goklay_customer/src/dependencies.dart';
import 'package:goklay_customer/src/environment.dart';
import 'package:goklay_customer/src/l10n/customer_strings.dart';
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

/// Scrolls the list back to the top.
///
/// Needed after a [reveal] that walked to the bottom: everything above the
/// fold has been disposed, and a finder for it comes back empty.
Future<void> scrollToTop(WidgetTester tester) async {
  final Finder list = find.byType(ListView);
  if (list.evaluate().isEmpty) {
    return;
  }
  for (int attempt = 0; attempt < 20; attempt++) {
    await tester.drag(list.first, const Offset(0, 400));
    await tester.pumpAndSettle();
  }
}

/// Scrolls [finder] into view without tapping it.
///
/// A `ListView` builds only what fits, so a widget below the fold is not in
/// the tree at all — `ensureVisible` cannot help until it exists. Dragging the
/// list is what a person does, and it avoids `scrollUntilVisible`'s need to
/// name a single scrollable on a screen where every `TextField` has one.
Future<void> reveal(WidgetTester tester, Finder finder) async {
  final Finder list = find.byType(ListView);
  if (finder.evaluate().isEmpty && list.evaluate().isNotEmpty) {
    // Start from the top: a previous reveal may have walked past this widget,
    // and everything above the fold has been disposed since.
    await scrollToTop(tester);
  }
  for (int attempt = 0; attempt < 20 && finder.evaluate().isEmpty; attempt++) {
    if (list.evaluate().isEmpty) {
      break;
    }
    await tester.drag(list.first, const Offset(0, -240));
    await tester.pumpAndSettle();
  }
  if (finder.evaluate().isNotEmpty) {
    await tester.ensureVisible(finder);
    await tester.pumpAndSettle();
  }
}

/// Scrolls [finder] into view, then taps it.
Future<void> revealAndTap(WidgetTester tester, Finder finder) async {
  await reveal(tester, finder);
  await tester.tap(finder);
  await tester.pumpAndSettle();
}

/// Scrolls the field labelled [label] into view and types [value] into it.
Future<void> enterInto(
  WidgetTester tester,
  String label,
  String value,
) async {
  final Finder field = find.widgetWithText(LabelledField, label);
  await reveal(tester, field);
  await tester.enterText(
    find.descendant(of: field, matching: find.byType(TextField)),
    value,
  );
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
