import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_partner/src/api/models/job.dart';
import 'package:goklay_partner/src/api/models/ledger.dart';
import 'package:goklay_partner/src/api/models/partner.dart';
import 'package:goklay_partner/src/dependencies.dart';

import 'support/fixtures.dart';
import 'support/harness.dart';

void main() {
  late FakeBackend backend;
  late Dependencies dependencies;

  Map<String, Object?> lastBody(String path) =>
      backend.to(path).last.body! as Map<String, Object?>;

  setUp(() async {
    backend = FakeBackend(<String, Object? Function(SentRequest)>{});
    dependencies = await harnessDependencies(backend);
  });

  tearDown(() => dependencies.dispose());

  void route(String key, Object? Function(SentRequest request) handler) =>
      backend.routes[key] = handler;

  test('signing up sends no area: a rider works anywhere (D1)', () async {
    route('POST /v1/partner', (_) => partnerJson());
    await dependencies.partner.register(
      name: 'করিম',
      phone: '01811111111',
      vehicle: 'bike',
    );
    final Map<String, Object?> body = lastBody('/v1/partner');
    expect(body['name'], 'করিম');
    expect(body['vehicle'], 'bike');
    expect(body.containsKey('area_code'), isFalse);
    expect(body.containsKey('lat'), isFalse);
  });

  test('a rider with no vehicle sends none rather than an empty one', () async {
    route('POST /v1/partner', (_) => partnerJson());
    await dependencies.partner.register(name: 'করিম', phone: '018');
    expect(lastBody('/v1/partner').containsKey('vehicle'), isFalse);
  });

  test('the shift is set to what the app was told to set', () async {
    route('PUT /v1/partner/availability', (_) => partnerJson());
    await dependencies.partner.setAvailability(DeliveryPartner.available);
    expect(
      lastBody('/v1/partner/availability')['availability'],
      'available',
    );
    await dependencies.partner.setAvailability(DeliveryPartner.offline);
    expect(lastBody('/v1/partner/availability')['availability'], 'offline');
  });

  test('the distance choice and the location each post their own', () async {
    route('PUT /v1/partner/preference', (_) => partnerJson());
    route('PUT /v1/partner/location', (_) => partnerJson());
    await dependencies.partner.setPreference('short');
    expect(lastBody('/v1/partner/preference')['preference'], 'short');
    await dependencies.partner.reportLocation(lat: 23.75, lng: 90.38);
    expect(lastBody('/v1/partner/location')['lat'], 23.75);
  });

  test('the record, the feed and the jobs all read', () async {
    route('GET /v1/partner', (_) => partnerJson());
    route('GET /v1/partner/feed', (_) => feedJson());
    route('GET /v1/partner/jobs', (_) => jobListJson());
    expect((await dependencies.partner.me()).value.name, 'করিম');
    final ApiPage<PartnerFeed> feed = await dependencies.partner.feed();
    expect(feed.value.jobs, hasLength(1));
    await dependencies.partner.jobs();
    expect(
      backend.to('/v1/partner/jobs').last.url.queryParameters['live'],
      'true',
    );
    await dependencies.partner.jobs(live: false);
    expect(
      backend.to('/v1/partner/jobs').last.url.queryParameters
          .containsKey('live'),
      isFalse,
    );
  });

  test('all five transitions post to their own path', () async {
    for (final String action in <String>[
      'accept',
      'decline',
      'collect',
      'deliver',
      'fail',
    ]) {
      route('POST /v1/partner/jobs/job-1/$action', (_) => jobJson());
    }
    await dependencies.partner.accept('job-1');
    await dependencies.partner.decline('job-1');
    await dependencies.partner.collect('job-1');
    await dependencies.partner.deliver('job-1');
    final DeliveryJob failed = await dependencies.partner.fail(
      jobId: 'job-1',
      reason: 'দরজা খোলেনি',
    );
    expect(failed.id, 'job-1');
    expect(backend.to('/v1/partner/jobs/job-1/accept').single.body, isNull);
    expect(
      lastBody('/v1/partner/jobs/job-1/fail')['reason'],
      'দরজা খোলেনি',
    );
  });

  test('a second rider taking the same job is refused', () async {
    route(
      'POST /v1/partner/jobs/job-1/accept',
      (_) => errorBody('already_taken', 'অন্য একজন নিয়ে নিয়েছেন'),
    );
    backend.statuses['POST /v1/partner/jobs/job-1/accept'] = 409;
    await expectLater(
      dependencies.partner.accept('job-1'),
      throwsA(
        isA<ApiError>()
            .having((ApiError e) => e.statusCode, 'status', 409)
            .having(
              (ApiError e) => e.message,
              'message',
              'অন্য একজন নিয়ে নিয়েছেন',
            ),
      ),
    );
  });

  test('the ledger reads both totals and the held collections', () async {
    route('GET /v1/partner/cod', (_) => ledgerJson());
    final ApiPage<Ledger> ledger = await dependencies.partner.ledger();
    expect(ledger.value.outstanding.display, '৳ ৭০০');
    expect(ledger.value.held, hasLength(1));
  });

  group('the outbox', () {
    test('a queued tap is replayed oldest first', () async {
      route('POST /v1/partner/jobs/job-1/collect', (_) => jobJson());
      route('POST /v1/partner/jobs/job-1/deliver', (_) => jobJson());
      dependencies.outbox
        ..enqueue(
          const QueuedAction(
            id: 'a',
            method: 'POST',
            path: '/v1/partner/jobs/job-1/collect',
          ),
        )
        ..enqueue(
          const QueuedAction(
            id: 'b',
            method: 'POST',
            path: '/v1/partner/jobs/job-1/deliver',
          ),
        );
      final FlushReport report = await dependencies.flushOutbox();
      expect(report.sent.map((QueuedAction a) => a.id), <String>['a', 'b']);
      expect(report.rejected, isEmpty);
      expect(dependencies.outbox.isEmpty, isTrue);
      expect(backend.sent.first.url.path, '/v1/partner/jobs/job-1/collect');
    });

    test('an action the server refuses is dropped and reported', () async {
      route(
        'POST /v1/partner/jobs/job-1/deliver',
        (_) => errorBody('bad_state', 'এখনো নেওয়া হয়নি'),
      );
      backend.statuses['POST /v1/partner/jobs/job-1/deliver'] = 409;
      dependencies.outbox.enqueue(
        const QueuedAction(
          id: 'b',
          method: 'POST',
          path: '/v1/partner/jobs/job-1/deliver',
        ),
      );
      final FlushReport report = await dependencies.flushOutbox();
      expect(report.rejected, hasLength(1));
      expect(dependencies.outbox.isEmpty, isTrue);
    });

    test('signing out clears the outbox as well as the tokens', () async {
      route('GET /v1/partner', (_) => partnerJson());
      await dependencies.partner.me();
      dependencies.outbox.enqueue(
        const QueuedAction(
          id: 'a',
          method: 'POST',
          path: '/v1/partner/jobs/job-1/collect',
        ),
      );
      await dependencies.signOut();
      expect(dependencies.outbox.isEmpty, isTrue);
      expect(dependencies.session.isSignedIn, isFalse);
      expect(await dependencies.api.cached('/v1/partner'), isNull);
    });
  });
}
