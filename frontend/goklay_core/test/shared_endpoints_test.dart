import 'dart:ui' show Locale;

import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';

import 'support/harness.dart';

void main() {
  late FakeBackend backend;
  late GoklayApiClient client;
  late AuthApi auth;
  late AccountApi account;
  late Session session;

  Map<String, Object?> lastBody(String path) =>
      backend.to(path).last.body! as Map<String, Object?>;

  setUp(() async {
    backend = FakeBackend(<String, Object? Function(SentRequest)>{});
    session = Session(
      InMemoryTokenStorage(
        const StoredTokens(accessToken: 'access', refreshToken: 'refresh'),
      ),
    );
    await session.restore();
    client = GoklayApiClient(
      baseUrl: Uri.parse('http://api.test'),
      httpClient: backend.client,
      accessToken: () => session.accessToken,
    );
    auth = AuthApi(client);
    account = AccountApi(client);
  });

  tearDown(() {
    session.dispose();
    client.close();
  });

  void route(String key, Object? Function(SentRequest request) handler) =>
      backend.routes[key] = handler;

  group('auth', () {
    test('requesting a code sends the number and reads the countdown', () async {
      route('POST /v1/auth/otp/request', (_) => <String, Object?>{
        'phone': '+88017*****678',
        'expires_in': 300,
        'resend_after': 720,
      });
      final OtpChallenge challenge = await auth.requestOtp(
        '01712345678',
      );
      expect(challenge.resendAfter, const Duration(seconds: 720));
      expect(lastBody('/v1/auth/otp/request')['phone'], '01712345678');
    });

    test('the app states its own role rather than discovering it', () async {
      route('POST /v1/auth/otp/verify', (_) => tokenPairJson());
      final AuthResult result = await auth.verifyOtp(
        phone: '01712345678',
        code: '123456',
        role: AuthResult.customerRole,
        device: 'Pixel 8',
      );
      expect(result.isFor(AuthResult.customerRole), isTrue);
      expect(result.isFor(AuthResult.partnerRole), isFalse);
      final Map<String, Object?> body = lastBody('/v1/auth/otp/verify');
      expect(body['role'], 'customer');
      expect(body['device'], 'Pixel 8');
    });

    test('each app sends its own role', () async {
      route('POST /v1/auth/otp/verify', (_) => tokenPairJson(role: 'partner'));
      await auth.verifyOtp(
        phone: '017',
        code: '123456',
        role: AuthResult.partnerRole,
      );
      expect(lastBody('/v1/auth/otp/verify')['role'], 'partner');
      route('POST /v1/auth/otp/verify', (_) => tokenPairJson(role: 'merchant'));
      await auth.verifyOtp(
        phone: '017',
        code: '123456',
        role: AuthResult.merchantRole,
      );
      expect(lastBody('/v1/auth/otp/verify')['role'], 'merchant');
    });

    test('an empty device name is left off rather than sent blank', () async {
      route('POST /v1/auth/otp/verify', (_) => tokenPairJson());
      await auth.verifyOtp(
        phone: '017',
        code: '123456',
        role: AuthResult.customerRole,
      );
      expect(lastBody('/v1/auth/otp/verify').containsKey('device'), isFalse);
    });

    test('refresh exchanges the stored token', () async {
      route('POST /v1/auth/refresh', (_) => tokenPairJson());
      await auth.refresh('refresh-0');
      expect(lastBody('/v1/auth/refresh')['refresh_token'], 'refresh-0');
    });

    test('logout and logout-all are plain posts', () async {
      route('POST /v1/auth/logout', (_) => null);
      route('POST /v1/auth/logout-all', (_) => null);
      await auth.logout();
      await auth.logoutAll();
      expect(backend.to('/v1/auth/logout'), hasLength(1));
      expect(backend.to('/v1/auth/logout-all'), hasLength(1));
    });

    test('the device list is read out of its envelope', () async {
      route('GET /v1/auth/sessions', (_) => <String, Object?>{
        'sessions': <Object?>[
          <String, Object?>{
            'session_id': 'ses-1',
            'device': 'Pixel 8',
            'current': true,
          },
          'not an object',
        ],
      });
      final ApiPage<List<DeviceSession>> page =
          await auth.sessions();
      expect(page.value, hasLength(1));
      expect(page.value.single.device, 'Pixel 8');
    });

    test('a missing sessions array is an empty list, not a throw', () async {
      route('GET /v1/auth/sessions', (_) => <String, Object?>{});
      expect((await auth.sessions()).value, isEmpty);
    });
  });

  group('account', () {
    test('the profile read carries its freshness through to the store', () async {
      route('GET /v1/me', (_) => profileJson());
      final ApiPage<Profile> page = await account.profile();
      expect(page.value.displayName, 'রিয়া');
      expect(page.fromCache, isFalse);
      expect(page.toAsyncData(), isA<AsyncData<Profile>>());
    });

    test('a patch sends only the fields that were given', () async {
      route('PATCH /v1/me', (_) => profileJson());
      await account.updateProfile(name: 'নাদিয়া');
      final Map<String, Object?> body = lastBody('/v1/me');
      expect(body, <String, Object?>{'name': 'নাদিয়া'});
    });

    test('an email or a language can be patched on their own', () async {
      route('PATCH /v1/me', (_) => profileJson());
      await account.updateProfile(email: 'riya@goklay.test');
      expect(lastBody('/v1/me'), <String, Object?>{
        'email': 'riya@goklay.test',
      });
      await account.updateProfile(language: 'en');
      expect(lastBody('/v1/me'), <String, Object?>{'language': 'en'});
    });

    test('the address book skips entries that are not objects', () async {
      route('GET /v1/me/addresses', (_) => <String, Object?>{
        'addresses': <Object?>[addressJson(), 7],
      });
      final ApiPage<List<Address>> page = await account.addresses();
      expect(page.value, hasLength(1));
    });

    test('a missing addresses array is an empty book', () async {
      route('GET /v1/me/addresses', (_) => <String, Object?>{});
      expect((await account.addresses()).value, isEmpty);
    });

    test('a new address sends no area, district or division', () async {
      route('POST /v1/me/addresses', (_) => addressJson());
      await account.addAddress(
        label: 'বাসা',
        recipientName: 'রিয়া',
        recipientPhone: '01712345678',
        line1: 'রোড ৫',
        lat: 23.7461,
        lng: 90.3742,
        makeDefault: true,
      );
      final Map<String, Object?> body = lastBody('/v1/me/addresses');
      expect(body['lat'], 23.7461);
      expect(body['make_default'], isTrue);
      expect(body.containsKey('area_code'), isFalse);
      expect(body.containsKey('division_code'), isFalse);
    });

    test('an address can be removed and another made default', () async {
      route('DELETE /v1/me/addresses/adr-1', (_) => null);
      route('POST /v1/me/addresses/adr-2/default', (_) => addressJson(id: 'adr-2'));
      await account.deleteAddress('adr-1');
      final Address promoted = await account.makeDefault('adr-2');
      expect(promoted.id, 'adr-2');
    });
  });

  group('the transport itself', () {
    test('the bearer token is read fresh from the session', () async {
      route('GET /v1/me', (_) => profileJson());
      await account.profile();
      expect(
        backend.to('/v1/me').last.headers['authorization'],
        'Bearer access',
      );
    });

    test('Bengali sends no lang parameter; English sends one', () async {
      route('GET /v1/me', (_) => profileJson());
      await account.profile();
      expect(
        backend.to('/v1/me').last.url.queryParameters.containsKey('lang'),
        isFalse,
      );
      client.locale = const Locale('en');
      await account.profile();
      expect(backend.to('/v1/me').last.url.queryParameters['lang'], 'en');
    });

    test('a failure arrives as the server\'s own sentence', () async {
      route('GET /v1/me', (_) => errorBody('config_unavailable', 'পরে দেখুন'));
      backend.statuses['GET /v1/me'] = 503;
      await expectLater(
        account.profile(),
        throwsA(
          isA<ApiError>()
              .having((ApiError e) => e.code, 'code', 'config_unavailable')
              .having((ApiError e) => e.message, 'message', 'পরে দেখুন'),
        ),
      );
    });

    test('signing out clears the tokens and the cached screens', () async {
      route('GET /v1/me', (_) => profileJson());
      await account.profile();
      await session.signOut();
      await client.cache.clear();
      expect(session.isSignedIn, isFalse);
      expect(await client.cached('/v1/me'), isNull);
    });
  });
}
