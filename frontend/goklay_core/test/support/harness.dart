import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

/// One recorded request, so a test can assert on the path, the query and the
/// body that was actually sent.
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

/// A backend that answers from a table of canned responses, keyed by
/// `'<METHOD> <path>'`.
///
/// A request for a path the table does not have fails the test loudly rather
/// than returning an empty body: a screen that painted nothing because a test
/// forgot a route is the sort of green that hides a bug.
class FakeBackend {
  /// Creates a backend over [routes].
  FakeBackend(this.routes);

  /// The canned answers.
  final Map<String, Object? Function(SentRequest request)> routes;

  /// Statuses to answer with, by the same key. Anything absent is 200.
  final Map<String, int> statuses = <String, int>{};

  /// Every request made, in order.
  final List<SentRequest> sent = <SentRequest>[];

  /// The client to hand to [GoklayApiClient].
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

/// A client over [backend], signed in unless told otherwise.
GoklayApiClient harnessClient(
  FakeBackend backend, {
  String? token = 'access',
  Locale locale = const Locale('bn'),
}) => GoklayApiClient(
  baseUrl: Uri.parse('http://api.test'),
  httpClient: backend.client,
  locale: locale,
  accessToken: () => token,
);

/// Pumps [child] inside the theme and both localisation tables.
///
/// Not `pumpAndSettle`: a screen that is still loading is showing a spinner,
/// and a spinner never settles. Two pumps is enough for the delegates, which
/// resolve on a microtask.
Future<void> pumpWidgetUnderTest(
  WidgetTester tester,
  Widget child, {
  Locale locale = const Locale('bn'),
}) async {
  await tester.pumpWidget(
    MaterialApp(
      theme: GoklayTheme.light(),
      locale: locale,
      localizationsDelegates: const <LocalizationsDelegate<Object>>[
        GoklayLocalizations.delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
      ],
      supportedLocales: GoklayLocalizations.supportedLocales,
      home: child,
    ),
  );
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 50));
}

/// A `Money` object.
Map<String, Object?> money(int minor, String display) => <String, Object?>{
  'minor': minor,
  'currency': 'BDT',
  'display': display,
};

/// An `Address`.
Map<String, Object?> addressJson({
  String id = 'adr-1',
  String label = 'বাসা',
  bool isDefault = true,
}) => <String, Object?>{
  'id': id,
  'label': label,
  'recipient_name': 'রিয়া',
  'recipient_phone': '01712345678',
  'line1': 'রোড ৫, ধানমন্ডি',
  'line2': '',
  'instructions': 'নীল গেট',
  'single_line': 'রোড ৫, ধানমন্ডি, ঢাকা',
  'lat': 23.7461,
  'lng': 90.3742,
  'area_code': 'DHK-DHM',
  'area_name': 'ধানমন্ডি',
  'district_code': 'DHK',
  'division_code': 'DHA',
  'is_default': isDefault,
};

/// A `Profile`.
Map<String, Object?> profileJson({String name = 'রিয়া'}) => <String, Object?>{
  'user_id': 'usr-1',
  'name': name,
  'display_name': name.isEmpty ? 'অতিথি' : name,
  'email': 'riya@example.com',
  'language': 'bn',
};

/// A `TokenPair`.
Map<String, Object?> tokenPairJson({String role = 'customer'}) =>
    <String, Object?>{
      'access_token': 'access-1',
      'refresh_token': 'refresh-1',
      'token_type': 'Bearer',
      'expires_in': 900,
      'role': role,
      'user_id': 'usr-1',
      'session_id': 'ses-1',
      'new_user': false,
    };

/// An `Area`.
Map<String, Object?> areaJson() => <String, Object?>{
  'area_code': 'DHK-DHM',
  'area_name': 'ধানমন্ডি',
  'district_code': 'DHK',
  'division_code': 'DHA',
  'division_name': 'ঢাকা',
};
