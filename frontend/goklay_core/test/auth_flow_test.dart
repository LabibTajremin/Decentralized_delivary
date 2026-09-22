import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';

import 'support/harness.dart';

/// The sign-in flow, which lives here because all three apps have it and only
/// the role differs.
void main() {
  const GoklayStringsBn bn = GoklayStringsBn();
  late FakeBackend backend;
  late GoklayApiClient client;
  late AuthApi auth;
  late Session session;

  void build({bool signedIn = true}) {
    session = Session(
      InMemoryTokenStorage(
        signedIn
            ? const StoredTokens(accessToken: 'a', refreshToken: 'r')
            : null,
      ),
    );
    client = GoklayApiClient(
      baseUrl: Uri.parse('http://api.test'),
      httpClient: backend.client,
      accessToken: () => session.accessToken,
    );
    auth = AuthApi(client);
  }

  setUp(() {
    backend = FakeBackend(<String, Object? Function(SentRequest)>{});
    build();
  });

  tearDown(() {
    session.dispose();
    client.close();
  });

  group('splash', () {
    testWidgets('waits for storage before answering', (
      WidgetTester tester,
    ) async {
      bool? answer;
      await pumpWidgetUnderTest(
        tester,
        GoklaySplashScreen(
          session: session,
          onReady: (bool signedIn) => answer = signedIn,
        ),
      );
      expect(find.text('GoKlay'), findsOneWidget);
      await tester.pump();
      expect(answer, isTrue);
    });

    testWidgets('says so when nobody is signed in', (
      WidgetTester tester,
    ) async {
      build(signedIn: false);
      bool? answer;
      await pumpWidgetUnderTest(
        tester,
        GoklaySplashScreen(
          session: session,
          onReady: (bool signedIn) => answer = signedIn,
        ),
      );
      await tester.pump();
      expect(answer, isFalse);
    });
  });

  group('phone sign-in', () {
    testWidgets('the button waits for a number, then sends it', (
      WidgetTester tester,
    ) async {
      backend.routes['POST /v1/auth/otp/request'] = (_) => <String, Object?>{
        'expires_in': 300,
        'resend_after': 2,
      };
      String? sentPhone;
      await pumpWidgetUnderTest(
        tester,
        PhoneSignInScreen(
          auth: auth,
          onCodeSent: (String phone, OtpChallenge _) => sentPhone = phone,
        ),
      );
      expect(
        tester.widget<GoklayButton>(find.byType(GoklayButton)).isEnabled,
        isFalse,
      );
      await tester.enterText(find.byType(TextField), ' 01712345678 ');
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(bn.sendCode));
      await tester.pumpAndSettle();
      expect(sentPhone, '01712345678');
    });

    testWidgets('a refusal is shown in the server\'s own words', (
      WidgetTester tester,
    ) async {
      backend.routes['POST /v1/auth/otp/request'] =
          (_) => errorBody('rate_limited', 'একটু পরে চেষ্টা করুন');
      backend.statuses['POST /v1/auth/otp/request'] = 429;
      bool advanced = false;
      await pumpWidgetUnderTest(
        tester,
        PhoneSignInScreen(auth: auth, onCodeSent: (_, _) => advanced = true),
      );
      await tester.enterText(find.byType(TextField), '01712345678');
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(bn.sendCode));
      await tester.pumpAndSettle();
      expect(find.text('একটু পরে চেষ্টা করুন'), findsOneWidget);
      expect(advanced, isFalse);
    });
  });

  group('otp', () {
    const OtpChallenge challenge = OtpChallenge(
      expiresIn: Duration(seconds: 300),
      resendAfter: Duration(seconds: 2),
    );

    Future<void> pumpOtp(
      WidgetTester tester, {
      void Function(AuthResult)? onVerified,
      void Function(ApiError)? onFailed,
    }) => pumpWidgetUnderTest(
      tester,
      OtpScreen(
        auth: auth,
        session: session,
        role: AuthResult.customerRole,
        phone: '01712345678',
        challenge: challenge,
        onVerified: onVerified ?? (_) {},
        onFailed: onFailed ?? (_) {},
      ),
    );

    testWidgets('verify waits for six digits, then stores the tokens', (
      WidgetTester tester,
    ) async {
      backend.routes['POST /v1/auth/otp/verify'] = (_) => tokenPairJson();
      build(signedIn: false);
      AuthResult? verified;
      await pumpOtp(
        tester,
        onVerified: (AuthResult result) => verified = result,
      );
      await tester.enterText(find.byType(TextField), '12345');
      await tester.pumpAndSettle();
      expect(
        tester
            .widget<GoklayButton>(
              find.widgetWithText(GoklayButton, bn.verifyCode),
            )
            .isEnabled,
        isFalse,
      );
      await tester.enterText(find.byType(TextField), '123456');
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(bn.verifyCode));
      await tester.pumpAndSettle();
      expect(verified!.isFor(AuthResult.customerRole), isTrue);
      expect(session.accessToken, 'access-1');
    });

    testWidgets('a demo challenge shows the code and fills the field', (
      WidgetTester tester,
    ) async {
      backend.routes['POST /v1/auth/otp/verify'] = (_) => tokenPairJson();
      build(signedIn: false);
      await pumpWidgetUnderTest(
        tester,
        OtpScreen(
          auth: auth,
          session: session,
          role: AuthResult.customerRole,
          phone: '01712345678',
          challenge: const OtpChallenge(
            expiresIn: Duration(seconds: 300),
            resendAfter: Duration(seconds: 2),
            demoCode: '482913',
          ),
          onVerified: (_) {},
          onFailed: (_) {},
        ),
      );

      // Shown, with a line saying why there is a code on screen at all.
      expect(find.text(bn.demoCodeLabel), findsOneWidget);
      expect(find.text('482913'), findsWidgets);

      // And filled in, so the button is live without retyping what the
      // screen has just displayed.
      expect(
        tester
            .widget<GoklayButton>(
              find.widgetWithText(GoklayButton, bn.verifyCode),
            )
            .isEnabled,
        isTrue,
      );
    });

    testWidgets('an ordinary challenge shows no code and fills nothing', (
      WidgetTester tester,
    ) async {
      build(signedIn: false);
      await pumpOtp(tester);
      expect(find.text(bn.demoCodeLabel), findsNothing);
      expect(
        tester.widget<TextField>(find.byType(TextField)).controller!.text,
        isEmpty,
      );
    });

    testWidgets('asking again replaces the demo code with the new one', (
      WidgetTester tester,
    ) async {
      // The old code stops working the moment a new one is issued, so a
      // screen that went on showing it would be telling the visitor to type
      // something that is now wrong.
      backend.routes['POST /v1/auth/otp/request'] = (_) => <String, Object?>{
        'expires_in': 300,
        'resend_after': 2,
        'demo_code': '111111',
      };
      build(signedIn: false);
      await pumpWidgetUnderTest(
        tester,
        OtpScreen(
          auth: auth,
          session: session,
          role: AuthResult.customerRole,
          phone: '01712345678',
          challenge: const OtpChallenge(
            expiresIn: Duration(seconds: 300),
            resendAfter: Duration(seconds: 2),
            demoCode: '482913',
          ),
          onVerified: (_) {},
          onFailed: (_) {},
        ),
      );
      expect(find.text('482913'), findsWidgets);

      await tester.pump(const Duration(seconds: 3));
      await tester.tap(find.bySemanticsLabel(bn.resendCode));
      await tester.pumpAndSettle();

      expect(find.text('111111'), findsWidgets);
      expect(find.text('482913'), findsNothing);
    });

    testWidgets('a refused code reports the failure upward', (
      WidgetTester tester,
    ) async {
      backend.routes['POST /v1/auth/otp/verify'] =
          (_) => errorBody('invalid_code', 'কোড মেলেনি');
      backend.statuses['POST /v1/auth/otp/verify'] = 401;
      build(signedIn: false);
      ApiError? failure;
      await pumpOtp(
        tester,
        onFailed: (ApiError error) => failure = error,
      );
      await tester.enterText(find.byType(TextField), '999999');
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(bn.verifyCode));
      await tester.pumpAndSettle();
      expect(failure!.message, 'কোড মেলেনি');
    });

    testWidgets('resend appears only when the server\'s countdown runs out', (
      WidgetTester tester,
    ) async {
      backend.routes['POST /v1/auth/otp/request'] = (_) => <String, Object?>{
        'expires_in': 300,
        'resend_after': 2,
      };
      await pumpOtp(tester);
      expect(find.textContaining(bn.resendCountdown), findsOneWidget);
      expect(find.text(bn.resendCode), findsNothing);

      await tester.pump(const Duration(seconds: 1));
      await tester.pump(const Duration(seconds: 1));
      await tester.pump();
      expect(find.text(bn.resendCode), findsOneWidget);

      await tester.tap(find.bySemanticsLabel(bn.resendCode));
      await tester.pump();
      await tester.pump();
      expect(backend.to('/v1/auth/otp/request'), hasLength(1));
      // The countdown restarts from the fresh challenge.
      expect(find.textContaining(bn.resendCountdown), findsOneWidget);
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('a failed resend leaves the screen usable', (
      WidgetTester tester,
    ) async {
      backend.routes['POST /v1/auth/otp/request'] =
          (_) => errorBody('rate_limited', 'পরে চেষ্টা করুন');
      backend.statuses['POST /v1/auth/otp/request'] = 429;
      await pumpOtp(tester);
      await tester.pump(const Duration(seconds: 1));
      await tester.pump(const Duration(seconds: 1));
      await tester.pump();
      await tester.tap(find.bySemanticsLabel(bn.resendCode));
      await tester.pump();
      await tester.pump();
      expect(find.text('পরে চেষ্টা করুন'), findsOneWidget);
    });
  });

  group('verification result', () {
    testWidgets('success sends the customer on', (WidgetTester tester) async {
      int continued = 0;
      await pumpWidgetUnderTest(
        tester,
        VerificationResultScreen.success(onContinue: () => continued += 1),
      );
      expect(find.text(bn.verifiedTitle), findsOneWidget);
      expect(find.byIcon(Icons.check_circle), findsOneWidget);
      await tester.tap(find.bySemanticsLabel(bn.getStarted));
      await tester.pumpAndSettle();
      expect(continued, 1);
    });

    testWidgets('failure prints the server\'s reason where there is one', (
      WidgetTester tester,
    ) async {
      await pumpWidgetUnderTest(
        tester,
        VerificationResultScreen.failure(
          onContinue: () {},
          error: const ApiError(
            statusCode: 401,
            code: 'invalid_code',
            message: 'কোড মেলেনি',
          ),
        ),
      );
      expect(find.text('কোড মেলেনি'), findsOneWidget);
      expect(find.byIcon(Icons.error_outline), findsOneWidget);
    });

    testWidgets('failure with no error falls back to the generic line', (
      WidgetTester tester,
    ) async {
      await pumpWidgetUnderTest(
        tester,
        VerificationResultScreen.failure(onContinue: () {}, error: null),
      );
      expect(find.text(bn.verificationFailedBody), findsOneWidget);
    });
  });
}
