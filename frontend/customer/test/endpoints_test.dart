import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_customer/src/api/endpoints/cart_api.dart';
import 'package:goklay_customer/src/api/endpoints/order_api.dart';
import 'package:goklay_customer/src/api/models/cart.dart';
import 'package:goklay_customer/src/api/models/notification.dart';
import 'package:goklay_customer/src/api/models/order.dart';
import 'package:goklay_customer/src/api/models/payment.dart';
import 'package:goklay_customer/src/api/models/support.dart';
import 'package:goklay_customer/src/dependencies.dart';

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

  group('discovery', () {
    test('a search sends the level it was told, never one it invented', () async {
      route('GET /v1/discovery/merchants', (_) => searchJson());
      await dependencies.discovery.search(
        lat: 23.7,
        lng: 90.3,
        level: 2,
        type: 'grocery',
        query: 'chaal',
      );
      final Uri url = backend.to('/v1/discovery/merchants').last.url;
      expect(url.queryParameters['level'], '2');
      expect(url.queryParameters['type'], 'grocery');
      expect(url.queryParameters['q'], 'chaal');
    });

    test('an empty type or query is left off the request entirely', () async {
      route('GET /v1/discovery/merchants', (_) => searchJson());
      await dependencies.discovery.search(lat: 23.7, lng: 90.3, type: '', query: '');
      final Uri url = backend.to('/v1/discovery/merchants').last.url;
      expect(url.queryParameters.containsKey('type'), isFalse);
      expect(url.queryParameters.containsKey('q'), isFalse);
    });

    test('resolving a coordinate names its division', () async {
      route('GET /v1/geo/resolve', (_) => areaJson());
      final ApiPage<Area> page = await dependencies.discovery.resolve(
        lat: 23.7461,
        lng: 90.3742,
      );
      expect(page.value.divisionName, 'ঢাকা');
    });
  });

  group('catalogue', () {
    test('a menu, an item and a combo each read from their own path', () async {
      route('GET /v1/catalogue/mer-1/menu', (_) => menuJson());
      route('GET /v1/catalogue/mer-1/items/itm-1', (_) => itemJson());
      route('GET /v1/catalogue/mer-1/combos/cmb-1', (_) => comboJson());
      expect((await dependencies.catalogue.menu('mer-1')).value.items,
          hasLength(1));
      expect(
        (await dependencies.catalogue.item(
          merchantId: 'mer-1',
          itemId: 'itm-1',
        )).value.name,
        'কাচ্চি',
      );
      expect(
        (await dependencies.catalogue.combo(
          merchantId: 'mer-1',
          comboId: 'cmb-1',
        )).value.lines,
        hasLength(1),
      );
    });
  });

  group('cart', () {
    test('choices go as ids, and never carry a price', () async {
      route('POST /v1/cart/items', (_) => cartJson());
      await dependencies.cart.add(
        merchantId: 'mer-1',
        kind: 'item',
        targetId: 'itm-1',
        quantity: 2,
        choices: const <CartChoice>[
          CartChoice(groupId: 'grp-1', optionId: 'grp-1-b'),
        ],
        note: 'ঝাল কম',
      );
      final Map<String, Object?> body = lastBody('/v1/cart/items');
      expect(body['quantity'], 2);
      expect(body['choices'], <Object?>[
        <String, Object?>{'group_id': 'grp-1', 'option_id': 'grp-1-b'},
      ]);
      expect(body.toString(), isNot(contains('price')));
    });

    test('every write answers with the whole revalidated cart', () async {
      route('GET /v1/cart', (_) => cartJson());
      route('PUT /v1/cart/lines/lin-1', (_) => cartJson(quantity: 3));
      route('DELETE /v1/cart/lines/lin-1', (_) => cartJson(empty: true));
      route('PUT /v1/cart/address', (_) => cartJson(addressId: 'adr-1'));
      route('DELETE /v1/cart', (_) => null);

      expect((await dependencies.cart.cart()).value.count, 1);
      final Cart raised = await dependencies.cart.setQuantity(
        lineId: 'lin-1',
        quantity: 3,
      );
      expect(raised.lines.single.quantity, 3);
      expect(lastBody('/v1/cart/lines/lin-1')['quantity'], 3);
      expect((await dependencies.cart.removeLine('lin-1')).isEmpty, isTrue);
      final Cart bound = await dependencies.cart.setAddress(
        addressId: 'adr-1',
        lat: 23.7,
        lng: 90.3,
      );
      expect(bound.addressId, 'adr-1');
      await dependencies.cart.clear();
      expect(backend.to('/v1/cart').last.method, 'DELETE');
    });
  });

  group('orders', () {
    test('placing sends the idempotency key that stops a double tap', () async {
      route('POST /v1/orders', (_) => orderJson());
      final Order order = await dependencies.orders.place(
        addressId: 'adr-1',
        paymentMethod: 'cash',
        idempotencyKey: 'crt-1-42',
      );
      expect(order.code, 'GK-7F3K');
      expect(lastBody('/v1/orders')['idempotency_key'], 'crt-1-42');
    });

    test('the live tab is a server filter, not a local one', () async {
      route('GET /v1/orders', (_) => <String, Object?>{
        'orders': <Object?>[orderJson(), 'not an order'],
        'total': 1,
      });
      final ApiPage<OrderList> page = await dependencies.orders.list(live: true);
      expect(page.value.orders, hasLength(1));
      expect(page.value.isEmpty, isFalse);
      expect(
        backend.to('/v1/orders').last.url.queryParameters['live'],
        'true',
      );
    });

    test('the past tab asks for no live filter at all', () async {
      route('GET /v1/orders', (_) => <String, Object?>{'orders': <Object?>[], 'total': 0});
      final ApiPage<OrderList> page = await dependencies.orders.list();
      expect(page.value.isEmpty, isTrue);
      expect(
        backend.to('/v1/orders').last.url.queryParameters.containsKey('live'),
        isFalse,
      );
    });

    test('one order, its cancellation window and cancelling it', () async {
      route('GET /v1/orders/ord-1', (_) => orderJson());
      route('GET /v1/orders/ord-1/cancellation', (_) => cancellationJson());
      route('POST /v1/orders/ord-1/cancel',
          (_) => orderJson(status: 'cancelled', live: false));
      expect((await dependencies.orders.order('ord-1')).value.id, 'ord-1');
      expect(
        (await dependencies.orders.cancellation('ord-1')).value.secondsLeft,
        240,
      );
      final Order cancelled = await dependencies.orders.cancel(
        orderId: 'ord-1',
        reason: 'ভুল ঠিকানা',
      );
      expect(cancelled.live, isFalse);
      expect(lastBody('/v1/orders/ord-1/cancel')['reason'], 'ভুল ঠিকানা');
    });
  });

  group('payments', () {
    test('checkout resumes rather than starting a second attempt', () async {
      route('POST /v1/payments/checkout', (_) => checkoutJson());
      route('GET /v1/payments/ord-1', (_) => paymentJson());
      final Checkout checkout = await dependencies.payments.checkout('ord-1');
      expect(checkout.paymentId, 'pay-1');
      expect(lastBody('/v1/payments/checkout')['order_id'], 'ord-1');
      expect((await dependencies.payments.payment('ord-1')).value.isCaptured,
          isTrue);
    });
  });

  group('reviews and support', () {
    test('a review names its subject by the wire value', () async {
      route('POST /v1/reviews', (_) => reviewJson());
      await dependencies.support.submitReview(
        orderId: 'ord-1',
        subject: ReviewSubject.partner,
        subjectId: 'ptn-1',
        rating: 4,
        comment: 'দ্রুত',
      );
      final Map<String, Object?> body = lastBody('/v1/reviews');
      expect(body['subject'], 'partner');
      expect(body['rating'], 4);
    });

    test('reviews and ratings read their own endpoints', () async {
      route('GET /v1/reviews', (_) => <String, Object?>{
        'reviews': <Object?>[reviewJson(), 7],
      });
      route('GET /v1/ratings', (_) => ratingJson());
      final ApiPage<List<Review>> reviews = await dependencies.support.reviews(
        subject: ReviewSubject.merchant,
        subjectId: 'mer-1',
      );
      expect(reviews.value, hasLength(1));
      expect(
        backend.to('/v1/reviews').last.url.queryParameters['subject_id'],
        'mer-1',
      );
      expect(
        (await dependencies.support.rating(
          subject: ReviewSubject.merchant,
          subjectId: 'mer-1',
        )).value.count,
        4,
      );
    });

    test('a missing reviews array is an empty list', () async {
      route('GET /v1/reviews', (_) => <String, Object?>{});
      expect(
        (await dependencies.support.reviews(
          subject: ReviewSubject.merchant,
          subjectId: 'mer-1',
        )).value,
        isEmpty,
      );
    });

    test('a ticket is raised against an order and listed back', () async {
      route('POST /v1/support/tickets', (_) => ticketJson());
      route('GET /v1/me/support/tickets', (_) => <String, Object?>{
        'tickets': <Object?>[ticketJson()],
      });
      final SupportTicket ticket = await dependencies.support.raiseTicket(
        orderId: 'ord-1',
        subject: 'খাবার ঠান্ডা ছিল',
      );
      expect(ticket.isOpen, isTrue);
      expect(lastBody('/v1/support/tickets')['order_id'], 'ord-1');
      expect((await dependencies.support.tickets()).value, hasLength(1));
    });
  });

  group('notifications', () {
    test('a bare array is read without an envelope', () async {
      route('GET /v1/me/notifications',
          (_) => <Object?>[notificationJson(), 'not an object']);
      final ApiPage<List<AppNotification>> page =
          await dependencies.notifications.list();
      expect(page.value, hasLength(1));
    });

    test('a body that is not an array at all is an empty list', () async {
      route('GET /v1/me/notifications', (_) => <String, Object?>{});
      expect((await dependencies.notifications.list()).value, isEmpty);
    });

    test('a device registers with its platform and token', () async {
      route('POST /v1/me/device', (_) => null);
      await dependencies.notifications.registerDevice(
        platform: 'android',
        token: 'fcm-1',
      );
      expect(lastBody('/v1/me/device')['platform'], 'android');
    });
  });

}
