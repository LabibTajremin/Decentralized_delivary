import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_customer/src/api/models/order.dart';
import 'package:goklay_customer/src/dependencies.dart';
import 'package:goklay_customer/src/l10n/customer_strings.dart';
import 'package:goklay_customer/src/screens/review_screen.dart';
import 'package:goklay_customer/src/screens/shop_reviews_screen.dart';
import 'package:goklay_customer/src/screens/support_screen.dart';

import 'support/fixtures.dart';
import 'support/harness.dart';

void main() {
  const CustomerStringsBn bn = CustomerStringsBn();
  const GoklayStringsBn core = GoklayStringsBn();
  late FakeBackend backend;

  setUp(() => backend = FakeBackend(<String, Object? Function(SentRequest)>{}));

  void route(String key, Object? Function(SentRequest) handler) =>
      backend.routes[key] = handler;

  group('leaving a review', () {
    Future<Dependencies> pumpReview(
      WidgetTester tester, {
      String? partnerId,
    }) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        ReviewScreen(
          order: Order.fromJson(
            orderJson(status: 'delivered', live: false, partnerId: partnerId),
          ),
        ),
        dependencies: dependencies,
      );
      return dependencies;
    }

    testWidgets('a rider who never collected it is not offered', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await pumpReview(tester);
      expect(find.text(bn.rateShop), findsOneWidget);
      expect(find.text(bn.rateRider), findsNothing);
      dependencies.dispose();
    });

    testWidgets('nothing can be sent until a star is chosen', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await pumpReview(tester);
      expect(
        tester
            .widget<GoklayButton>(
              find.widgetWithText(GoklayButton, bn.submitReview),
            )
            .isEnabled,
        isFalse,
      );
      dependencies.dispose();
    });

    testWidgets('the shop alone is one review, named by the order\'s ids', (
      WidgetTester tester,
    ) async {
      route('POST /v1/reviews', (_) => reviewJson());
      final Dependencies dependencies = await pumpReview(tester);
      await tester.tap(find.bySemanticsLabel('4').first);
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField), 'ভালো ছিল');
      await tester.pumpAndSettle();
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.submitReview),
      );
      final List<SentRequest> sent = backend.to('/v1/reviews');
      expect(sent, hasLength(1));
      final Map<String, Object?> body = sent.single.body! as Map<String, Object?>;
      expect(body['subject'], 'merchant');
      expect(body['subject_id'], 'mer-1');
      expect(body['rating'], 4);
      expect(body['comment'], 'ভালো ছিল');
      expect(find.text(bn.reviewThanks), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('shop and rider are two reviews, sent separately', (
      WidgetTester tester,
    ) async {
      route('POST /v1/reviews', (_) => reviewJson());
      final Dependencies dependencies = await pumpReview(
        tester,
        partnerId: 'ptn-1',
      );
      final Finder shopStars = find
          .descendant(
            of: find.byType(StarRating).first,
            matching: find.bySemanticsLabel('5'),
          );
      final Finder riderStars = find
          .descendant(
            of: find.byType(StarRating).last,
            matching: find.bySemanticsLabel('3'),
          );
      await tester.tap(shopStars);
      await tester.pumpAndSettle();
      await tester.tap(riderStars);
      await tester.pumpAndSettle();
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.submitReview),
      );
      final List<SentRequest> sent = backend.to('/v1/reviews');
      expect(sent, hasLength(2));
      expect(
        (sent.first.body! as Map<String, Object?>)['subject'],
        'merchant',
      );
      expect((sent.last.body! as Map<String, Object?>)['subject'], 'partner');
      expect((sent.last.body! as Map<String, Object?>)['subject_id'], 'ptn-1');
      dependencies.dispose();
    });

    testWidgets('a refusal is shown and nothing is claimed to have sent', (
      WidgetTester tester,
    ) async {
      route('POST /v1/reviews', (_) => errorBody('duplicate', 'আগে দিয়েছেন'));
      backend.statuses['POST /v1/reviews'] = 409;
      final Dependencies dependencies = await pumpReview(tester);
      await tester.tap(find.bySemanticsLabel('4').first);
      await tester.pumpAndSettle();
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.submitReview),
      );
      expect(find.text('আগে দিয়েছেন'), findsOneWidget);
      expect(find.text(bn.reviewThanks), findsNothing);
      dependencies.dispose();
    });
  });

  group('a shop\'s reviews', () {
    Future<Dependencies> pumpShopReviews(WidgetTester tester) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        const ShopReviewsScreen(
          merchantId: 'mer-1',
          merchantName: 'Kacchi Bhai',
        ),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      return dependencies;
    }

    testWidgets('the average is the server\'s, not one averaged here', (
      WidgetTester tester,
    ) async {
      route('GET /v1/reviews',
          (_) => <String, Object?>{'reviews': <Object?>[reviewJson(rating: 1)]});
      route('GET /v1/ratings', (_) => ratingJson());
      final Dependencies dependencies = await pumpShopReviews(tester);
      expect(find.text('4.5 (4)'), findsOneWidget);
      expect(find.text('ভালো'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a shop nobody has rated says so', (WidgetTester tester) async {
      route('GET /v1/reviews',
          (_) => <String, Object?>{'reviews': <Object?>[]});
      route('GET /v1/ratings', (_) => ratingJson(count: 0));
      final Dependencies dependencies = await pumpShopReviews(tester);
      expect(find.text(bn.noRatingYet), findsOneWidget);
      expect(find.text(bn.noReviews), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a missing aggregate does not blank the reviews', (
      WidgetTester tester,
    ) async {
      route('GET /v1/reviews',
          (_) => <String, Object?>{'reviews': <Object?>[reviewJson()]});
      route('GET /v1/ratings', (_) => errorBody('boom', 'সমস্যা'));
      backend.statuses['GET /v1/ratings'] = 503;
      final Dependencies dependencies = await pumpShopReviews(tester);
      expect(find.text('ভালো'), findsOneWidget);
      expect(find.text(bn.noRatingYet), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a failed read offers a retry', (WidgetTester tester) async {
      route('GET /v1/reviews', (_) => errorBody('boom', 'সমস্যা'));
      backend.statuses['GET /v1/reviews'] = 503;
      route('GET /v1/ratings', (_) => ratingJson());
      final Dependencies dependencies = await pumpShopReviews(tester);
      expect(find.byType(ErrorView), findsOneWidget);
      await tester.tap(find.bySemanticsLabel(core.retry));
      await tester.pumpAndSettle();
      expect(backend.to('/v1/reviews'), hasLength(2));
      dependencies.dispose();
    });
  });

  group('support', () {
    Future<Dependencies> pumpSupport(
      WidgetTester tester, {
      String? orderId,
    }) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        SupportScreen(orderId: orderId),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      return dependencies;
    }

    testWidgets('without an order there is no form, only the list', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me/support/tickets', (_) => <String, Object?>{
        'tickets': <Object?>[ticketJson(), ticketJson(status: 'resolved')],
      });
      final Dependencies dependencies = await pumpSupport(tester);
      expect(find.byType(TextField), findsNothing);
      expect(find.text(bn.ticketOpen), findsOneWidget);
      expect(find.text(bn.ticketResolved), findsOneWidget);
      expect(find.text('ফেরত দেওয়া হয়েছে'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('an empty list says so', (WidgetTester tester) async {
      route('GET /v1/me/support/tickets',
          (_) => <String, Object?>{'tickets': <Object?>[]});
      final Dependencies dependencies = await pumpSupport(tester);
      expect(find.text(bn.noTickets), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('raising one needs a subject, then re-reads the list', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me/support/tickets',
          (_) => <String, Object?>{'tickets': <Object?>[]});
      route('POST /v1/support/tickets', (_) => ticketJson());
      final Dependencies dependencies = await pumpSupport(
        tester,
        orderId: 'ord-1',
      );
      expect(
        tester
            .widget<GoklayButton>(
              find.widgetWithText(GoklayButton, bn.raiseTicket),
            )
            .isEnabled,
        isFalse,
      );
      await tester.enterText(find.byType(TextField), 'খাবার ঠান্ডা ছিল');
      await tester.pumpAndSettle();
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.raiseTicket),
      );
      final Map<String, Object?> body =
          backend.to('/v1/support/tickets').single.body!
              as Map<String, Object?>;
      expect(body['order_id'], 'ord-1');
      expect(body['subject'], 'খাবার ঠান্ডা ছিল');
      expect(backend.to('/v1/me/support/tickets'), hasLength(2));
      dependencies.dispose();
    });

    testWidgets('a refusal is shown', (WidgetTester tester) async {
      route('GET /v1/me/support/tickets',
          (_) => <String, Object?>{'tickets': <Object?>[]});
      route('POST /v1/support/tickets',
          (_) => errorBody('not_yours', 'এই অর্ডার আপনার নয়'));
      backend.statuses['POST /v1/support/tickets'] = 403;
      final Dependencies dependencies = await pumpSupport(
        tester,
        orderId: 'ord-1',
      );
      await tester.enterText(find.byType(TextField), 'সমস্যা');
      await tester.pumpAndSettle();
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.raiseTicket),
      );
      expect(find.text('এই অর্ডার আপনার নয়'), findsOneWidget);
      dependencies.dispose();
    });
  });
}
