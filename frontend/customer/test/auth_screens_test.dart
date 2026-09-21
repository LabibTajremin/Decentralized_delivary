import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_customer/src/api/models/auth.dart';
import 'package:goklay_customer/src/dependencies.dart';
import 'package:goklay_customer/src/l10n/customer_strings.dart';
import 'package:goklay_customer/src/screens/onboarding_screen.dart';
import 'package:goklay_customer/src/screens/otp_screen.dart';
import 'package:goklay_customer/src/screens/phone_sign_in_screen.dart';
import 'package:goklay_customer/src/screens/sign_in_options_screen.dart';
import 'package:goklay_customer/src/screens/splash_screen.dart';
import 'package:goklay_customer/src/screens/verification_result_screen.dart';

import 'support/fixtures.dart';
import 'support/harness.dart';

void main() {
  const CustomerStringsBn bn = CustomerStringsBn();
  late FakeBackend backend;

  setUp(() => backend = FakeBackend(<String, Object? Function(SentRequest)>{}));

  group('splash', () {
    testWidgets('waits for storage before answering', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      bool? answer;
      await pumpScreen(
        tester,
        SplashScreen(onReady: (bool signedIn) => answer = signedIn),
        dependencies: dependencies,
      );
      expect(find.text('GoKlay'), findsOneWidget);
      await tester.pump();
      expect(answer, isTrue);
      dependencies.dispose();
    });

    testWidgets('says so when nobody is signed in', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await harnessDependencies(
        backend,
        signedIn: false,
      );
      bool? answer;
      await pumpScreen(
        tester,
        SplashScreen(onReady: (bool signedIn) => answer = signedIn),
        dependencies: dependencies,
      );
      await tester.pump();
      expect(answer, isFalse);
      dependencies.dispose();
    });
  });

  group('onboarding', () {
    testWidgets('walks the three pages and then finishes', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      int finished = 0;
      await pumpScreen(
        tester,
        OnboardingScreen(onFinished: () => finished += 1),
        dependencies: dependencies,
      );
      expect(find.text(bn.onboardingTitle1), findsOneWidget);
      await tester.tap(find.bySemanticsLabel(bn.next));
      await tester.pumpAndSettle();
      expect(find.text(bn.onboardingTitle2), findsOneWidget);
      await tester.tap(find.bySemanticsLabel(bn.next));
      await tester.pumpAndSettle();
      expect(find.text(bn.onboardingTitle3), findsOneWidget);
      await tester.tap(find.bySemanticsLabel(bn.getStarted));
      await tester.pumpAndSettle();
      expect(finished, 1);
      dependencies.dispose();
    });

    testWidgets('skipping finishes straight away', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      int finished = 0;
      await pumpScreen(
        tester,
        OnboardingScreen(onFinished: () => finished += 1),
        dependencies: dependencies,
      );
      await tester.tap(find.bySemanticsLabel(bn.skip));
      await tester.pumpAndSettle();
      expect(finished, 1);
      dependencies.dispose();
    });
  });

  testWidgets('sign-in offers the one method this product has', (
    WidgetTester tester,
  ) async {
    final Dependencies dependencies = await harnessDependencies(backend);
    int continued = 0;
    await pumpScreen(
      tester,
      SignInOptionsScreen(onContinue: () => continued += 1),
      dependencies: dependencies,
    );
    expect(find.byType(GoklayButton), findsOneWidget);
    await tester.tap(find.bySemanticsLabel(bn.continueWithPhone));
    await tester.pumpAndSettle();
    expect(continued, 1);
    dependencies.dispose();
  });

  group('phone sign-in', () {
    testWidgets('the button waits for a number, then sends it', (
      WidgetTester tester,
    ) async {
      backend.routes['POST /v1/auth/otp/request'] = (_) => <String, Object?>{
        'expires_in': 300,
        'resend_after': 2,
      };
      final Dependencies dependencies = await harnessDependencies(backend);
      String? sentPhone;
      await pumpScreen(
        tester,
        PhoneSignInScreen(
          onCodeSent: (String phone, OtpChallenge _) => sentPhone = phone,
        ),
        dependencies: dependencies,
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
      dependencies.dispose();
    });

    testWidgets('a refusal is shown in the server\'s own words', (
      WidgetTester tester,
    ) async {
      backend.routes['POST /v1/auth/otp/request'] =
          (_) => errorBody('rate_limited', 'একটু পরে চেষ্টা করুন');
      backend.statuses['POST /v1/auth/otp/request'] = 429;
      final Dependencies dependencies = await harnessDependencies(backend);
      bool advanced = false;
      await pumpScreen(
        tester,
        PhoneSignInScreen(onCodeSent: (_, _) => advanced = true),
        dependencies: dependencies,
      );
      await tester.enterText(find.byType(TextField), '01712345678');
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(bn.sendCode));
      await tester.pumpAndSettle();
      expect(find.text('একটু পরে চেষ্টা করুন'), findsOneWidget);
      expect(advanced, isFalse);
      dependencies.dispose();
    });
  });

  group('otp', () {
    const OtpChallenge challenge = OtpChallenge(
      expiresIn: Duration(seconds: 300),
      resendAfter: Duration(seconds: 2),
    );

    Future<void> pumpOtp(
      WidgetTester tester,
      Dependencies dependencies, {
      void Function(AuthResult)? onVerified,
      void Function(ApiError)? onFailed,
    }) => pumpScreen(
      tester,
      OtpScreen(
        phone: '01712345678',
        challenge: challenge,
        onVerified: onVerified ?? (_) {},
        onFailed: onFailed ?? (_) {},
      ),
      dependencies: dependencies,
    );

    testWidgets('verify waits for six digits, then stores the tokens', (
      WidgetTester tester,
    ) async {
      backend.routes['POST /v1/auth/otp/verify'] = (_) => tokenPairJson();
      final Dependencies dependencies = await harnessDependencies(
        backend,
        signedIn: false,
      );
      AuthResult? verified;
      await pumpOtp(
        tester,
        dependencies,
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
      expect(verified!.isCustomer, isTrue);
      expect(dependencies.session.accessToken, 'access-1');
      dependencies.dispose();
    });

    testWidgets('a refused code reports the failure upward', (
      WidgetTester tester,
    ) async {
      backend.routes['POST /v1/auth/otp/verify'] =
          (_) => errorBody('invalid_code', 'কোড মেলেনি');
      backend.statuses['POST /v1/auth/otp/verify'] = 401;
      final Dependencies dependencies = await harnessDependencies(
        backend,
        signedIn: false,
      );
      ApiError? failure;
      await pumpOtp(
        tester,
        dependencies,
        onFailed: (ApiError error) => failure = error,
      );
      await tester.enterText(find.byType(TextField), '999999');
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(bn.verifyCode));
      await tester.pumpAndSettle();
      expect(failure!.message, 'কোড মেলেনি');
      dependencies.dispose();
    });

    testWidgets('resend appears only when the server\'s countdown runs out', (
      WidgetTester tester,
    ) async {
      backend.routes['POST /v1/auth/otp/request'] = (_) => <String, Object?>{
        'expires_in': 300,
        'resend_after': 2,
      };
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpOtp(tester, dependencies);
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
      dependencies.dispose();
    });

    testWidgets('a failed resend leaves the screen usable', (
      WidgetTester tester,
    ) async {
      backend.routes['POST /v1/auth/otp/request'] =
          (_) => errorBody('rate_limited', 'পরে চেষ্টা করুন');
      backend.statuses['POST /v1/auth/otp/request'] = 429;
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpOtp(tester, dependencies);
      await tester.pump(const Duration(seconds: 1));
      await tester.pump(const Duration(seconds: 1));
      await tester.pump();
      await tester.tap(find.bySemanticsLabel(bn.resendCode));
      await tester.pump();
      await tester.pump();
      expect(find.text('পরে চেষ্টা করুন'), findsOneWidget);
      dependencies.dispose();
    });
  });

  group('verification result', () {
    testWidgets('success sends the customer on', (WidgetTester tester) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      int continued = 0;
      await pumpScreen(
        tester,
        VerificationResultScreen.success(onContinue: () => continued += 1),
        dependencies: dependencies,
      );
      expect(find.text(bn.verifiedTitle), findsOneWidget);
      expect(find.byIcon(Icons.check_circle), findsOneWidget);
      await tester.tap(find.bySemanticsLabel(bn.getStarted));
      await tester.pumpAndSettle();
      expect(continued, 1);
      dependencies.dispose();
    });

    testWidgets('failure prints the server\'s reason where there is one', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        VerificationResultScreen.failure(
          onContinue: () {},
          error: const ApiError(
            statusCode: 401,
            code: 'invalid_code',
            message: 'কোড মেলেনি',
          ),
        ),
        dependencies: dependencies,
      );
      expect(find.text('কোড মেলেনি'), findsOneWidget);
      expect(find.byIcon(Icons.error_outline), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('failure with no error falls back to the generic line', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        VerificationResultScreen.failure(onContinue: () {}, error: null),
        dependencies: dependencies,
      );
      expect(find.text(bn.verificationFailedBody), findsOneWidget);
      dependencies.dispose();
    });
  });
}
