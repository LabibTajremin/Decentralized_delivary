import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_merchant/src/api/models/merchant.dart';
import 'package:goklay_merchant/src/dependencies.dart';
import 'package:goklay_merchant/src/l10n/merchant_strings.dart';
import 'package:goklay_merchant/src/screens/document_screen.dart';
import 'package:goklay_merchant/src/screens/holiday_screen.dart';
import 'package:goklay_merchant/src/screens/hours_screen.dart';
import 'package:goklay_merchant/src/screens/shop_details_screen.dart';
import 'package:goklay_merchant/src/screens/shop_screen.dart';

import 'support/fixtures.dart';
import 'support/harness.dart';

void main() {
  const MerchantStringsBn bn = MerchantStringsBn();
  const GoklayStringsBn core = GoklayStringsBn();
  late FakeBackend backend;

  setUp(() => backend = FakeBackend(<String, Object? Function(SentRequest)>{}));

  void route(String key, Object? Function(SentRequest) handler) =>
      backend.routes[key] = handler;

  group('the shop screen', () {
    Future<Dependencies> pumpShop(
      WidgetTester tester, {
      Future<bool> Function(Merchant)? onAddDocument,
    }) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        ShopScreen(
          onEditDetails: (_) {},
          onHours: (_) {},
          onHoliday: (_) {},
          onAddDocument: onAddDocument ?? (_) async => false,
        ),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      return dependencies;
    }

    testWidgets('shows the shop, its opening sentence and its papers', (
      WidgetTester tester,
    ) async {
      route('GET /v1/merchants/me', (_) => merchantJson());
      final Dependencies dependencies = await pumpShop(tester);
      expect(find.text('নূরজাহান হোটেল'), findsOneWidget);
      expect(find.text('খোলা আছে — বন্ধ হবে 22:00'), findsOneWidget);
      expect(find.text('trade_licence'), findsOneWidget);
      expect(find.text('TL-99'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('what is still wanted comes from the server', (
      WidgetTester tester,
    ) async {
      route(
        'GET /v1/merchants/me',
        (_) => merchantJson(
          status: 'draft',
          missing: <String>['food_licence'],
        ),
      );
      final Dependencies dependencies = await pumpShop(tester);
      expect(find.text(bn.stillNeeded), findsOneWidget);
      expect(find.text('food_licence'), findsOneWidget);
      expect(find.text(bn.submitForReview), findsNothing);
      dependencies.dispose();
    });

    testWidgets('the submit button appears only when can_submit says so', (
      WidgetTester tester,
    ) async {
      route(
        'GET /v1/merchants/me',
        (_) => merchantJson(status: 'draft', canSubmit: true),
      );
      route(
        'POST /v1/merchants/me/submit',
        (_) => merchantJson(status: 'pending_review'),
      );
      final Dependencies dependencies = await pumpShop(tester);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.submitForReview),
      );
      expect(backend.to('/v1/merchants/me/submit'), hasLength(1));
      expect(find.text(bn.awaitingReview), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a refused submission is shown', (WidgetTester tester) async {
      route(
        'GET /v1/merchants/me',
        (_) => merchantJson(status: 'draft', canSubmit: true),
      );
      route(
        'POST /v1/merchants/me/submit',
        (_) => errorBody('documents_missing', 'কাগজ বাকি আছে'),
      );
      backend.statuses['POST /v1/merchants/me/submit'] = 409;
      final Dependencies dependencies = await pumpShop(tester);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.submitForReview),
      );
      expect(find.text('কাগজ বাকি আছে'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a rejection note and a holiday are both said plainly', (
      WidgetTester tester,
    ) async {
      route(
        'GET /v1/merchants/me',
        (_) => merchantJson(
          status: 'rejected',
          reviewNote: 'কাগজ পড়া যায়নি',
          holiday: true,
        ),
      );
      final Dependencies dependencies = await pumpShop(tester);
      expect(find.text('কাগজ পড়া যায়নি'), findsOneWidget);
      expect(find.text(bn.onHoliday), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('adding a document re-reads the shop, declining does not', (
      WidgetTester tester,
    ) async {
      route('GET /v1/merchants/me', (_) => merchantJson());
      bool added = false;
      final Dependencies dependencies = await pumpShop(
        tester,
        onAddDocument: (_) async => added,
      );
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.addDocument),
      );
      expect(backend.to('/v1/merchants/me'), hasLength(1));
      added = true;
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.addDocument),
      );
      expect(backend.to('/v1/merchants/me'), hasLength(2));
      dependencies.dispose();
    });

    testWidgets('every row opens its screen', (WidgetTester tester) async {
      route('GET /v1/merchants/me', (_) => merchantJson());
      int details = 0;
      int hours = 0;
      int holiday = 0;
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        ShopScreen(
          onEditDetails: (_) => details += 1,
          onHours: (_) => hours += 1,
          onHoliday: (_) => holiday += 1,
          onAddDocument: (_) async => false,
        ),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      await revealAndTap(
        tester,
        find.widgetWithText(SettingRow, bn.shopRow),
      );
      await revealAndTap(
        tester,
        find.widgetWithText(SettingRow, bn.hoursRow),
      );
      await revealAndTap(
        tester,
        find.widgetWithText(SettingRow, bn.holidayRow),
      );
      expect(<int>[details, hours, holiday], <int>[1, 1, 1]);
      dependencies.dispose();
    });

    testWidgets('a failed read offers a retry', (WidgetTester tester) async {
      route('GET /v1/merchants/me', (_) => errorBody('boom', 'সমস্যা'));
      backend.statuses['GET /v1/merchants/me'] = 503;
      final Dependencies dependencies = await pumpShop(tester);
      await tester.tap(find.bySemanticsLabel(core.retry));
      await tester.pumpAndSettle();
      expect(backend.to('/v1/merchants/me'), hasLength(2));
      dependencies.dispose();
    });
  });

  group('the details form', () {
    Future<Dependencies> pumpDetails(
      WidgetTester tester, {
      Merchant? shop,
      void Function(Merchant)? onSaved,
    }) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        ShopDetailsScreen(shop: shop, onSaved: onSaved ?? (_) {}),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      return dependencies;
    }

    testWidgets('the types and their papers come from the server', (
      WidgetTester tester,
    ) async {
      route(
        'GET /v1/merchants/registration-requirements',
        (_) => requirementsJson(),
      );
      final Dependencies dependencies = await pumpDetails(tester);
      expect(find.text(bn.typeRestaurant), findsOneWidget);
      expect(find.text(bn.typeGrocery), findsOneWidget);
      expect(find.text(bn.typePharmacy), findsOneWidget);
      expect(find.text('trade_licence, drug_licence'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('nothing is sent until a type and a point are chosen', (
      WidgetTester tester,
    ) async {
      route(
        'GET /v1/merchants/registration-requirements',
        (_) => requirementsJson(),
      );
      route('POST /v1/merchants', (_) => merchantJson(status: 'draft'));
      Merchant? saved;
      final Dependencies dependencies = await pumpDetails(
        tester,
        onSaved: (Merchant shop) => saved = shop,
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
      await scrollToTop(tester);
      await revealAndTap(tester, find.text(bn.typeRestaurant));
      await tester.pumpAndSettle();
      await _fillPoint(tester);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.register),
      );
      expect(saved, isNotNull);
      expect(
        (backend.to('/v1/merchants').single.body! as Map<String, Object?>)['type'],
        'restaurant',
      );
      dependencies.dispose();
    });

    testWidgets('editing an existing shop patches instead of registering', (
      WidgetTester tester,
    ) async {
      route(
        'GET /v1/merchants/registration-requirements',
        (_) => requirementsJson(),
      );
      route('PATCH /v1/merchants/me', (_) => merchantJson());
      final Dependencies dependencies = await pumpDetails(
        tester,
        shop: Merchant.fromJson(merchantJson()),
      );
      expect(find.text(bn.shopRow), findsWidgets);
      await revealAndTap(tester, find.widgetWithText(GoklayButton, bn.save));
      expect(backend.to('/v1/merchants/me'), hasLength(1));
      expect(backend.to('/v1/merchants'), isEmpty);
      dependencies.dispose();
    });

    testWidgets('a refusal is shown in the server\'s words', (
      WidgetTester tester,
    ) async {
      route(
        'GET /v1/merchants/registration-requirements',
        (_) => requirementsJson(),
      );
      route(
        'POST /v1/merchants',
        (_) => errorBody('outside_service_area', 'এই এলাকায় সেবা নেই'),
      );
      backend.statuses['POST /v1/merchants'] = 404;
      final Dependencies dependencies = await pumpDetails(tester);
      await revealAndTap(tester, find.text(bn.typeGrocery));
      await _fillPoint(tester);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.register),
      );
      expect(find.text('এই এলাকায় সেবা নেই'), findsOneWidget);
      dependencies.dispose();
    });

    test('an unknown type falls back to its own code as a label', () {
      expect(
        ShopDetailsScreen.typeLabel(bn, 'restaurant'),
        bn.typeRestaurant,
      );
      expect(ShopDetailsScreen.typeLabel(bn, 'bakery'), 'bakery');
    });
  });

  group('hours and holiday', () {
    testWidgets('a blank day is left out rather than sent empty', (
      WidgetTester tester,
    ) async {
      route('PUT /v1/merchants/me/hours', (_) => merchantJson());
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        HoursScreen(
          shop: Merchant.fromJson(merchantJson()),
          onSaved: (_) {},
        ),
        dependencies: dependencies,
      );
      await revealAndTap(tester, find.widgetWithText(GoklayButton, bn.save));
      final Map<String, Object?> days =
          (backend.to('/v1/merchants/me/hours').single.body!
              as Map<String, Object?>)['days']! as Map<String, Object?>;
      expect(days.keys, <String>['0', '1']);
      expect(days['0'], <String>['09:00-22:00']);
      dependencies.dispose();
    });

    testWidgets('a rejected window is shown as the server refused it', (
      WidgetTester tester,
    ) async {
      route(
        'PUT /v1/merchants/me/hours',
        (_) => errorBody('overlapping_windows', 'সময় একটার সাথে আরেকটা মিলে গেছে'),
      );
      backend.statuses['PUT /v1/merchants/me/hours'] = 400;
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        HoursScreen(
          shop: Merchant.fromJson(merchantJson()),
          onSaved: (_) {},
        ),
        dependencies: dependencies,
      );
      await revealAndTap(tester, find.widgetWithText(GoklayButton, bn.save));
      expect(
        find.text('সময় একটার সাথে আরেকটা মিলে গেছে'),
        findsOneWidget,
      );
      dependencies.dispose();
    });

    test('the weekday names are Sunday-first, as the API keys are', () {
      expect(HoursScreen.weekdays.first, '0');
      expect(HoursScreen.dayNames(bn).first, 'রবিবার');
      expect(HoursScreen.dayNames(const MerchantStringsEn()).first, 'Sunday');
    });

    testWidgets('a holiday is set and then cleared', (
      WidgetTester tester,
    ) async {
      route('PUT /v1/merchants/me/holiday', (_) => merchantJson());
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        HolidayScreen(
          shop: Merchant.fromJson(merchantJson(holiday: true)),
          onSaved: (_) {},
        ),
        dependencies: dependencies,
      );
      expect(find.text(bn.onHoliday), findsOneWidget);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.reopenShop),
      );
      expect(
        backend.to('/v1/merchants/me/holiday').single.body,
        isEmpty,
      );
      dependencies.dispose();
    });

    testWidgets('closing sends the reason and the date', (
      WidgetTester tester,
    ) async {
      route('PUT /v1/merchants/me/holiday', (_) => merchantJson());
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        HolidayScreen(
          shop: Merchant.fromJson(merchantJson()),
          onSaved: (_) {},
        ),
        dependencies: dependencies,
      );
      expect(find.text(bn.reopenShop), findsNothing);
      await enterInto(tester, bn.holidayReason, 'ঈদের ছুটি');
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.closeShop),
      );
      expect(
        (backend.to('/v1/merchants/me/holiday').single.body!
            as Map<String, Object?>)['reason'],
        'ঈদের ছুটি',
      );
      dependencies.dispose();
    });

    testWidgets('a refused holiday is shown', (WidgetTester tester) async {
      route(
        'PUT /v1/merchants/me/holiday',
        (_) => errorBody('live_orders', 'চলমান অর্ডার আছে'),
      );
      backend.statuses['PUT /v1/merchants/me/holiday'] = 409;
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        HolidayScreen(
          shop: Merchant.fromJson(merchantJson()),
          onSaved: (_) {},
        ),
        dependencies: dependencies,
      );
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.closeShop),
      );
      expect(find.text('চলমান অর্ডার আছে'), findsOneWidget);
      dependencies.dispose();
    });
  });

  group('the document form', () {
    testWidgets('it offers what is still wanted, and adds one', (
      WidgetTester tester,
    ) async {
      route('POST /v1/merchants/me/documents', (_) => merchantJson());
      final Dependencies dependencies = await harnessDependencies(backend);
      Merchant? added;
      await pumpScreen(
        tester,
        DocumentScreen(
          shop: Merchant.fromJson(
            merchantJson(missing: <String>['food_licence']),
          ),
          onAdded: (Merchant shop) => added = shop,
        ),
        dependencies: dependencies,
      );
      expect(find.text('food_licence'), findsOneWidget);
      expect(find.text('trade_licence'), findsNothing);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.addDocument),
      );
      expect(added, isNotNull);
      expect(
        (backend.to('/v1/merchants/me/documents').single.body!
            as Map<String, Object?>)['kind'],
        'food_licence',
      );
      dependencies.dispose();
    });

    testWidgets('with nothing missing it offers the required list', (
      WidgetTester tester,
    ) async {
      route('POST /v1/merchants/me/documents', (_) => merchantJson());
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        DocumentScreen(
          shop: Merchant.fromJson(merchantJson()),
          onAdded: (_) {},
        ),
        dependencies: dependencies,
      );
      expect(find.text('trade_licence'), findsOneWidget);
      expect(find.text('food_licence'), findsOneWidget);
      await revealAndTap(tester, find.text('food_licence'));
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.addDocument),
      );
      expect(
        (backend.to('/v1/merchants/me/documents').single.body!
            as Map<String, Object?>)['kind'],
        'food_licence',
      );
      dependencies.dispose();
    });

    testWidgets('a refusal is shown', (WidgetTester tester) async {
      route(
        'POST /v1/merchants/me/documents',
        (_) => errorBody('bad_kind', 'এই কাগজ লাগে না'),
      );
      backend.statuses['POST /v1/merchants/me/documents'] = 400;
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        DocumentScreen(
          shop: Merchant.fromJson(merchantJson()),
          onAdded: (_) {},
        ),
        dependencies: dependencies,
      );
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.addDocument),
      );
      expect(find.text('এই কাগজ লাগে না'), findsOneWidget);
      dependencies.dispose();
    });
  });
}

/// Types a coordinate into the two fields every form here has.
Future<void> _fillPoint(WidgetTester tester) async {
  await enterInto(tester, 'lat', '23.7509');
  await enterInto(tester, 'lng', '90.3925');
}
