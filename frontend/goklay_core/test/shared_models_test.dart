import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';

import 'support/harness.dart';

void main() {
  group('json readers', () {
    const Map<String, Object?> full = <String, Object?>{
      'text': 'hello',
      'blank': '',
      'number': 7,
      'decimal': 1.5,
      'wholeAsDouble': 3.0,
      'flag': true,
      'object': <String, Object?>{'a': 1},
      'list': <Object?>[
        <String, Object?>{'a': 1},
        'not an object',
      ],
      'notAList': 'nope',
      'notAnObject': 'nope',
    };

    test('read the values that are there', () {
      expect(readString(full, 'text'), 'hello');
      expect(readOptionalString(full, 'text'), 'hello');
      expect(readInt(full, 'number'), 7);
      expect(readInt(full, 'wholeAsDouble'), 3);
      expect(readDouble(full, 'decimal'), 1.5);
      expect(readBool(full, 'flag'), isTrue);
      expect(readObject(full, 'object'), <String, Object?>{'a': 1});
    });

    test('a missing or blank field has a defined answer, never a throw', () {
      expect(readString(full, 'absent'), '');
      expect(readOptionalString(full, 'absent'), isNull);
      expect(readOptionalString(full, 'blank'), isNull);
      expect(readInt(full, 'absent'), 0);
      expect(readDouble(full, 'absent'), 0);
      expect(readObject(full, 'notAnObject'), isEmpty);
    });

    test('a flag defaults to false, because every flag is a capability', () {
      expect(readBool(full, 'absent'), isFalse);
    });

    test('a list skips entries that are not objects', () {
      expect(
        readList(full, 'list', (Map<String, Object?> j) => readInt(j, 'a')),
        <int>[1],
      );
      expect(
        readList(full, 'notAList', (Map<String, Object?> j) => j),
        isEmpty,
      );
    });
  });

  group('auth', () {
    test('a challenge carries the server\'s two durations', () {
      final OtpChallenge challenge = OtpChallenge.fromJson(<String, Object?>{
        'expires_in': 300,
        'resend_after': 720,
      });
      expect(challenge.expiresIn, const Duration(seconds: 300));
      expect(challenge.resendAfter, const Duration(seconds: 720));
    });

    test('a token is only for the app whose role it carries', () {
      final AuthResult customer = AuthResult.fromJson(tokenPairJson());
      expect(customer.isFor(AuthResult.customerRole), isTrue);
      expect(customer.isFor(AuthResult.merchantRole), isFalse);
      final AuthResult rider = AuthResult.fromJson(
        tokenPairJson(role: 'partner'),
      );
      expect(rider.isFor(AuthResult.partnerRole), isTrue);
      expect(rider.isFor(AuthResult.customerRole), isFalse);
    });

    test('a device session carries a label and a time, never a token', () {
      final DeviceSession session = DeviceSession.fromJson(<String, Object?>{
        'session_id': 'ses-1',
        'device': 'Pixel 8',
        'last_seen_at': '2026-09-21T10:00:00Z',
        'current': true,
      });
      expect(session.device, 'Pixel 8');
      expect(session.isCurrent, isTrue);
      expect(session.lastSeenAt, isNotNull);
    });
  });

  group('account', () {
    test('display_name is never empty, even when name is', () {
      expect(Profile.fromJson(profileJson(name: '')).hasName, isFalse);
      expect(
        Profile.fromJson(profileJson(name: '')).displayName,
        isNotEmpty,
      );
    });

    test('an address carries the server\'s one-line rendering', () {
      final Address address = Address.fromJson(addressJson());
      expect(address.singleLine, 'রোড ৫, ধানমন্ডি, ঢাকা');
      expect(address.isDefault, isTrue);
      expect(address.lat, 23.7461);
    });

    test('an area names the division D3 will not let a search cross', () {
      expect(Area.fromJson(areaJson()).divisionCode, 'DHA');
    });
  });

  test('money is carried, never computed', () {
    final Money amount = Money.fromJson(money(32000, '৳ ৩২০'));
    expect(amount.minor, 32000);
    expect(amount.display, '৳ ৩২০');
  });
}
