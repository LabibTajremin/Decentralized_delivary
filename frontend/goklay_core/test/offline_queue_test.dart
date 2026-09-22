import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

GoklayApiClient _clientThat(
  Future<http.Response> Function(http.Request) handler,
) {
  return GoklayApiClient(
    baseUrl: Uri.parse('https://api.goklay.test'),
    httpClient: MockClient(handler),
  );
}

QueuedAction _action(String id, String path) =>
    QueuedAction(id: id, method: 'POST', path: path);

http.Response _refusal(String code) => http.Response(
  jsonEncode(<String, Object?>{
    'error': <String, Object?>{'code': code, 'message': 'no'},
  }),
  409,
);

void main() {
  group('holding work', () {
    test('starts empty and reports what it is holding', () {
      final OfflineQueue queue = OfflineQueue();
      expect(queue.isEmpty, isTrue);
      expect(queue.pending, isEmpty);

      queue.enqueue(_action('a', '/v1/partner/jobs/J1/collect'));
      expect(queue.isEmpty, isFalse);
      expect(queue.pending.single.id, 'a');
    });

    test('the pending list cannot be modified from outside', () {
      final OfflineQueue queue = OfflineQueue()..enqueue(_action('a', '/x'));
      expect(
        () => queue.pending.add(_action('b', '/y')),
        throwsUnsupportedError,
      );
    });

    test('clear drops everything, so one rider never replays another\'s work', () {
      final OfflineQueue queue = OfflineQueue()
        ..enqueue(_action('a', '/x'))
        ..enqueue(_action('b', '/y'));
      queue.clear();
      expect(queue.isEmpty, isTrue);
    });
  });

  group('replaying', () {
    test('sends everything in the order it was taken', () async {
      final List<String> paths = <String>[];
      final GoklayApiClient client = _clientThat((http.Request request) async {
        paths.add(request.url.path);
        return http.Response('{}', 200);
      });

      final OfflineQueue queue = OfflineQueue()
        ..enqueue(_action('a', '/v1/partner/jobs/J1/collect'))
        ..enqueue(_action('b', '/v1/partner/jobs/J1/deliver'));

      final FlushReport report = await queue.flush(client);
      expect(
        paths,
        <String>['/v1/partner/jobs/J1/collect', '/v1/partner/jobs/J1/deliver'],
        reason:
            'order matters: the order state machine refuses delivered from an '
            'order that never reached picked_up',
      );
      expect(report.sent.map((QueuedAction a) => a.id), <String>['a', 'b']);
      expect(report.rejected, isEmpty);
      expect(report.isEmpty, isFalse);
      expect(queue.isEmpty, isTrue);
    });

    test('carries the queued body through', () async {
      String? sent;
      final GoklayApiClient client = _clientThat((http.Request request) async {
        sent = request.body;
        return http.Response('{}', 200);
      });

      await (OfflineQueue()..enqueue(
        const QueuedAction(
          id: 'a',
          method: 'POST',
          path: '/v1/partner/jobs/J1/fail',
          body: <String, Object?>{'reason': 'nobody home'},
        ),
      )).flush(client);

      expect(sent, '{"reason":"nobody home"}');
    });

    test('stops at the first action the network still cannot carry', () async {
      int attempts = 0;
      final GoklayApiClient client = _clientThat((http.Request request) async {
        attempts++;
        if (attempts == 1) {
          return http.Response('{}', 200);
        }
        throw const SocketException('still down');
      });

      final OfflineQueue queue = OfflineQueue()
        ..enqueue(_action('a', '/v1/a'))
        ..enqueue(_action('b', '/v1/b'))
        ..enqueue(_action('c', '/v1/c'));

      final FlushReport report = await queue.flush(client);
      expect(report.sent.map((QueuedAction a) => a.id), <String>['a']);
      expect(report.rejected, isEmpty);
      // b and c are still queued, still in order.
      expect(
        queue.pending.map((QueuedAction a) => a.id),
        <String>['b', 'c'],
      );
    });

    test('drops what the server actively refuses, and says so', () async {
      // A 409 will be a 409 in an hour too. Retrying it forever means the
      // queue never drains and the rider's later work never goes.
      final GoklayApiClient client = _clientThat((http.Request request) async {
        if (request.url.path == '/v1/b') {
          return _refusal('invalid_transition');
        }
        return http.Response('{}', 200);
      });

      final OfflineQueue queue = OfflineQueue()
        ..enqueue(_action('a', '/v1/a'))
        ..enqueue(_action('b', '/v1/b'))
        ..enqueue(_action('c', '/v1/c'));

      final FlushReport report = await queue.flush(client);
      expect(report.sent.map((QueuedAction a) => a.id), <String>['a', 'c']);
      expect(report.rejected, hasLength(1));
      expect(report.rejected.single.$1.id, 'b');
      expect(report.rejected.single.$2.code, 'invalid_transition');
      expect(queue.isEmpty, isTrue);
    });

    test('an empty queue reports that nothing moved', () async {
      final GoklayApiClient client = _clientThat((http.Request request) async {
        fail('an empty queue must not send anything');
      });
      final FlushReport report = await OfflineQueue().flush(client);
      expect(report.isEmpty, isTrue);
      expect(report.sent, isEmpty);
      expect(report.rejected, isEmpty);
    });
  });
}
