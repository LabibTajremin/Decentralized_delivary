import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_partner/src/api/models/job.dart';
import 'package:goklay_partner/src/api/models/partner.dart';
import 'package:goklay_partner/src/dependencies.dart';
import 'package:goklay_partner/src/l10n/partner_strings.dart';
import 'package:goklay_partner/src/screens/account_screen.dart';
import 'package:goklay_partner/src/screens/cash_screen.dart';
import 'package:goklay_partner/src/screens/feed_screen.dart';
import 'package:goklay_partner/src/screens/job_screen.dart';
import 'package:goklay_partner/src/screens/jobs_screen.dart';
import 'package:goklay_partner/src/screens/register_screen.dart';

import 'support/fixtures.dart';
import 'support/harness.dart';

void main() {
  const PartnerStringsBn bn = PartnerStringsBn();
  const GoklayStringsBn core = GoklayStringsBn();
  late FakeBackend backend;

  setUp(() => backend = FakeBackend(<String, Object? Function(SentRequest)>{}));

  void route(String key, Object? Function(SentRequest) handler) =>
      backend.routes[key] = handler;

  group('signing up', () {
    testWidgets('nothing is sent without a name and a number', (
      WidgetTester tester,
    ) async {
      route('POST /v1/partner', (_) => partnerJson());
      final Dependencies dependencies = await harnessDependencies(backend);
      DeliveryPartner? registered;
      await pumpScreen(
        tester,
        RegisterScreen(
          onRegistered: (DeliveryPartner p) => registered = p,
        ),
        dependencies: dependencies,
      );
      await reveal(tester, find.widgetWithText(GoklayButton, bn.register));
      expect(
        tester
            .widget<GoklayButton>(
              find.widgetWithText(GoklayButton, bn.register),
            )
            .isEnabled,
        isFalse,
      );
      await enterInto(tester, bn.riderName, 'করিম');
      await enterInto(tester, bn.riderPhone, '01811111111');
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.register),
      );
      expect(registered!.name, 'করিম');
      dependencies.dispose();
    });

    testWidgets('a refusal is shown in the server\'s words', (
      WidgetTester tester,
    ) async {
      route('POST /v1/partner', (_) => errorBody('bad_phone', 'নম্বরটি ভুল'));
      backend.statuses['POST /v1/partner'] = 400;
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        RegisterScreen(onRegistered: (_) {}),
        dependencies: dependencies,
      );
      await enterInto(tester, bn.riderName, 'করিম');
      await enterInto(tester, bn.riderPhone, '018');
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.register),
      );
      expect(find.text('নম্বরটি ভুল'), findsOneWidget);
      dependencies.dispose();
    });
  });

  group('the feed', () {
    Future<Dependencies> pumpFeed(
      WidgetTester tester, {
      void Function(DeliveryJob)? onJob,
    }) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        FeedScreen(onJobSelected: onJob ?? (_) {}),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      return dependencies;
    }

    testWidgets('an offer shows the shop, the distances and both answers', (
      WidgetTester tester,
    ) async {
      route('GET /v1/partner/feed', (_) => feedJson());
      final Dependencies dependencies = await pumpFeed(tester);
      expect(find.text('নূরজাহান হোটেল'), findsOneWidget);
      expect(find.text('৩.২ কিমি'), findsOneWidget);
      expect(find.textContaining('৪০০ মিটার'), findsOneWidget);
      expect(find.text(bn.acceptJob), findsOneWidget);
      expect(find.text(bn.declineJob), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('an empty feed says why in the server\'s sentence', (
      WidgetTester tester,
    ) async {
      route(
        'GET /v1/partner/feed',
        (_) => feedJson(
          jobs: <Map<String, Object?>>[],
          availability: 'offline',
          reason: 'offline',
          notice: 'আপনি অফলাইন আছেন',
        ),
      );
      final Dependencies dependencies = await pumpFeed(tester);
      expect(find.text('আপনি অফলাইন আছেন'), findsOneWidget);
      expect(find.text(bn.goOnShift), findsOneWidget);
      expect(find.text(bn.acceptJob), findsNothing);
      dependencies.dispose();
    });

    testWidgets('going on shift posts available and re-reads', (
      WidgetTester tester,
    ) async {
      route(
        'GET /v1/partner/feed',
        (_) => feedJson(jobs: <Map<String, Object?>>[], availability: 'offline'),
      );
      route(
        'PUT /v1/partner/availability',
        (_) => partnerJson(),
      );
      final Dependencies dependencies = await pumpFeed(tester);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.goOnShift),
      );
      expect(
        (backend.to('/v1/partner/availability').single.body!
            as Map<String, Object?>)['availability'],
        'available',
      );
      expect(backend.to('/v1/partner/feed'), hasLength(2));
      dependencies.dispose();
    });

    testWidgets('going off shift posts offline', (WidgetTester tester) async {
      route('GET /v1/partner/feed', (_) => feedJson());
      route('PUT /v1/partner/availability', (_) => partnerJson());
      final Dependencies dependencies = await pumpFeed(tester);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.goOffShift),
      );
      expect(
        (backend.to('/v1/partner/availability').single.body!
            as Map<String, Object?>)['availability'],
        'offline',
      );
      dependencies.dispose();
    });

    testWidgets('accepting and declining both re-read the feed', (
      WidgetTester tester,
    ) async {
      route('GET /v1/partner/feed', (_) => feedJson());
      route('POST /v1/partner/jobs/job-1/accept', (_) => jobJson());
      route('POST /v1/partner/jobs/job-1/decline', (_) => jobJson());
      final Dependencies dependencies = await pumpFeed(tester);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.acceptJob),
      );
      expect(backend.to('/v1/partner/jobs/job-1/accept'), hasLength(1));
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.declineJob),
      );
      expect(backend.to('/v1/partner/jobs/job-1/decline'), hasLength(1));
      dependencies.dispose();
    });

    testWidgets('a job somebody else took is reported, not hidden', (
      WidgetTester tester,
    ) async {
      route('GET /v1/partner/feed', (_) => feedJson());
      route(
        'POST /v1/partner/jobs/job-1/accept',
        (_) => errorBody('already_taken', 'অন্য একজন নিয়ে নিয়েছেন'),
      );
      backend.statuses['POST /v1/partner/jobs/job-1/accept'] = 409;
      final Dependencies dependencies = await pumpFeed(tester);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.acceptJob),
      );
      expect(find.text('অন্য একজন নিয়ে নিয়েছেন'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('tapping a job opens it', (WidgetTester tester) async {
      route('GET /v1/partner/feed', (_) => feedJson());
      DeliveryJob? opened;
      final Dependencies dependencies = await pumpFeed(
        tester,
        onJob: (DeliveryJob job) => opened = job,
      );
      await tester.tap(find.bySemanticsLabel('GK-7F3K'));
      await tester.pumpAndSettle();
      expect(opened!.id, 'job-1');
      dependencies.dispose();
    });

    testWidgets('a failed read offers a retry', (WidgetTester tester) async {
      route('GET /v1/partner/feed', (_) => errorBody('b', 'সমস্যা'));
      backend.statuses['GET /v1/partner/feed'] = 503;
      final Dependencies dependencies = await pumpFeed(tester);
      await tester.tap(find.bySemanticsLabel(core.retry));
      await tester.pumpAndSettle();
      expect(backend.to('/v1/partner/feed'), hasLength(2));
      dependencies.dispose();
    });
  });

  group('my deliveries', () {
    testWidgets('the two tabs each re-read with their own filter', (
      WidgetTester tester,
    ) async {
      route('GET /v1/partner/jobs', (_) => jobListJson());
      final Dependencies dependencies = await harnessDependencies(backend);
      DeliveryJob? opened;
      await pumpScreen(
        tester,
        JobsScreen(onJobSelected: (DeliveryJob job) => opened = job),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      expect(find.text('GK-7F3K'), findsOneWidget);
      await tester.tap(find.bySemanticsLabel('GK-7F3K'));
      await tester.pumpAndSettle();
      expect(opened, isNotNull);

      await tester.tap(find.text(bn.jobsPast));
      await tester.pumpAndSettle();
      expect(
        backend.to('/v1/partner/jobs').last.url.queryParameters
            .containsKey('live'),
        isFalse,
      );
      await tester.tap(find.text(bn.jobsLive));
      await tester.pumpAndSettle();
      expect(
        backend.to('/v1/partner/jobs').last.url.queryParameters['live'],
        'true',
      );
      dependencies.dispose();
    });

    testWidgets('an empty list says so, and a failure retries', (
      WidgetTester tester,
    ) async {
      route(
        'GET /v1/partner/jobs',
        (_) => jobListJson(jobs: <Map<String, Object?>>[]),
      );
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        JobsScreen(onJobSelected: (_) {}),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      expect(find.text(bn.noJobs), findsOneWidget);

      backend.routes['GET /v1/partner/jobs'] = (_) => errorBody('b', 'সমস্যা');
      backend.statuses['GET /v1/partner/jobs'] = 503;
      await tester.tap(find.text(bn.jobsPast));
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(core.retry));
      await tester.pumpAndSettle();
      expect(backend.to('/v1/partner/jobs'), hasLength(3));
      dependencies.dispose();
    });
  });

  group('one delivery', () {
    Future<Dependencies> pumpJob(
      WidgetTester tester, {
      String status = 'assigned',
      String reason = '',
    }) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        JobScreen(
          job: DeliveryJob.fromJson(
            jobJson(status: status, reason: reason),
          ),
        ),
        dependencies: dependencies,
      );
      return dependencies;
    }

    testWidgets('it shows both ends and the one thing to do next', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await pumpJob(tester);
      expect(find.text('নূরজাহান হোটেল'), findsOneWidget);
      expect(find.text('রোড ৫, ধানমন্ডি, ঢাকা'), findsOneWidget);
      expect(find.text('01799999999'), findsOneWidget);
      expect(find.text(bn.collect), findsOneWidget);
      expect(find.text(bn.deliver), findsNothing);
      dependencies.dispose();
    });

    testWidgets('collecting moves it on to delivering', (
      WidgetTester tester,
    ) async {
      route(
        'POST /v1/partner/jobs/job-1/collect',
        (_) => jobJson(status: 'collected'),
      );
      final Dependencies dependencies = await pumpJob(tester);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.collect),
      );
      await reveal(tester, find.widgetWithText(GoklayButton, bn.deliver));
      expect(find.widgetWithText(GoklayButton, bn.deliver), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('delivering is posted from the collected state', (
      WidgetTester tester,
    ) async {
      route(
        'POST /v1/partner/jobs/job-1/deliver',
        (_) => jobJson(status: 'delivered', live: false),
      );
      final Dependencies dependencies = await pumpJob(
        tester,
        status: 'collected',
      );
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.deliver),
      );
      expect(backend.to('/v1/partner/jobs/job-1/deliver'), hasLength(1));
      dependencies.dispose();
    });

    testWidgets('an offer can be taken or passed on from here', (
      WidgetTester tester,
    ) async {
      route(
        'POST /v1/partner/jobs/job-1/accept',
        (_) => jobJson(status: 'assigned'),
      );
      route(
        'POST /v1/partner/jobs/job-1/decline',
        (_) => jobJson(status: 'waiting'),
      );
      final Dependencies dependencies = await pumpJob(
        tester,
        status: 'offered',
      );
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.declineJob),
      );
      expect(backend.to('/v1/partner/jobs/job-1/decline'), hasLength(1));
      dependencies.dispose();
    });

    testWidgets('taking an offer from here moves it to collect', (
      WidgetTester tester,
    ) async {
      route(
        'POST /v1/partner/jobs/job-1/accept',
        (_) => jobJson(status: 'assigned'),
      );
      final Dependencies dependencies = await pumpJob(
        tester,
        status: 'offered',
      );
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.acceptJob),
      );
      await reveal(tester, find.widgetWithText(GoklayButton, bn.collect));
      expect(find.widgetWithText(GoklayButton, bn.collect), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('failing needs a reason, and sends the rider\'s words', (
      WidgetTester tester,
    ) async {
      route(
        'POST /v1/partner/jobs/job-1/fail',
        (_) => jobJson(status: 'failed', live: false, reason: 'দরজা খোলেনি'),
      );
      final Dependencies dependencies = await pumpJob(tester);
      await reveal(tester, find.widgetWithText(GoklayButton, bn.failJob));
      expect(
        tester
            .widget<GoklayButton>(
              find.widgetWithText(GoklayButton, bn.failJob),
            )
            .isEnabled,
        isFalse,
      );
      await enterInto(tester, bn.failReason, 'দরজা খোলেনি');
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.failJob),
      );
      expect(
        (backend.to('/v1/partner/jobs/job-1/fail').single.body!
            as Map<String, Object?>)['reason'],
        'দরজা খোলেনি',
      );
      dependencies.dispose();
    });

    testWidgets('a failed job shows the reason it carries', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await pumpJob(
        tester,
        status: 'failed',
        reason: 'দরজা খোলেনি',
      );
      expect(find.text('দরজা খোলেনি'), findsOneWidget);
      expect(find.text(bn.collect), findsNothing);
      dependencies.dispose();
    });

    testWidgets('a refusal is shown rather than queued', (
      WidgetTester tester,
    ) async {
      route(
        'POST /v1/partner/jobs/job-1/collect',
        (_) => errorBody('bad_state', 'এখনো নেওয়া যাবে না'),
      );
      backend.statuses['POST /v1/partner/jobs/job-1/collect'] = 409;
      final Dependencies dependencies = await pumpJob(tester);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.collect),
      );
      expect(find.text('এখনো নেওয়া যাবে না'), findsOneWidget);
      expect(dependencies.outbox.isEmpty, isTrue);
      dependencies.dispose();
    });

    testWidgets('a tap made with no signal is queued, not lost', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await offlineDependencies();
      await pumpScreen(
        tester,
        JobScreen(job: DeliveryJob.fromJson(jobJson(status: 'assigned'))),
        dependencies: dependencies,
      );
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.collect),
      );
      expect(find.text(core.queuedOffline), findsOneWidget);
      expect(dependencies.outbox.pending, hasLength(1));
      expect(
        dependencies.outbox.pending.single.path,
        '/v1/partner/jobs/job-1/collect',
      );
      dependencies.dispose();
    });

    testWidgets('a queued failure keeps the reason with it', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await offlineDependencies();
      await pumpScreen(
        tester,
        JobScreen(job: DeliveryJob.fromJson(jobJson(status: 'collected'))),
        dependencies: dependencies,
      );
      await enterInto(tester, bn.failReason, 'ঠিকানা পাইনি');
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.failJob),
      );
      expect(
        (dependencies.outbox.pending.single.body! as Map<String, Object?>)[
            'reason'],
        'ঠিকানা পাইনি',
      );
      dependencies.dispose();
    });
  });

  group('the ledger screen', () {
    // Both values of showBack, which is the screen's only parameter and was
    // not asserted anywhere: the shell builds this tab with showBack: false
    // because a tab is not something you go back from, and the pushed screen
    // takes the default.
    //
    // Constructing it from a loop variable rather than as `const` is also
    // what makes the constructor run. Every other call site in the app and
    // the tests is a compile-time constant, so the constructor is
    // canonicalised and never executes — which left its line covered or not
    // depending on which isolate happened to materialise the constant first,
    // and failed the coverage gate on CI while passing on every local run.
    for (final bool showBack in <bool>[true, false]) {
      testWidgets('it prints both totals and what is being carried '
          '(showBack: $showBack)', (WidgetTester tester) async {
        route('GET /v1/partner/cod', (_) => ledgerJson());
        final Dependencies dependencies = await harnessDependencies(backend);
        await pumpScreen(
          tester,
          CashScreen(showBack: showBack),
          dependencies: dependencies,
        );
        await tester.pumpAndSettle();
        expect(find.text('৳ ৭০০'), findsWidgets);
        expect(find.text('৳ ১,২০০'), findsOneWidget);
        expect(find.text('ord-1'), findsOneWidget);
        expect(
          tester.widget<GoklayScaffold>(find.byType(GoklayScaffold)).showBack,
          showBack,
        );
        dependencies.dispose();
      });
    }

    testWidgets('a settled rider is told there is nothing to hand over', (
      WidgetTester tester,
    ) async {
      route('GET /v1/partner/cod', (_) => ledgerJson(settled: true));
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        const CashScreen(),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      expect(find.text(bn.settled), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a failed read offers a retry', (WidgetTester tester) async {
      route('GET /v1/partner/cod', (_) => errorBody('b', 'সমস্যা'));
      backend.statuses['GET /v1/partner/cod'] = 503;
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        const CashScreen(),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(core.retry));
      await tester.pumpAndSettle();
      expect(backend.to('/v1/partner/cod'), hasLength(2));
      dependencies.dispose();
    });
  });

  group('the account', () {
    Future<Dependencies> pumpAccount(
      WidgetTester tester, {
      void Function(Locale)? onLanguage,
      VoidCallback? onSignedOut,
    }) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        PartnerAccountScreen(
          onLanguageChanged: onLanguage ?? (_) {},
          onSignedOut: onSignedOut ?? () {},
        ),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      return dependencies;
    }

    testWidgets('the distance choice is D4\'s three, and posts one', (
      WidgetTester tester,
    ) async {
      route('GET /v1/partner', (_) => partnerJson());
      route(
        'PUT /v1/partner/preference',
        (_) => partnerJson(preference: 'short'),
      );
      final Dependencies dependencies = await pumpAccount(tester);
      expect(find.text('করিম'), findsOneWidget);
      expect(find.textContaining('82'), findsOneWidget);
      await revealAndTap(
        tester,
        find.widgetWithText(SettingRow, bn.preferenceShort),
      );
      expect(
        (backend.to('/v1/partner/preference').single.body!
            as Map<String, Object?>)['preference'],
        'short',
      );
      dependencies.dispose();
    });

    testWidgets('a location is reported only when it is a real point', (
      WidgetTester tester,
    ) async {
      route('GET /v1/partner', (_) => partnerJson());
      route('PUT /v1/partner/location', (_) => partnerJson());
      final Dependencies dependencies = await pumpAccount(tester);
      await enterInto(tester, 'lat', 'not a number');
      await reveal(
        tester,
        find.widgetWithText(GoklayButton, bn.updateLocation),
      );
      expect(
        tester
            .widget<GoklayButton>(
              find.widgetWithText(GoklayButton, bn.updateLocation),
            )
            .isEnabled,
        isFalse,
      );
      await enterInto(tester, 'lat', '23.80');
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.updateLocation),
      );
      expect(
        (backend.to('/v1/partner/location').single.body!
            as Map<String, Object?>)['lat'],
        23.80,
      );
      dependencies.dispose();
    });

    testWidgets('switching language changes the requests too', (
      WidgetTester tester,
    ) async {
      route('GET /v1/partner', (_) => partnerJson());
      route('PATCH /v1/me', (_) => profileJson());
      Locale? chosen;
      final Dependencies dependencies = await pumpAccount(
        tester,
        onLanguage: (Locale locale) => chosen = locale,
      );
      await revealAndTap(tester, find.widgetWithText(SettingRow, 'English'));
      expect(chosen, const Locale('en'));
      expect(dependencies.locale, const Locale('en'));
      expect(backend.to('/v1/me').last.url.queryParameters['lang'], 'en');
      await revealAndTap(tester, find.widgetWithText(SettingRow, 'বাংলা'));
      expect(dependencies.locale, const Locale('bn'));
      dependencies.dispose();
    });

    testWidgets('signing out clears the tokens even when the call fails', (
      WidgetTester tester,
    ) async {
      route('GET /v1/partner', (_) => partnerJson());
      route('POST /v1/auth/logout', (_) => errorBody('b', 'সমস্যা'));
      backend.statuses['POST /v1/auth/logout'] = 503;
      int signedOut = 0;
      final Dependencies dependencies = await pumpAccount(
        tester,
        onSignedOut: () => signedOut += 1,
      );
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.signOut),
      );
      expect(signedOut, 1);
      expect(dependencies.session.isSignedIn, isFalse);
      dependencies.dispose();
    });

    testWidgets('a refused preference is shown', (WidgetTester tester) async {
      route('GET /v1/partner', (_) => partnerJson());
      route(
        'PUT /v1/partner/preference',
        (_) => errorBody('bad_preference', 'এটি চলবে না'),
      );
      backend.statuses['PUT /v1/partner/preference'] = 400;
      final Dependencies dependencies = await pumpAccount(tester);
      await revealAndTap(
        tester,
        find.widgetWithText(SettingRow, bn.preferenceLong),
      );
      expect(find.text('এটি চলবে না'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a failed read offers a retry', (WidgetTester tester) async {
      route('GET /v1/partner', (_) => errorBody('b', 'সমস্যা'));
      backend.statuses['GET /v1/partner'] = 503;
      final Dependencies dependencies = await pumpAccount(tester);
      await tester.tap(find.bySemanticsLabel(core.retry));
      await tester.pumpAndSettle();
      expect(backend.to('/v1/partner'), hasLength(2));
      dependencies.dispose();
    });
  });
}
