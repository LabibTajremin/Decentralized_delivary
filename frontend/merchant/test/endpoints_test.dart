import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_merchant/src/api/models/catalogue.dart';
import 'package:goklay_merchant/src/api/models/merchant.dart';
import 'package:goklay_merchant/src/dependencies.dart';

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

  group('the shop', () {
    test('the registration requirements come from the server', () async {
      route(
        'GET /v1/merchants/registration-requirements',
        (_) => requirementsJson(),
      );
      final ApiPage<RegistrationRequirements> page = await dependencies
          .merchant
          .requirements();
      expect(page.value.types, contains('pharmacy'));
    });

    test('registering sends no area, district or division', () async {
      route('POST /v1/merchants', (_) => merchantJson(status: 'draft'));
      await dependencies.merchant.register(
        name: 'নূরজাহান হোটেল',
        type: 'restaurant',
        phone: '01712345678',
        line1: '১২/এ',
        lat: 23.7509,
        lng: 90.3925,
        email: 'shop@example.com',
      );
      final Map<String, Object?> body = lastBody('/v1/merchants');
      expect(body['type'], 'restaurant');
      expect(body['lat'], 23.7509);
      expect(body.containsKey('area_code'), isFalse);
      expect(body.containsKey('division_code'), isFalse);
    });

    test('the owner reads and patches their own shop', () async {
      route('GET /v1/merchants/me', (_) => merchantJson());
      route('PATCH /v1/merchants/me', (_) => merchantJson());
      expect((await dependencies.merchant.me()).value.id, 'mch-1');
      await dependencies.merchant.updateDetails(
        name: 'নূরজাহান',
        type: 'restaurant',
        phone: '017',
        line1: 'x',
        lat: 1,
        lng: 2,
      );
      expect(lastBody('/v1/merchants/me')['name'], 'নূরজাহান');
    });

    test('a document is added, then the shop is submitted', () async {
      route(
        'POST /v1/merchants/me/documents',
        (_) => merchantJson(canSubmit: true),
      );
      route(
        'POST /v1/merchants/me/submit',
        (_) => merchantJson(status: 'pending_review'),
      );
      final Merchant withDocument = await dependencies.merchant.addDocument(
        kind: 'food_licence',
        number: 'FL-1',
        fileUrl: '/static/fl.pdf',
      );
      expect(withDocument.canSubmit, isTrue);
      expect(lastBody('/v1/merchants/me/documents')['kind'], 'food_licence');
      final Merchant submitted = await dependencies.merchant
          .submitForReview();
      expect(submitted.isAwaitingReview, isTrue);
    });

    test('the hours go exactly as typed', () async {
      route('PUT /v1/merchants/me/hours', (_) => merchantJson());
      await dependencies.merchant.setHours(<String, List<String>>{
        '0': <String>['22:00-24:00'],
        '1': <String>['00:00-02:00'],
      });
      expect(lastBody('/v1/merchants/me/hours')['days'], <String, Object?>{
        '0': <String>['22:00-24:00'],
        '1': <String>['00:00-02:00'],
      });
    });

    test('a holiday is set, and cleared with an empty body', () async {
      route('PUT /v1/merchants/me/holiday', (_) => merchantJson());
      await dependencies.merchant.setHoliday(
        until: DateTime.utc(2026, 9, 25),
        reason: 'ঈদের ছুটি',
      );
      final Map<String, Object?> set = lastBody('/v1/merchants/me/holiday');
      expect(set['reason'], 'ঈদের ছুটি');
      expect(set['until'], startsWith('2026-09-25'));

      await dependencies.merchant.setHoliday(clear: true);
      expect(lastBody('/v1/merchants/me/holiday'), isEmpty);
    });

    test('an indefinite holiday leaves the date off', () async {
      route('PUT /v1/merchants/me/holiday', (_) => merchantJson());
      await dependencies.merchant.setHoliday(reason: 'বন্ধ');
      final Map<String, Object?> body = lastBody('/v1/merchants/me/holiday');
      expect(body.containsKey('until'), isFalse);
      expect(body['reason'], 'বন্ধ');
    });
  });

  group('the catalogue', () {
    test('capabilities, sections, items and bundles all read', () async {
      route(
        'GET /v1/merchants/mch-1/catalogue/capabilities',
        (_) => capabilitiesJson(),
      );
      route(
        'GET /v1/merchants/mch-1/catalogue/categories',
        (_) => <String, Object?>{
          'categories': <Object?>[categoryJson(), 7],
        },
      );
      route(
        'GET /v1/merchants/mch-1/catalogue/items',
        (_) => <String, Object?>{'items': <Object?>[itemJson()]},
      );
      route(
        'GET /v1/merchants/mch-1/catalogue/combos',
        (_) => <String, Object?>{'combos': <Object?>[comboJson()]},
      );
      final ApiPage<CatalogueCapabilities> capabilities = await dependencies
          .catalogue
          .capabilities('mch-1');
      expect(capabilities.value.combos, isTrue);
      expect(
        (await dependencies.catalogue.categories('mch-1')).value,
        hasLength(1),
      );
      expect((await dependencies.catalogue.items('mch-1')).value, hasLength(1));
      expect(
        (await dependencies.catalogue.combos('mch-1')).value,
        hasLength(1),
      );
    });

    test('a missing array is an empty list rather than a throw', () async {
      route(
        'GET /v1/merchants/mch-1/catalogue/categories',
        (_) => <String, Object?>{},
      );
      expect(
        (await dependencies.catalogue.categories('mch-1')).value,
        isEmpty,
      );
    });

    test('a section is added and hidden', () async {
      route(
        'POST /v1/merchants/mch-1/catalogue/categories',
        (_) => categoryJson(),
      );
      route(
        'PUT /v1/merchants/mch-1/catalogue/categories/cat-1/active',
        (_) => categoryJson(active: false),
      );
      await dependencies.catalogue.addCategory(
        merchantId: 'mch-1',
        name: 'বিরিয়ানি',
      );
      expect(
        lastBody('/v1/merchants/mch-1/catalogue/categories')['name'],
        'বিরিয়ানি',
      );
      final OwnerCategory hidden = await dependencies.catalogue
          .setCategoryActive(
            merchantId: 'mch-1',
            categoryId: 'cat-1',
            active: false,
          );
      expect(hidden.active, isFalse);
    });

    test('an item is added with a minor-unit price and nothing else', () async {
      route('POST /v1/merchants/mch-1/catalogue/items', (_) => itemJson());
      await dependencies.catalogue.addItem(
        merchantId: 'mch-1',
        categoryId: 'cat-1',
        name: 'কাচ্চি',
        priceMinor: 32000,
        description: 'বাসমতি',
      );
      final Map<String, Object?> body = lastBody(
        '/v1/merchants/mch-1/catalogue/items',
      );
      expect(body['price_minor'], 32000);
      expect(body.containsKey('price'), isFalse);
      expect(body.containsKey('unit'), isFalse);
      expect(body.containsKey('requires_prescription'), isFalse);
    });

    test('the per-type fields are sent only when they were given', () async {
      route('POST /v1/merchants/mch-1/catalogue/items', (_) => itemJson());
      await dependencies.catalogue.addItem(
        merchantId: 'mch-1',
        categoryId: 'cat-1',
        name: 'নাপা',
        priceMinor: 1200,
        unit: 'strip',
        packSize: '10 pcs',
        brand: 'Beximco',
        requiresPrescription: true,
      );
      final Map<String, Object?> body = lastBody(
        '/v1/merchants/mch-1/catalogue/items',
      );
      expect(body['unit'], 'strip');
      expect(body['pack_size'], '10 pcs');
      expect(body['brand'], 'Beximco');
      expect(body['requires_prescription'], isTrue);
    });

    test('an item is switched off and its shelf set', () async {
      route(
        'PUT /v1/merchants/mch-1/catalogue/items/itm-1/active',
        (_) => itemJson(active: false, orderable: false),
      );
      route(
        'PUT /v1/merchants/mch-1/catalogue/items/itm-1/stock',
        (_) => itemJson(stockTracked: true, stock: 5),
      );
      expect(
        (await dependencies.catalogue.setItemActive(
          merchantId: 'mch-1',
          itemId: 'itm-1',
          active: false,
        )).active,
        isFalse,
      );
      final OwnerItem stocked = await dependencies.catalogue.setStock(
        merchantId: 'mch-1',
        itemId: 'itm-1',
        quantity: 5,
      );
      expect(stocked.stockQuantity, 5);
      expect(
        lastBody('/v1/merchants/mch-1/catalogue/items/itm-1/stock')['quantity'],
        5,
      );
    });

    test('a bundle is switched off', () async {
      route(
        'PUT /v1/merchants/mch-1/catalogue/combos/cmb-1/active',
        (_) => comboJson(active: false),
      );
      expect(
        (await dependencies.catalogue.setComboActive(
          merchantId: 'mch-1',
          comboId: 'cmb-1',
          active: false,
        )).active,
        isFalse,
      );
    });
  });

  group('the order board', () {
    test('the live board is a server filter', () async {
      route('GET /v1/merchants/mch-1/orders', (_) => orderListJson());
      await dependencies.orders.list(merchantId: 'mch-1');
      expect(
        backend.to('/v1/merchants/mch-1/orders').last.url
            .queryParameters['live'],
        'true',
      );
      await dependencies.orders.list(merchantId: 'mch-1', live: false);
      expect(
        backend.to('/v1/merchants/mch-1/orders').last.url.queryParameters
            .containsKey('live'),
        isFalse,
      );
    });

    test('one order reads from its own path', () async {
      route(
        'GET /v1/merchants/mch-1/orders/ord-1',
        (_) => orderJson(),
      );
      expect(
        (await dependencies.orders.order(
          merchantId: 'mch-1',
          orderId: 'ord-1',
        )).value.code,
        'GK-7F3K',
      );
    });

    test('all four transitions post to their own path', () async {
      for (final String action in <String>[
        'accept',
        'preparing',
        'ready',
        'reject',
      ]) {
        route(
          'POST /v1/merchants/mch-1/orders/ord-1/$action',
          (_) => orderJson(status: action),
        );
      }
      await dependencies.orders.accept(merchantId: 'mch-1', orderId: 'ord-1');
      await dependencies.orders.preparing(
        merchantId: 'mch-1',
        orderId: 'ord-1',
      );
      await dependencies.orders.ready(merchantId: 'mch-1', orderId: 'ord-1');
      await dependencies.orders.reject(
        merchantId: 'mch-1',
        orderId: 'ord-1',
        reason: 'উপকরণ নেই',
      );
      expect(
        backend.to('/v1/merchants/mch-1/orders/ord-1/accept'),
        hasLength(1),
      );
      expect(
        lastBody('/v1/merchants/mch-1/orders/ord-1/reject')['reason'],
        'উপকরণ নেই',
      );
      expect(
        backend.to('/v1/merchants/mch-1/orders/ord-1/accept').single.body,
        isNull,
      );
    });

    test('a refused transition arrives as the server\'s own sentence', () async {
      route(
        'POST /v1/merchants/mch-1/orders/ord-1/accept',
        (_) => errorBody('already_cancelled', 'অর্ডারটি বাতিল হয়ে গেছে'),
      );
      backend.statuses['POST /v1/merchants/mch-1/orders/ord-1/accept'] = 409;
      await expectLater(
        dependencies.orders.accept(merchantId: 'mch-1', orderId: 'ord-1'),
        throwsA(
          isA<ApiError>().having(
            (ApiError e) => e.message,
            'message',
            'অর্ডারটি বাতিল হয়ে গেছে',
          ),
        ),
      );
    });
  });

  test('signing out clears the tokens and the cached board', () async {
    route('GET /v1/merchants/me', (_) => merchantJson());
    await dependencies.merchant.me();
    await dependencies.signOut();
    expect(dependencies.session.isSignedIn, isFalse);
    expect(await dependencies.api.cached('/v1/merchants/me'), isNull);
  });
}
