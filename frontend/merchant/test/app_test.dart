import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_merchant/src/app.dart';
import 'package:goklay_merchant/src/app_scope.dart';
import 'package:goklay_merchant/src/dependencies.dart';
import 'package:goklay_merchant/src/environment.dart';
import 'package:goklay_merchant/src/l10n/merchant_strings.dart';
import 'package:goklay_merchant/src/screens/account_screen.dart';
import 'package:goklay_merchant/src/screens/board_screen.dart';
import 'package:goklay_merchant/src/screens/catalogue_screen.dart';
import 'package:goklay_merchant/src/screens/document_screen.dart';
import 'package:goklay_merchant/src/screens/holiday_screen.dart';
import 'package:goklay_merchant/src/screens/hours_screen.dart';
import 'package:goklay_merchant/src/screens/item_form_screen.dart';
import 'package:goklay_merchant/src/screens/main_shell.dart';
import 'package:goklay_merchant/src/screens/order_screen.dart';
import 'package:goklay_merchant/src/screens/shop_details_screen.dart';
import 'package:goklay_merchant/src/screens/shop_screen.dart';

import 'support/fixtures.dart';
import 'support/harness.dart';

/// Everything a signed-in shop's first screens ask for.
Map<String, Object? Function(SentRequest)> shellRoutes() =>
    <String, Object? Function(SentRequest)>{
      'GET /v1/merchants/me': (_) => merchantJson(),
      'GET /v1/me': (_) => profileJson(),
      'GET /v1/merchants/mch-1/orders': (_) => orderListJson(),
      'GET /v1/merchants/mch-1/catalogue/capabilities': (_) =>
          capabilitiesJson(),
      'GET /v1/merchants/mch-1/catalogue/categories': (_) =>
          <String, Object?>{
            'categories': <Object?>[categoryJson()],
          },
      'GET /v1/merchants/mch-1/catalogue/items': (_) => <String, Object?>{
        'items': <Object?>[itemJson()],
      },
      'GET /v1/merchants/mch-1/catalogue/combos': (_) => <String, Object?>{
        'combos': <Object?>[comboJson()],
      },
    };

