import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_partner/src/app.dart';
import 'package:goklay_partner/src/app_scope.dart';
import 'package:goklay_partner/src/dependencies.dart';
import 'package:goklay_partner/src/environment.dart';
import 'package:goklay_partner/src/l10n/partner_strings.dart';
import 'package:goklay_partner/src/screens/account_screen.dart';
import 'package:goklay_partner/src/screens/cash_screen.dart';
import 'package:goklay_partner/src/screens/feed_screen.dart';
import 'package:goklay_partner/src/screens/job_screen.dart';
import 'package:goklay_partner/src/screens/jobs_screen.dart';
import 'package:goklay_partner/src/screens/main_shell.dart';
import 'package:goklay_partner/src/screens/register_screen.dart';

import 'support/fixtures.dart';
import 'support/harness.dart';

/// Everything a signed-up rider's first screens ask for.
Map<String, Object? Function(SentRequest)> shellRoutes() =>
    <String, Object? Function(SentRequest)>{
      'GET /v1/partner': (_) => partnerJson(),
      'GET /v1/partner/feed': (_) => feedJson(),
      'GET /v1/partner/jobs': (_) => jobListJson(),
      'GET /v1/partner/cod': (_) => ledgerJson(),
      'GET /v1/me': (_) => profileJson(),
    };

