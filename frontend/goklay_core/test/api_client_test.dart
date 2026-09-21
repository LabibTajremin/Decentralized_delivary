import 'dart:convert';
import 'dart:io';

import 'package:flutter/widgets.dart' show Locale;
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

final Uri _base = Uri.parse('https://api.goklay.test');

GoklayApiClient _client(
  MockClient http, {
  ResponseCache? cache,
  Locale locale = const Locale('bn'),
  String? Function()? token,
}) {
  return GoklayApiClient(
    baseUrl: _base,
    httpClient: http,
    cache: cache,
    locale: locale,
    accessToken: token,
  );
}

void main() {
  group('reading', () {
    test('a fresh body is returned and remembered with its ETag', () async {
      final InMemoryResponseCache cache = InMemoryResponseCache();
      final GoklayApiClient client = _client(
        MockClient((http.Request request) async {
          return http.Response(
            '{"status":"ok"}',
            200,
            headers: <String, String>{'etag': 'W/"v1"'},
          );
        }),
        cache: cache,
      );

      final ApiResponse response = await client.get('/v1/orders');
      expect(response.fromCache, isFalse);
      expect(response.asObject['status'], 'ok');

      final CachedResponse? stored = await cache.read(
        '$_base/v1/orders',
      );
      expect(stored?.etag, 'W/"v1"');
      expect(stored?.body, '{"status":"ok"}');
    });

    test('an unchanged resource costs a 304 and reuses the stored body', () async {
      final InMemoryResponseCache cache = InMemoryResponseCache();
      await cache.write(
        '$_base/v1/orders',
        CachedResponse(
          body: '{"status":"cached"}',
          storedAt: DateTime(2026, 9, 21),
          etag: 'W/"v1"',
        ),
      );

      String? sentIfNoneMatch;
      final GoklayApiClient client = _client(
        MockClient((http.Request request) async {
          sentIfNoneMatch = request.headers['if-none-match'];
          return http.Response('', 304);
        }),
        cache: cache,
      );

      final ApiResponse response = await client.get('/v1/orders');
      expect(sentIfNoneMatch, 'W/"v1"');
      expect(response.fromCache, isTrue);
      expect(response.asObject['status'], 'cached');
      expect(response.storedAt, DateTime(2026, 9, 21));
    });

    test('a 304 with nothing cached is treated as a failure, not a blank', () async {
      // Cannot normally happen — the client only sends If-None-Match when it
      // has something — but serving an empty screen would be worse than an
      // error the caller can retry.
      final GoklayApiClient client = _client(
        MockClient((http.Request request) async => http.Response('', 304)),
      );
      await expectLater(
        client.get('/v1/orders'),
        throwsA(isA<ApiError>().having((ApiError e) => e.statusCode, 'status', 304)),
      );
    });

    test('a server failure surfaces with its code and message', () async {
      final GoklayApiClient client = _client(
        MockClient(
          (http.Request request) async => http.Response.bytes(
            utf8.encode(
              jsonEncode(<String, Object?>{
                'error': <String, Object?>{
                  'code': 'order_not_delivered',
                  'message': 'আগে ডেলিভারি হতে হবে',
                },
              }),
            ),
            409,
          ),
        ),
      );

      await expectLater(
        client.get('/v1/orders'),
        throwsA(
          isA<ApiError>()
              .having((ApiError e) => e.code, 'code', 'order_not_delivered')
              .having((ApiError e) => e.statusCode, 'status', 409),
        ),
      );
    });

    test('a failure with an unparseable body still becomes an ApiError', () async {
      final GoklayApiClient client = _client(
        MockClient(
          (http.Request request) async => http.Response('<html>502</html>', 502),
        ),
      );
      await expectLater(
        client.get('/v1/orders'),
        throwsA(isA<ApiError>().having((ApiError e) => e.code, 'code', 'unknown_error')),
      );
    });

    test('an empty 200 body decodes to null rather than throwing', () async {
      final GoklayApiClient client = _client(
        MockClient((http.Request request) async => http.Response('', 200)),
      );
      final ApiResponse response = await client.get('/v1/ping');
      expect(response.data, isNull);
      expect(response.asObject, isEmpty);
    });

    test('a list body is returned as a list', () async {
      final GoklayApiClient client = _client(
        MockClient((http.Request request) async => http.Response('[1,2]', 200)),
      );
      final ApiResponse response = await client.get('/v1/things');
      expect(response.data, <Object?>[1, 2]);
      expect(response.asObject, isEmpty);
    });

    test('Bengali survives a body the server did not label utf-8', () async {
      // package:http decodes an unlabelled body as latin1, which would mangle
      // every sentence this Bengali-first product shows. The transport decodes
      // the bytes itself rather than trusting the header.
      const String sentence = 'আপনার অর্ডার পৌঁছে গেছে';
      final GoklayApiClient client = _client(
        MockClient(
          (http.Request request) async => http.Response.bytes(
            utf8.encode(jsonEncode(<String, Object?>{'notice': sentence})),
            200,
            headers: <String, String>{'content-type': 'application/json'},
          ),
        ),
      );

      final ApiResponse response = await client.get('/v1/orders/ORD-1');
      expect(response.asObject['notice'], sentence);
    });
  });

  group('when the network is gone', () {
    test('a socket failure falls back to the cached body', () async {
      final InMemoryResponseCache cache = InMemoryResponseCache();
      await cache.write(
        '$_base/v1/orders',
        CachedResponse(
          body: '{"status":"cached"}',
          storedAt: DateTime(2026, 9, 21),
        ),
      );
      final GoklayApiClient client = _client(
        MockClient((http.Request request) async {
          throw const SocketException('no route to host');
        }),
        cache: cache,
      );

      final ApiResponse response = await client.get('/v1/orders');
      expect(response.fromCache, isTrue);
      expect(response.asObject['status'], 'cached');
    });

    test('a client failure falls back the same way', () async {
      final InMemoryResponseCache cache = InMemoryResponseCache();
      await cache.write(
        '$_base/v1/orders',
        CachedResponse(body: '{"a":1}', storedAt: DateTime(2026, 9, 21)),
      );
      final GoklayApiClient client = _client(
        MockClient((http.Request request) async {
          throw http.ClientException('connection closed');
        }),
        cache: cache,
      );
      expect((await client.get('/v1/orders')).fromCache, isTrue);
    });

    test('with nothing cached, offline is an error the caller can name', () async {
      final GoklayApiClient client = _client(
        MockClient((http.Request request) async {
          throw const SocketException('down');
        }),
      );
      await expectLater(
        client.get('/v1/orders'),
        throwsA(isA<ApiError>().having((ApiError e) => e.isOffline, 'isOffline', isTrue)),
      );
    });
  });

  group('cache-only reads', () {
    test('return the stored body without touching the network', () async {
      final InMemoryResponseCache cache = InMemoryResponseCache();
      await cache.write(
        '$_base/v1/home',
        CachedResponse(body: '{"a":1}', storedAt: DateTime(2026, 9, 21)),
      );
      final GoklayApiClient client = _client(
        MockClient((http.Request request) async {
          fail('the network must not be touched by a cache-only read');
        }),
        cache: cache,
      );

      final ApiResponse? response = await client.cached('/v1/home');
      expect(response?.fromCache, isTrue);
      expect(response?.asObject['a'], 1);
    });

    test('return null when nothing has been stored yet', () async {
      final GoklayApiClient client = _client(
        MockClient((http.Request request) async => http.Response('{}', 200)),
      );
      expect(await client.cached('/v1/home'), isNull);
    });
  });

  group('writing', () {
    test('sends the body and returns what the server answered', () async {
      String? sentBody;
      String? sentContentType;
      final GoklayApiClient client = _client(
        MockClient((http.Request request) async {
          sentBody = request.body;
          sentContentType = request.headers['content-type'];
          return http.Response('{"id":"rev_1"}', 200);
        }),
      );

      final ApiResponse response = await client.send(
        'POST',
        '/v1/reviews',
        body: <String, Object?>{'rating': 5},
      );
      expect(sentBody, '{"rating":5}');
      expect(sentContentType, 'application/json; charset=utf-8');
      expect(response.asObject['id'], 'rev_1');
      expect(response.fromCache, isFalse);
    });

    test('a write with no body sends none', () async {
      String? contentType;
      final GoklayApiClient client = _client(
        MockClient((http.Request request) async {
          contentType = request.headers['content-type'];
          return http.Response('', 204);
        }),
      );
      final ApiResponse response = await client.send('POST', '/v1/ping');
      expect(contentType, isNull);
      expect(response.data, isNull);
    });

    test('a failure is raised rather than papered over with a cached body', () async {
      // The reason writes never fall back: telling somebody their order was
      // placed when it was not is worse than telling them it failed.
      final GoklayApiClient client = _client(
        MockClient(
          (http.Request request) async => http.Response(
            jsonEncode(<String, Object?>{
              'error': <String, Object?>{'code': 'already_reviewed', 'message': 'x'},
            }),
            409,
          ),
        ),
      );
      await expectLater(
        client.send('POST', '/v1/reviews'),
        throwsA(isA<ApiError>().having((ApiError e) => e.code, 'code', 'already_reviewed')),
      );
    });

    test('an offline write raises offline, by either exception', () async {
      final GoklayApiClient socketDown = _client(
        MockClient((http.Request request) async {
          throw const SocketException('down');
        }),
      );
      await expectLater(
        socketDown.send('POST', '/v1/reviews'),
        throwsA(isA<ApiError>().having((ApiError e) => e.isOffline, 'isOffline', isTrue)),
      );

      final GoklayApiClient clientDown = _client(
        MockClient((http.Request request) async {
          throw http.ClientException('closed');
        }),
      );
      await expectLater(
        clientDown.send('POST', '/v1/reviews'),
        throwsA(isA<ApiError>().having((ApiError e) => e.isOffline, 'isOffline', isTrue)),
      );
    });

    test('a write body that is not JSON still yields an ApiError', () async {
      final GoklayApiClient client = _client(
        MockClient((http.Request request) async => http.Response('nope', 500)),
      );
      await expectLater(
        client.send('POST', '/v1/reviews'),
        throwsA(isA<ApiError>()),
      );
    });
  });

  group('what goes on every request', () {
    test('Bengali sends no lang parameter; English sends one', () async {
      late Uri seen;
      final GoklayApiClient client = _client(
        MockClient((http.Request request) async {
          seen = request.url;
          return http.Response('{}', 200);
        }),
      );

      await client.get('/v1/home');
      expect(seen.queryParameters.containsKey('lang'), isFalse);

      client.locale = const Locale('en');
      await client.get('/v1/home');
      expect(seen.queryParameters['lang'], 'en');
    });

    test('caller query parameters survive alongside lang', () async {
      late Uri seen;
      final GoklayApiClient client = _client(
        MockClient((http.Request request) async {
          seen = request.url;
          return http.Response('{}', 200);
        }),
        locale: const Locale('en'),
      );

      await client.get(
        '/v1/reviews',
        query: <String, String>{'subject': 'merchant', 'subject_id': 'MER-1'},
      );
      expect(seen.queryParameters['subject'], 'merchant');
      expect(seen.queryParameters['subject_id'], 'MER-1');
      expect(seen.queryParameters['lang'], 'en');
    });

    test('the token is read fresh on every request', () async {
      String? token = 'first';
      final List<String?> seen = <String?>[];
      final GoklayApiClient client = _client(
        MockClient((http.Request request) async {
          seen.add(request.headers['authorization']);
          return http.Response('{}', 200);
        }),
        token: () => token,
      );

      await client.get('/v1/a');
      token = 'second';
      await client.get('/v1/b');
      expect(seen, <String>['Bearer first', 'Bearer second']);
    });

    test('no token, or an empty one, sends no authorization header', () async {
      final List<String?> seen = <String?>[];
      final GoklayApiClient anonymous = _client(
        MockClient((http.Request request) async {
          seen.add(request.headers['authorization']);
          return http.Response('{}', 200);
        }),
      );
      await anonymous.get('/v1/a');

      final GoklayApiClient empty = _client(
        MockClient((http.Request request) async {
          seen.add(request.headers['authorization']);
          return http.Response('{}', 200);
        }),
        token: () => '',
      );
      await empty.get('/v1/a');

      expect(seen, <String?>[null, null]);
    });

    test('a write carries the same headers a read does', () async {
      String? auth;
      final GoklayApiClient client = _client(
        MockClient((http.Request request) async {
          auth = request.headers['authorization'];
          return http.Response('{}', 200);
        }),
        token: () => 'tok',
      );
      await client.send('POST', '/v1/a');
      expect(auth, 'Bearer tok');
    });

    test('the client exposes its cache and can be closed', () async {
      final InMemoryResponseCache cache = InMemoryResponseCache();
      final GoklayApiClient client = _client(
        MockClient((http.Request request) async => http.Response('{}', 200)),
        cache: cache,
      );
      expect(client.cache, same(cache));
      expect(client.baseUrl, _base);
      client.close();
    });

    test('a client built without a cache still caches', () async {
      final GoklayApiClient client = _client(
        MockClient(
          (http.Request request) async => http.Response('{"a":1}', 200),
        ),
      );
      await client.get('/v1/home');
      expect(await client.cached('/v1/home'), isNotNull);
    });
  });
}
