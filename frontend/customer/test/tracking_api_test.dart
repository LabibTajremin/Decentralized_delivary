import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_customer/src/api/endpoints/tracking_api.dart';
import 'package:goklay_customer/src/api/models/tracking.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

/// A client that answers one server-sent-event stream.
http.Client sseClient(
  String body, {
  int status = 200,
  void Function(http.BaseRequest request)? onRequest,
}) {
  return MockClient.streaming((
    http.BaseRequest request,
    http.ByteStream _,
  ) async {
    onRequest?.call(request);
    return http.StreamedResponse(
      Stream<List<int>>.value(utf8.encode(body)),
      status,
      headers: <String, String>{'content-type': 'text/event-stream'},
    );
  });
}

TrackingApi apiOver(
  http.Client client, {
  String? token = 'access',
  String? language,
}) => TrackingApi(
  baseUrl: Uri.parse('http://api.test'),
  httpClient: client,
  accessToken: () => token,
  language: () => language,
);

String frame(Map<String, Object?> data) =>
    'data: ${jsonEncode(data)}\n\n';

Map<String, Object?> snapshot({
  String status = 'picked_up',
  bool live = true,
  bool withPartner = true,
}) => <String, Object?>{
  'order_id': 'ord-1',
  'status': status,
  'status_label': 'পথে আছে',
  'live': live,
  if (withPartner)
    'partner': <String, Object?>{
      'id': 'ptn-1',
      'name': 'করিম',
      'phone': '01811111111',
      'vehicle': 'bike',
      'lat': 23.75,
      'lng': 90.38,
    },
};

void main() {
  test('frames arrive in order and the stream ends on the last one', () async {
    final http.Client client = sseClient(
      frame(snapshot(withPartner: false)) +
          frame(snapshot()) +
          frame(snapshot(status: 'delivered', live: false)) +
          frame(snapshot(status: 'never sent', live: true)),
    );
    final List<TrackingSnapshot> seen = await apiOver(
      client,
    ).watch('ord-1').toList();
    expect(seen, hasLength(3));
    expect(seen.first.hasPartner, isFalse);
    expect(seen[1].partner!.name, 'করিম');
    expect(seen.last.live, isFalse);
  });

  test('a stream that ends without a blank line still yields its frame', () async {
    final http.Client client = sseClient(
      'data: ${jsonEncode(snapshot())}',
    );
    final List<TrackingSnapshot> seen = await apiOver(
      client,
    ).watch('ord-1').toList();
    expect(seen, hasLength(1));
    expect(seen.single.status, 'picked_up');
  });

  test('comments, event lines and unreadable payloads are skipped', () async {
    final http.Client client = sseClient(
      ': keep-alive\n\nevent: ping\ndata: not json\n\n${frame(snapshot(live: false))}',
    );
    final List<TrackingSnapshot> seen = await apiOver(
      client,
    ).watch('ord-1').toList();
    expect(seen, hasLength(1));
  });

  test('the bearer token and the language go on the request', () async {
    http.BaseRequest? captured;
    final http.Client client = sseClient(
      frame(snapshot(live: false)),
      onRequest: (http.BaseRequest request) => captured = request,
    );
    await apiOver(client, language: 'en').watch('ord-1').toList();
    expect(captured!.headers['authorization'], 'Bearer access');
    expect(captured!.headers['accept'], 'text/event-stream');
    expect(captured!.url.queryParameters['lang'], 'en');
    expect(captured!.url.path, '/v1/track/ord-1');
  });

  test('Bengali and a signed-out client send neither', () async {
    http.BaseRequest? captured;
    final http.Client client = sseClient(
      frame(snapshot(live: false)),
      onRequest: (http.BaseRequest request) => captured = request,
    );
    await apiOver(client, token: '').watch('ord-1').toList();
    expect(captured!.headers.containsKey('authorization'), isFalse);
    expect(captured!.url.queryParameters.containsKey('lang'), isFalse);
  });

  test('a refusal arrives as the server\'s own error, not a stream', () async {
    final http.Client client = sseClient(
      jsonEncode(<String, Object?>{
        'error': <String, Object?>{
          'code': 'not_found',
          'message': 'অর্ডার পাওয়া যায়নি',
        },
      }),
      status: 404,
    );
    await expectLater(
      apiOver(client).watch('ord-1').toList(),
      throwsA(
        isA<ApiError>()
            .having((ApiError e) => e.statusCode, 'status', 404)
            .having((ApiError e) => e.message, 'message', 'অর্ডার পাওয়া যায়নি'),
      ),
    );
  });

  test('a body that is not the envelope still becomes an ApiError', () async {
    final http.Client client = sseClient('<html>gateway</html>', status: 502);
    await expectLater(
      apiOver(client).watch('ord-1').toList(),
      throwsA(isA<ApiError>().having((ApiError e) => e.message, 'message', '')),
    );
  });

  test('a socket that never opens is reported as offline', () async {
    final http.Client client = MockClient.streaming((
      http.BaseRequest _,
      http.ByteStream _,
    ) async => throw const SocketException('no route to host'));
    await expectLater(
      apiOver(client).watch('ord-1').toList(),
      throwsA(isA<ApiError>().having((ApiError e) => e.isOffline, 'offline', isTrue)),
    );
  });

  test('a client exception is reported as offline too', () async {
    final http.Client client = MockClient.streaming((
      http.BaseRequest _,
      http.ByteStream _,
    ) async => throw http.ClientException('connection closed'));
    await expectLater(
      apiOver(client).watch('ord-1').toList(),
      throwsA(isA<ApiError>().having((ApiError e) => e.isOffline, 'offline', isTrue)),
    );
  });

  test('nothing is cached: a second watch re-reads the stream', () async {
    int connections = 0;
    final http.Client client = sseClient(
      frame(snapshot(live: false)),
      onRequest: (_) => connections += 1,
    );
    final TrackingApi api = apiOver(client);
    await api.watch('ord-1').toList();
    await api.watch('ord-1').toList();
    expect(connections, 2);
  });
}
