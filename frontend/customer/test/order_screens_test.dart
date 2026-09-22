import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_customer/src/api/models/order.dart';
import 'package:goklay_customer/src/dependencies.dart';
import 'package:goklay_customer/src/l10n/customer_strings.dart';
import 'package:goklay_customer/src/screens/order_screen.dart';
import 'package:goklay_customer/src/screens/orders_screen.dart';
import 'package:goklay_customer/src/screens/payment_screen.dart';
import 'package:goklay_customer/src/widgets/cards.dart';

import 'support/fixtures.dart';
import 'support/harness.dart';

void main() {
  const CustomerStringsBn bn = CustomerStringsBn();
  const GoklayStringsBn core = GoklayStringsBn();
  late FakeBackend backend;

  setUp(() => backend = FakeBackend(<String, Object? Function(SentRequest)>{}));

  void route(String key, Object? Function(SentRequest) handler) =>
      backend.routes[key] = handler;

  group('orders list', () {
    Future<Dependencies> pumpOrders(
      WidgetTester tester, {
      void Function(Order)? onSelected,
    }) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        OrdersScreen(onOrderSelected: onSelected ?? (_) {}),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      return dependencies;
    }

    testWidgets('opens on the live tab, which is a server filter', (
      WidgetTester tester,
    ) async {
      route('GET /v1/orders', (_) => <String, Object?>{
        'orders': <Object?>[orderJson()],
        'total': 1,
      });
      Order? opened;
      final Dependencies dependencies = await pumpOrders(
        tester,
        onSelected: (Order order) => opened = order,
      );
      expect(
        backend.to('/v1/orders').single.url.queryParameters['live'],
        'true',
      );
      await tester.tap(find.byType(OrderTile));
      await tester.pumpAndSettle();
      expect(opened!.id, 'ord-1');
      dependencies.dispose();
    });

    testWidgets('the past tab drops the filter and re-reads', (
      WidgetTester tester,
    ) async {
      route('GET /v1/orders', (_) => <String, Object?>{
        'orders': <Object?>[],
        'total': 0,
      });
      final Dependencies dependencies = await pumpOrders(tester);
      await tester.tap(find.text(bn.ordersPast));
      await tester.pumpAndSettle();
      expect(
        backend.to('/v1/orders').last.url.queryParameters.containsKey('live'),
        isFalse,
      );
      expect(find.text(bn.noOrders), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a failed read offers a retry', (WidgetTester tester) async {
      route('GET /v1/orders', (_) => errorBody('boom', 'সমস্যা'));
      backend.statuses['GET /v1/orders'] = 503;
      final Dependencies dependencies = await pumpOrders(tester);
      await tester.tap(find.bySemanticsLabel(core.retry));
      await tester.pumpAndSettle();
      expect(backend.to('/v1/orders'), hasLength(2));
      dependencies.dispose();
    });
  });

  group('one order', () {
    Future<Dependencies> pumpOrder(
      WidgetTester tester, {
      void Function(Order)? onTrack,
      void Function(Order)? onPay,
      void Function(Order)? onReview,
      void Function(Order)? onSupport,
    }) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        OrderScreen(
          orderId: 'ord-1',
          onTrack: onTrack ?? (_) {},
          onPay: onPay ?? (_) {},
          onReview: onReview ?? (_) {},
          onSupport: onSupport ?? (_) {},
        ),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      return dependencies;
    }

    testWidgets('shows the timeline, the address and the frozen receipt', (
      WidgetTester tester,
    ) async {
      route('GET /v1/orders/ord-1', (_) => orderJson());
      route('GET /v1/orders/ord-1/cancellation', (_) => cancellationJson());
      final Dependencies dependencies = await pumpOrder(tester);
      expect(find.text('GK-7F3K'), findsOneWidget);
      expect(find.text('অর্ডার হয়েছে'), findsOneWidget);
      expect(find.text('রোড ৫, ধানমন্ডি, ঢাকা'), findsOneWidget);
      expect(find.text('1 × কাচ্চি'), findsOneWidget);
      expect(find.text('সাবটোটাল'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a live order offers tracking and help, not a review', (
      WidgetTester tester,
    ) async {
      route('GET /v1/orders/ord-1', (_) => orderJson());
      route('GET /v1/orders/ord-1/cancellation', (_) => cancellationJson());
      int tracked = 0;
      int helped = 0;
      final Dependencies dependencies = await pumpOrder(
        tester,
        onTrack: (_) => tracked += 1,
        onSupport: (_) => helped += 1,
      );
      expect(find.text(bn.leaveReview), findsNothing);
      await tester.tap(find.bySemanticsLabel(bn.trackOrder));
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(bn.getHelp));
      await tester.pumpAndSettle();
      expect(tracked, 1);
      expect(helped, 1);
      dependencies.dispose();
    });

    testWidgets('a finished order offers a review instead', (
      WidgetTester tester,
    ) async {
      route('GET /v1/orders/ord-1',
          (_) => orderJson(status: 'delivered', live: false));
      route('GET /v1/orders/ord-1/cancellation',
          (_) => cancellationJson(allowed: false));
      int reviewed = 0;
      final Dependencies dependencies = await pumpOrder(
        tester,
        onReview: (_) => reviewed += 1,
      );
      expect(find.text(bn.trackOrder), findsNothing);
      await tester.tap(find.bySemanticsLabel(bn.leaveReview));
      await tester.pumpAndSettle();
      expect(reviewed, 1);
      dependencies.dispose();
    });

    testWidgets('an unpaid order offers payment', (WidgetTester tester) async {
      route('GET /v1/orders/ord-1',
          (_) => orderJson(status: 'pending_payment', payment: 'online'));
      route('GET /v1/orders/ord-1/cancellation', (_) => cancellationJson());
      int paid = 0;
      final Dependencies dependencies = await pumpOrder(
        tester,
        onPay: (_) => paid += 1,
      );
      await tester.tap(find.bySemanticsLabel(bn.payNow));
      await tester.pumpAndSettle();
      expect(paid, 1);
      dependencies.dispose();
    });

    testWidgets('cancelling repaints the order the server answered with', (
      WidgetTester tester,
    ) async {
      route('GET /v1/orders/ord-1', (_) => orderJson());
      route('GET /v1/orders/ord-1/cancellation', (SentRequest _) =>
          backend.to('/v1/orders/ord-1/cancel').isEmpty
              ? cancellationJson()
              : cancellationJson(allowed: false));
      route('POST /v1/orders/ord-1/cancel',
          (_) => orderJson(status: 'cancelled', live: false));
      final Dependencies dependencies = await pumpOrder(tester);
      expect(find.text('আর ৪ মিনিট বাতিল করা যাবে'), findsOneWidget);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.cancelOrder),
      );
      expect(find.text('আর বাতিল করা যাবে না'), findsOneWidget);
      expect(find.text(bn.cancelOrder), findsNothing);
      dependencies.dispose();
    });

    testWidgets('a refused cancellation is shown and the order stands', (
      WidgetTester tester,
    ) async {
      route('GET /v1/orders/ord-1', (_) => orderJson());
      route('GET /v1/orders/ord-1/cancellation', (_) => cancellationJson());
      route('POST /v1/orders/ord-1/cancel',
          (_) => errorBody('too_late', 'রান্না শুরু হয়ে গেছে'));
      backend.statuses['POST /v1/orders/ord-1/cancel'] = 409;
      final Dependencies dependencies = await pumpOrder(tester);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.cancelOrder),
      );
      expect(find.text('রান্না শুরু হয়ে গেছে'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a cancellation read that fails hides the button, not the order',
        (WidgetTester tester) async {
      route('GET /v1/orders/ord-1', (_) => orderJson());
      route('GET /v1/orders/ord-1/cancellation',
          (_) => errorBody('boom', 'সমস্যা'));
      backend.statuses['GET /v1/orders/ord-1/cancellation'] = 503;
      final Dependencies dependencies = await pumpOrder(tester);
      expect(find.text('GK-7F3K'), findsOneWidget);
      expect(find.text(bn.cancelOrder), findsNothing);
      dependencies.dispose();
    });

    testWidgets('a failed order read offers a retry', (
      WidgetTester tester,
    ) async {
      route('GET /v1/orders/ord-1', (_) => errorBody('not_found', 'পাওয়া যায়নি'));
      backend.statuses['GET /v1/orders/ord-1'] = 404;
      route('GET /v1/orders/ord-1/cancellation',
          (_) => errorBody('not_found', 'পাওয়া যায়নি'));
      backend.statuses['GET /v1/orders/ord-1/cancellation'] = 404;
      final Dependencies dependencies = await pumpOrder(tester);
      expect(find.text('পাওয়া যায়নি'), findsOneWidget);
      await tester.tap(find.bySemanticsLabel(core.retry));
      await tester.pumpAndSettle();
      expect(backend.to('/v1/orders/ord-1'), hasLength(2));
      dependencies.dispose();
    });

    testWidgets('an event with a reason prints it', (
      WidgetTester tester,
    ) async {
      final Map<String, Object?> rejected = orderJson(
        status: 'rejected',
        live: false,
      );
      (rejected['events']! as List<Object?>).add(<String, Object?>{
        'status': 'rejected',
        'label': 'দোকান ফিরিয়ে দিয়েছে',
        'actor': 'merchant',
        'reason': 'উপকরণ নেই',
        'at': '2026-09-21T10:06:00Z',
      });
      route('GET /v1/orders/ord-1', (_) => rejected);
      route('GET /v1/orders/ord-1/cancellation',
          (_) => cancellationJson(allowed: false));
      final Dependencies dependencies = await pumpOrder(tester);
      expect(find.text('উপকরণ নেই'), findsOneWidget);
      dependencies.dispose();
    });
  });

  group('payment', () {
    Future<Dependencies> pumpPayment(WidgetTester tester) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        const PaymentScreen(orderId: 'ord-1'),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      return dependencies;
    }

    testWidgets('the manual gateway gives no page, and the screen says so', (
      WidgetTester tester,
    ) async {
      route('POST /v1/payments/checkout', (_) => checkoutJson());
      final Dependencies dependencies = await pumpPayment(tester);
      expect(find.text('অপেক্ষমাণ'), findsOneWidget);
      expect(find.text(bn.noPaymentPage), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a gateway that does give one shows it', (
      WidgetTester tester,
    ) async {
      route('POST /v1/payments/checkout',
          (_) => checkoutJson(redirect: 'https://pay.test/abc'));
      final Dependencies dependencies = await pumpPayment(tester);
      expect(find.text('https://pay.test/abc'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('an ordinary deployment offers no way to complete a payment', (
      WidgetTester tester,
    ) async {
      route('POST /v1/payments/checkout', (_) => checkoutJson());
      final Dependencies dependencies = await pumpPayment(tester);
      expect(find.bySemanticsLabel(bn.completeDemoPayment), findsNothing);
      dependencies.dispose();
    });

    testWidgets('a demo deployment completes the payment from the screen', (
      WidgetTester tester,
    ) async {
      route('POST /v1/payments/checkout',
          (_) => checkoutJson(demoCompletion: true));
      route('POST /v1/payments/manual/complete', (_) => <String, Object?>{});
      route('GET /v1/payments/ord-1', (_) => paymentJson());

      final Dependencies dependencies = await pumpPayment(tester);
      expect(find.bySemanticsLabel(bn.completeDemoPayment), findsOneWidget);

      await tester.tap(find.bySemanticsLabel(bn.completeDemoPayment));
      await tester.pumpAndSettle();

      // The payment id came from the checkout, not from anywhere the screen
      // invented.
      final Map<String, Object?> sent =
          backend.to('/v1/payments/manual/complete').single.body!
              as Map<String, Object?>;
      expect(sent['payment_id'], 'pay-1');
      expect(sent['succeeded'], isTrue);

      // And the screen shows what the server says afterwards, rather than
      // assuming the completion worked.
      expect(find.text('পেমেন্ট হয়েছে'), findsOneWidget);
      // Once it is captured there is nothing left to complete.
      expect(find.bySemanticsLabel(bn.completeDemoPayment), findsNothing);
      dependencies.dispose();
    });

    testWidgets('a demo completion that fails is reported, not swallowed', (
      WidgetTester tester,
    ) async {
      route('POST /v1/payments/checkout',
          (_) => checkoutJson(demoCompletion: true));
      route('POST /v1/payments/manual/complete',
          (_) => errorBody('payments_unavailable', 'পেমেন্ট পড়া যায়নি।'));
      backend.statuses['POST /v1/payments/manual/complete'] = 503;

      final Dependencies dependencies = await pumpPayment(tester);
      await tester.tap(find.bySemanticsLabel(bn.completeDemoPayment));
      await tester.pumpAndSettle();

      expect(find.text('পেমেন্ট পড়া যায়নি।'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('re-reading shows the payment the server now has', (
      WidgetTester tester,
    ) async {
      route('POST /v1/payments/checkout', (_) => checkoutJson());
      route('GET /v1/payments/ord-1', (_) => paymentJson());
      final Dependencies dependencies = await pumpPayment(tester);
      await tester.tap(find.bySemanticsLabel(bn.checkPayment));
      await tester.pumpAndSettle();
      expect(find.text('পেমেন্ট হয়েছে'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a failed payment shows the gateway\'s reason', (
      WidgetTester tester,
    ) async {
      route('POST /v1/payments/checkout', (_) => checkoutJson());
      route('GET /v1/payments/ord-1',
          (_) => paymentJson(status: 'failed', reason: 'কার্ড বাতিল'));
      final Dependencies dependencies = await pumpPayment(tester);
      await tester.tap(find.bySemanticsLabel(bn.checkPayment));
      await tester.pumpAndSettle();
      expect(find.text('কার্ড বাতিল'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a re-read that fails leaves the checkout on screen', (
      WidgetTester tester,
    ) async {
      route('POST /v1/payments/checkout', (_) => checkoutJson());
      route('GET /v1/payments/ord-1', (_) => errorBody('boom', 'সমস্যা'));
      backend.statuses['GET /v1/payments/ord-1'] = 503;
      final Dependencies dependencies = await pumpPayment(tester);
      await tester.tap(find.bySemanticsLabel(bn.checkPayment));
      await tester.pumpAndSettle();
      expect(find.text('সমস্যা'), findsOneWidget);
      expect(find.text('অপেক্ষমাণ'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a checkout that never started offers a retry', (
      WidgetTester tester,
    ) async {
      route('POST /v1/payments/checkout',
          (_) => errorBody('already_paid', 'এই অর্ডার পরিশোধিত'));
      backend.statuses['POST /v1/payments/checkout'] = 409;
      final Dependencies dependencies = await pumpPayment(tester);
      expect(find.text('এই অর্ডার পরিশোধিত'), findsOneWidget);
      await tester.tap(find.bySemanticsLabel(core.retry));
      await tester.pumpAndSettle();
      expect(backend.to('/v1/payments/checkout'), hasLength(2));
      dependencies.dispose();
    });
  });
}