void main() {
  const PartnerStringsBn bn = PartnerStringsBn();
  const GoklayStringsBn core = GoklayStringsBn();
  late FakeBackend backend;

  setUp(() => backend = FakeBackend(shellRoutes()));

  Future<Dependencies> pumpApp(
    WidgetTester tester, {
    bool signedIn = true,
  }) async {
    final Dependencies dependencies = Dependencies(
      environment: PartnerEnvironment(
        apiBaseUrl: Uri.parse('http://api.test'),
      ),
      httpClient: backend.client,
      storage: harnessStorage(signedIn: signedIn),
    );
    await tester.pumpWidget(
      GoklayPartnerApp(
        environment: dependencies.environment,
        dependencies: dependencies,
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));
    await tester.pumpAndSettle();
    return dependencies;
  }

  testWidgets('the app opens in Bengali and can be handed a home', (
    WidgetTester tester,
  ) async {
    final Dependencies dependencies = Dependencies(
      environment: PartnerEnvironment(
        apiBaseUrl: Uri.parse('http://api.test'),
      ),
      httpClient: backend.client,
    );
    await tester.pumpWidget(
      GoklayPartnerApp(
        environment: dependencies.environment,
        dependencies: dependencies,
        home: const Text('replaced', textDirection: TextDirection.ltr),
      ),
    );
    await tester.pump();
    expect(find.text('replaced'), findsOneWidget);
    expect(
      tester.widget<MaterialApp>(find.byType(MaterialApp)).locale,
      const Locale('bn'),
    );
    dependencies.dispose();
  });

  testWidgets('with no graph given it builds its own', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(
      GoklayPartnerApp(
        environment: PartnerEnvironment.fromCompileTime(),
        home: const Text('own graph', textDirection: TextDirection.ltr),
      ),
    );
    await tester.pump();
    expect(find.text('own graph'), findsOneWidget);
  });

  testWidgets('a signed-up rider lands on the feed', (
    WidgetTester tester,
  ) async {
    final Dependencies dependencies = await pumpApp(tester);
    expect(find.byType(PartnerShell), findsOneWidget);
    expect(find.byType(FeedScreen), findsOneWidget);
    dependencies.dispose();
  });

  testWidgets('an account that has not signed up gets the form', (
    WidgetTester tester,
  ) async {
    backend.routes['GET /v1/partner'] =
        (_) => errorBody('not_found', 'রাইডার নেই');
    backend.statuses['GET /v1/partner'] = 404;
    backend.routes['POST /v1/partner'] = (_) => partnerJson();
    final Dependencies dependencies = await pumpApp(tester);
    expect(find.byType(RegisterScreen), findsOneWidget);
    await enterInto(tester, bn.riderName, 'করিম');
    await enterInto(tester, bn.riderPhone, '01811111111');
    await revealAndTap(
      tester,
      find.widgetWithText(GoklayButton, bn.register),
    );
    expect(find.byType(PartnerShell), findsOneWidget);
    dependencies.dispose();
  });

  testWidgets('a read that fails for another reason offers a retry', (
    WidgetTester tester,
  ) async {
    backend.routes['GET /v1/partner'] = (_) => errorBody('b', 'সমস্যা');
    backend.statuses['GET /v1/partner'] = 503;
    final Dependencies dependencies = await pumpApp(tester);
    expect(find.byType(ErrorView), findsOneWidget);
    await tester.tap(find.bySemanticsLabel(core.retry));
    await tester.pumpAndSettle();
    expect(backend.to('/v1/partner'), hasLength(2));
    dependencies.dispose();
  });

  testWidgets('signing in sends the partner role', (
    WidgetTester tester,
  ) async {
    backend.routes['POST /v1/auth/otp/request'] = (_) => <String, Object?>{
      'expires_in': 300,
      'resend_after': 2,
    };
    backend.routes['POST /v1/auth/otp/verify'] = (_) => tokenPairJson();
    final Dependencies dependencies = await pumpApp(tester, signedIn: false);
    await tester.enterText(find.byType(TextField), '01811111111');
    await tester.pumpAndSettle();
    await tester.tap(find.bySemanticsLabel(core.sendCode));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField), '123456');
    await tester.pumpAndSettle();
    await tester.tap(find.bySemanticsLabel(core.verifyCode));
    await tester.pumpAndSettle();
    expect(
      (backend.to('/v1/auth/otp/verify').single.body!
          as Map<String, Object?>)['role'],
      'partner',
    );
    expect(find.byType(PartnerShell), findsOneWidget);
    dependencies.dispose();
  });

  testWidgets('a refused code lands on the failure screen and starts over', (
    WidgetTester tester,
  ) async {
    backend.routes['POST /v1/auth/otp/request'] = (_) => <String, Object?>{
      'expires_in': 300,
      'resend_after': 2,
    };
    backend.routes['POST /v1/auth/otp/verify'] =
        (_) => errorBody('invalid_code', 'কোড মেলেনি');
    backend.statuses['POST /v1/auth/otp/verify'] = 401;
    final Dependencies dependencies = await pumpApp(tester, signedIn: false);
    await tester.enterText(find.byType(TextField), '01811111111');
    await tester.pumpAndSettle();
    await tester.tap(find.bySemanticsLabel(core.sendCode));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField), '999999');
    await tester.pumpAndSettle();
    await tester.tap(find.bySemanticsLabel(core.verifyCode));
    await tester.pumpAndSettle();
    expect(find.text('কোড মেলেনি'), findsOneWidget);
    await tester.tap(find.bySemanticsLabel(core.startOver));
    await tester.pumpAndSettle();
    expect(find.text(core.phoneLabel), findsOneWidget);
    dependencies.dispose();
  });

  group('the tabs', () {
    testWidgets('each one opens its own screen', (WidgetTester tester) async {
      final Dependencies dependencies = await pumpApp(tester);
      await tester.tap(find.text(bn.jobsTitle));
      await tester.pumpAndSettle();
      expect(find.byType(JobsScreen), findsOneWidget);

      await tester.tap(find.text(bn.cashTitle));
      await tester.pumpAndSettle();
      expect(find.byType(CashScreen), findsOneWidget);

      await tester.tap(find.text(bn.accountTitle));
      await tester.pumpAndSettle();
      expect(find.byType(PartnerAccountScreen), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('the feed opens a job', (WidgetTester tester) async {
      final Dependencies dependencies = await pumpApp(tester);
      await tester.tap(find.bySemanticsLabel('GK-7F3K').first);
      await tester.pumpAndSettle();
      expect(find.byType(JobScreen), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('the deliveries tab opens a job too', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await pumpApp(tester);
      await tester.tap(find.text(bn.jobsTitle));
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel('GK-7F3K').first);
      await tester.pumpAndSettle();
      expect(find.byType(JobScreen), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('signing out returns to the phone screen', (
      WidgetTester tester,
    ) async {
      backend.routes['POST /v1/auth/logout'] = (_) => null;
      final Dependencies dependencies = await pumpApp(tester);
      await tester.tap(find.text(bn.accountTitle));
      await tester.pumpAndSettle();
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.signOut),
      );
      expect(find.text(core.phoneLabel), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('the language switch rebuilds the whole app', (
      WidgetTester tester,
    ) async {
      backend.routes['PATCH /v1/me'] = (_) => profileJson();
      final Dependencies dependencies = await pumpApp(tester);
      await tester.tap(find.text(bn.accountTitle));
      await tester.pumpAndSettle();
      await revealAndTap(tester, find.widgetWithText(SettingRow, 'English'));
      expect(
        tester.widget<MaterialApp>(find.byType(MaterialApp)).locale,
        const Locale('en'),
      );
      dependencies.dispose();
    });
  });

  group('the outbox banner', () {
    testWidgets('it is invisible until something is queued', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await pumpApp(tester);
      expect(find.text(bn.sendNow), findsNothing);
      dependencies.dispose();
    });

    testWidgets('a queued tap shows the banner, and sending drains it', (
      WidgetTester tester,
    ) async {
      backend.routes['POST /v1/partner/jobs/job-1/collect'] =
          (_) => jobJson(status: 'collected');
      final Dependencies dependencies = await pumpApp(tester);
      dependencies.outbox.enqueue(
        const QueuedAction(
          id: 'a',
          method: 'POST',
          path: '/v1/partner/jobs/job-1/collect',
        ),
      );
      // The banner is rebuilt when the shell is, which a tab change does.
      await tester.tap(find.text(bn.jobsTitle));
      await tester.pumpAndSettle();
      expect(find.textContaining(bn.waitingToSend), findsOneWidget);
      await tester.tap(find.bySemanticsLabel(bn.sendNow));
      await tester.pumpAndSettle();
      expect(dependencies.outbox.isEmpty, isTrue);
      expect(backend.to('/v1/partner/jobs/job-1/collect'), hasLength(1));
      dependencies.dispose();
    });

    testWidgets('a queued tap the server refuses is reported', (
      WidgetTester tester,
    ) async {
      backend.routes['POST /v1/partner/jobs/job-1/deliver'] =
          (_) => errorBody('bad_state', 'এখনো নেওয়া হয়নি');
      backend.statuses['POST /v1/partner/jobs/job-1/deliver'] = 409;
      final Dependencies dependencies = await pumpApp(tester);
      dependencies.outbox.enqueue(
        const QueuedAction(
          id: 'b',
          method: 'POST',
          path: '/v1/partner/jobs/job-1/deliver',
        ),
      );
      await tester.tap(find.text(bn.jobsTitle));
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(bn.sendNow));
      await tester.pumpAndSettle();
      expect(find.text(bn.queuedActionRefused), findsOneWidget);
      expect(dependencies.outbox.isEmpty, isTrue);
      dependencies.dispose();
    });
  });

  testWidgets('the scope is found from anywhere below, and notices a swap', (
    WidgetTester tester,
  ) async {
    final Dependencies first = Dependencies(
      environment: PartnerEnvironment(
        apiBaseUrl: Uri.parse('http://api.test'),
      ),
      httpClient: backend.client,
    );
    late Dependencies seen;
    Widget scope(Dependencies dependencies) => PartnerScope(
      dependencies: dependencies,
      child: Builder(
        builder: (BuildContext context) {
          seen = PartnerScope.of(context);
          return const SizedBox.shrink();
        },
      ),
    );
    await tester.pumpWidget(scope(first));
    expect(seen, same(first));
    final Dependencies second = Dependencies(
      environment: first.environment,
      httpClient: backend.client,
    );
    await tester.pumpWidget(scope(second));
    expect(seen, same(second));
    first.dispose();
    second.dispose();
  });
}