void main() {
  const MerchantStringsBn bn = MerchantStringsBn();
  late FakeBackend backend;

  setUp(() => backend = FakeBackend(shellRoutes()));

  Future<Dependencies> pumpApp(
    WidgetTester tester, {
    bool signedIn = true,
  }) async {
    final Dependencies dependencies = Dependencies(
      environment: MerchantEnvironment(
        apiBaseUrl: Uri.parse('http://api.test'),
      ),
      httpClient: backend.client,
      storage: harnessStorage(signedIn: signedIn),
    );
    await tester.pumpWidget(
      GoklayMerchantApp(
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
      environment: MerchantEnvironment(
        apiBaseUrl: Uri.parse('http://api.test'),
      ),
      httpClient: backend.client,
    );
    await tester.pumpWidget(
      GoklayMerchantApp(
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
      GoklayMerchantApp(
        environment: MerchantEnvironment.fromCompileTime(),
        home: const Text('own graph', textDirection: TextDirection.ltr),
      ),
    );
    await tester.pump();
    expect(find.text('own graph'), findsOneWidget);
  });

  testWidgets('a signed-in owner with a shop lands on the board', (
    WidgetTester tester,
  ) async {
    final Dependencies dependencies = await pumpApp(tester);
    expect(find.byType(MerchantShell), findsOneWidget);
    expect(find.byType(BoardScreen), findsOneWidget);
    dependencies.dispose();
  });

  testWidgets('an owner with no shop is shown the registration form', (
    WidgetTester tester,
  ) async {
    backend.routes['GET /v1/merchants/me'] =
        (_) => errorBody('not_found', 'দোকান নেই');
    backend.statuses['GET /v1/merchants/me'] = 404;
    backend.routes['GET /v1/merchants/registration-requirements'] =
        (_) => requirementsJson();
    backend.routes['POST /v1/merchants'] =
        (_) => merchantJson(status: 'draft');
    final Dependencies dependencies = await pumpApp(tester);
    expect(find.byType(ShopDetailsScreen), findsOneWidget);
    expect(find.text(bn.registerSubtitle), findsOneWidget);

    await reveal(tester, find.widgetWithText(GoklayButton, bn.register));
    await scrollToTop(tester);
    await revealAndTap(tester, find.text(bn.typeRestaurant));
    await enterInto(tester, 'lat', '23.75');
    await enterInto(tester, 'lng', '90.39');
    await revealAndTap(
      tester,
      find.widgetWithText(GoklayButton, bn.register),
    );
    // The gate swaps to the shop the server just created.
    expect(find.byType(MerchantShell), findsOneWidget);
    dependencies.dispose();
  });

  testWidgets('a read that fails for another reason offers a retry', (
    WidgetTester tester,
  ) async {
    backend.routes['GET /v1/merchants/me'] = (_) => errorBody('b', 'সমস্যা');
    backend.statuses['GET /v1/merchants/me'] = 503;
    final Dependencies dependencies = await pumpApp(tester);
    expect(find.byType(ErrorView), findsOneWidget);
    await tester.tap(find.bySemanticsLabel(const GoklayStringsBn().retry));
    await tester.pumpAndSettle();
    expect(backend.to('/v1/merchants/me'), hasLength(2));
    dependencies.dispose();
  });

  testWidgets('signing in sends the merchant role', (
    WidgetTester tester,
  ) async {
    backend.routes['POST /v1/auth/otp/request'] = (_) => <String, Object?>{
      'expires_in': 300,
      'resend_after': 2,
    };
    backend.routes['POST /v1/auth/otp/verify'] = (_) => tokenPairJson();
    final Dependencies dependencies = await pumpApp(tester, signedIn: false);
    await tester.enterText(find.byType(TextField), '01712345678');
    await tester.pumpAndSettle();
    await tester.tap(find.bySemanticsLabel(const GoklayStringsBn().sendCode));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField), '123456');
    await tester.pumpAndSettle();
    await tester.tap(
      find.bySemanticsLabel(const GoklayStringsBn().verifyCode),
    );
    await tester.pumpAndSettle();
    expect(
      (backend.to('/v1/auth/otp/verify').single.body!
          as Map<String, Object?>)['role'],
      'merchant',
    );
    expect(find.byType(MerchantShell), findsOneWidget);
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
    await tester.enterText(find.byType(TextField), '01712345678');
    await tester.pumpAndSettle();
    await tester.tap(find.bySemanticsLabel(const GoklayStringsBn().sendCode));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField), '999999');
    await tester.pumpAndSettle();
    await tester.tap(
      find.bySemanticsLabel(const GoklayStringsBn().verifyCode),
    );
    await tester.pumpAndSettle();
    expect(find.text('কোড মেলেনি'), findsOneWidget);
    await tester.tap(find.bySemanticsLabel(const GoklayStringsBn().startOver));
    await tester.pumpAndSettle();
    expect(find.text(const GoklayStringsBn().phoneLabel), findsOneWidget);
    dependencies.dispose();
  });

  group('the tabs', () {
    testWidgets('each one opens its own screen', (WidgetTester tester) async {
      final Dependencies dependencies = await pumpApp(tester);
      await tester.tap(find.text(bn.catalogueTitle));
      await tester.pumpAndSettle();
      expect(find.byType(CatalogueScreen), findsOneWidget);

      await tester.tap(find.text(bn.shopTitle));
      await tester.pumpAndSettle();
      expect(find.byType(ShopScreen), findsOneWidget);

      await tester.tap(find.text(bn.accountTitle));
      await tester.pumpAndSettle();
      expect(find.byType(MerchantAccountScreen), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('the board opens an order', (WidgetTester tester) async {
      backend.routes['GET /v1/merchants/mch-1/orders/ord-1'] =
          (_) => orderJson();
      final Dependencies dependencies = await pumpApp(tester);
      await tester.tap(find.bySemanticsLabel('GK-7F3K'));
      await tester.pumpAndSettle();
      expect(find.byType(OrderScreen), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('the catalogue opens the item form', (
      WidgetTester tester,
    ) async {
      backend.routes['POST /v1/merchants/mch-1/catalogue/items'] =
          (_) => itemJson();
      final Dependencies dependencies = await pumpApp(tester);
      await tester.tap(find.text(bn.catalogueTitle));
      await tester.pumpAndSettle();
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.addItem),
      );
      expect(find.byType(ItemFormScreen), findsOneWidget);
      await enterInto(tester, bn.itemName, 'কাচ্চি');
      await enterInto(tester, bn.itemPriceMinor, '32000');
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.addItem),
      );
      expect(find.byType(CatalogueScreen), findsOneWidget);
      expect(backend.to('/v1/merchants/mch-1/catalogue/items'), hasLength(3));
      dependencies.dispose();
    });

    testWidgets('the shop tab reaches details, hours, holiday and papers', (
      WidgetTester tester,
    ) async {
      backend.routes['GET /v1/merchants/registration-requirements'] =
          (_) => requirementsJson();
      backend.routes['PUT /v1/merchants/me/hours'] = (_) => merchantJson();
      backend.routes['PUT /v1/merchants/me/holiday'] = (_) => merchantJson();
      backend.routes['POST /v1/merchants/me/documents'] =
          (_) => merchantJson();
      final Dependencies dependencies = await pumpApp(tester);
      await tester.tap(find.text(bn.shopTitle));
      await tester.pumpAndSettle();

      backend.routes['PATCH /v1/merchants/me'] = (_) => merchantJson();
      await revealAndTap(
        tester,
        find.widgetWithText(SettingRow, bn.shopRow),
      );
      expect(find.byType(ShopDetailsScreen), findsOneWidget);
      await revealAndTap(tester, find.widgetWithText(GoklayButton, bn.save));
      expect(find.byType(ShopScreen), findsOneWidget);
      expect(backend.to('/v1/merchants/me'), isNotEmpty);

      await revealAndTap(
        tester,
        find.widgetWithText(SettingRow, bn.hoursRow),
      );
      expect(find.byType(HoursScreen), findsOneWidget);
      await revealAndTap(tester, find.widgetWithText(GoklayButton, bn.save));
      expect(find.byType(ShopScreen), findsOneWidget);

      await revealAndTap(
        tester,
        find.widgetWithText(SettingRow, bn.holidayRow),
      );
      expect(find.byType(HolidayScreen), findsOneWidget);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.closeShop),
      );
      expect(find.byType(ShopScreen), findsOneWidget);

      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.addDocument),
      );
      expect(find.byType(DocumentScreen), findsOneWidget);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.addDocument),
      );
      expect(find.byType(ShopScreen), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('the account signs out and switches language', (
      WidgetTester tester,
    ) async {
      backend.routes['POST /v1/auth/logout'] = (_) => null;
      backend.routes['PATCH /v1/me'] = (_) => profileJson();
      final Dependencies dependencies = await pumpApp(tester);
      await tester.tap(find.text(bn.accountTitle));
      await tester.pumpAndSettle();
      await revealAndTap(
        tester,
        find.widgetWithText(SettingRow, 'ভাষা / Language'),
      );
      expect(find.byType(MerchantLanguageScreen), findsOneWidget);
      await tester.tap(find.widgetWithText(SettingRow, 'English'));
      await tester.pumpAndSettle();
      expect(
        tester.widget<MaterialApp>(find.byType(MaterialApp)).locale,
        const Locale('en'),
      );
      await goBack(tester);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, const MerchantStringsEn().signOut),
      );
      expect(find.text(const GoklayStringsEn().phoneLabel), findsOneWidget);
      dependencies.dispose();
    });
  });

  testWidgets('the scope is found from anywhere below, and notices a swap', (
    WidgetTester tester,
  ) async {
    final Dependencies first = Dependencies(
      environment: MerchantEnvironment(
        apiBaseUrl: Uri.parse('http://api.test'),
      ),
      httpClient: backend.client,
    );
    late Dependencies seen;
    Widget scope(Dependencies dependencies) => MerchantScope(
      dependencies: dependencies,
      child: Builder(
        builder: (BuildContext context) {
          seen = MerchantScope.of(context);
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
