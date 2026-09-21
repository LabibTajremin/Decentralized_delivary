import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';

void main() {
  group('Money', () {
    test('reads the object the API sends', () {
      const Map<String, Object?> json = <String, Object?>{
        'minor': 25000,
        'currency': 'BDT',
        'display': '৳ ২৫০',
      };
      final Money money = Money.fromJson(json);
      expect(money.minor, 25000);
      expect(money.currency, 'BDT');
      expect(money.display, '৳ ২৫০');
    });

    test('survives a field the server left out', () {
      final Money money = Money.fromJson(const <String, Object?>{});
      expect(money.minor, 0);
      expect(money.currency, 'BDT');
      expect(money.display, '');
    });

    test('accepts a minor unit that arrived as a double', () {
      // A JSON number is a num; some encoders emit 25000.0.
      final Money money = Money.fromJson(const <String, Object?>{
        'minor': 25000.0,
      });
      expect(money.minor, 25000);
    });

    test('two amounts are equal when all three parts match', () {
      const Money a = Money(minor: 100, currency: 'BDT', display: '৳ ১');
      const Money b = Money(minor: 100, currency: 'BDT', display: '৳ ১');
      const Money c = Money(minor: 100, currency: 'BDT', display: 'BDT 1');
      expect(a, b);
      expect(a.hashCode, b.hashCode);
      expect(a, isNot(c));
      expect(a, isNot(const Object()));
    });

    test('describes itself with both representations', () {
      const Money money = Money(minor: 100, currency: 'BDT', display: '৳ ১');
      expect(money.toString(), contains('100'));
      expect(money.toString(), contains('৳ ১'));
    });

    test('exposes no way to do arithmetic', () {
      // The guarantee this class exists for. If somebody adds an operator,
      // this test is the reminder of why they should not have.
      const Money money = Money(minor: 100, currency: 'BDT', display: '৳ ১');
      expect(money, isNot(isA<num>()));
      // The only members are the three the server sent, plus object identity.
      expect(money.display, '৳ ১');
    });
  });

  group('Capability', () {
    test('reads the object the API sends', () {
      final Capability can = Capability.fromJson(const <String, Object?>{
        'allowed': false,
        'reason': 'window_closed',
        'text': 'বাতিল করার সময় শেষ',
      });
      expect(can.allowed, isFalse);
      expect(can.reason, 'window_closed');
      expect(can.text, 'বাতিল করার সময় শেষ');
    });

    test('a missing flag is a refusal, never a permission', () {
      // Defaulting the other way would let a malformed response unlock an
      // action the server never granted.
      final Capability can = Capability.fromJson(const <String, Object?>{});
      expect(can.allowed, isFalse);
      expect(can.reason, isNull);
      expect(can.text, isNull);
    });

    test('yes is the permitted, unexplained case', () {
      expect(Capability.yes.allowed, isTrue);
      expect(Capability.yes.text, isNull);
    });

    test('equality covers all three parts', () {
      const Capability a = Capability(allowed: true, reason: 'r', text: 't');
      const Capability b = Capability(allowed: true, reason: 'r', text: 't');
      expect(a, b);
      expect(a.hashCode, b.hashCode);
      expect(a, isNot(Capability.yes));
      expect(a, isNot(const Object()));
      expect(a.toString(), contains('allowed: true'));
    });
  });

  group('ApiError', () {
    test('reads the envelope every endpoint uses', () {
      final ApiError error = ApiError.fromResponse(400, <String, Object?>{
        'error': <String, Object?>{
          'code': 'config_out_of_bounds',
          'message': 'That value is too large.',
          'details': <String, Object?>{'maximum': 100000},
        },
      });
      expect(error.statusCode, 400);
      expect(error.code, 'config_out_of_bounds');
      expect(error.message, 'That value is too large.');
      expect(error.details['maximum'], '100000');
    });

    test('an envelope without details still reads', () {
      final ApiError error = ApiError.fromResponse(404, <String, Object?>{
        'error': <String, Object?>{'code': 'not_found', 'message': 'Gone.'},
      });
      expect(error.details, isEmpty);
    });

    test('details that are not an object are dropped, not crashed on', () {
      final ApiError error = ApiError.fromResponse(400, <String, Object?>{
        'error': <String, Object?>{
          'code': 'bad',
          'message': 'no',
          'details': 'not an object',
        },
      });
      expect(error.details, isEmpty);
    });

    test('a gateway HTML page becomes something a screen can still show', () {
      // The body is not JSON, or is JSON of the wrong shape. Both have to
      // produce an error rather than throw while handling a throw.
      expect(ApiError.fromResponse(502, 'not json at all').code, 'unknown_error');
      expect(
        ApiError.fromResponse(502, <String, Object?>{'nope': 1}).code,
        'unknown_error',
      );
      expect(ApiError.fromResponse(502, null).code, 'unknown_error');
    });

    test('an unreadable failure carries no message to print', () {
      // Empty on purpose: the caller falls back to a translated string rather
      // than showing a user whatever English a proxy happened to emit.
      expect(ApiError.unreadable(500).message, isEmpty);
    });

    test('a missing code or message falls back rather than throwing', () {
      final ApiError error = ApiError.fromResponse(500, <String, Object?>{
        'error': <String, Object?>{},
      });
      expect(error.code, 'unknown_error');
      expect(error.message, isEmpty);
    });

    test('offline is its own kind of failure', () {
      final ApiError error = ApiError.offline();
      expect(error.isOffline, isTrue);
      expect(error.statusCode, 0);
      expect(ApiError.unreadable(500).isOffline, isFalse);
    });

    test('401 is the one status the caller has to branch on', () {
      expect(ApiError.unreadable(401).isUnauthenticated, isTrue);
      expect(ApiError.unreadable(403).isUnauthenticated, isFalse);
    });

    test('describes itself with the status and the code', () {
      expect(ApiError.unreadable(503).toString(), 'ApiError(503 unknown_error)');
    });
  });

  group('InMemoryResponseCache', () {
    test('stores, replaces and clears', () async {
      final InMemoryResponseCache cache = InMemoryResponseCache();
      expect(await cache.read('a'), isNull);

      final DateTime now = DateTime(2026, 9, 21);
      await cache.write(
        'a',
        CachedResponse(body: 'first', storedAt: now, etag: 'W/"1"'),
      );
      expect((await cache.read('a'))?.body, 'first');
      expect((await cache.read('a'))?.etag, 'W/"1"');
      expect((await cache.read('a'))?.storedAt, now);

      await cache.write('a', CachedResponse(body: 'second', storedAt: now));
      expect((await cache.read('a'))?.body, 'second');
      expect((await cache.read('a'))?.etag, isNull);

      await cache.clear();
      expect(await cache.read('a'), isNull);
    });
  });
}
