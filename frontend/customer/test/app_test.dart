import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_customer/src/app.dart';
import 'package:goklay_customer/src/app_scope.dart';
import 'package:goklay_customer/src/dependencies.dart';
import 'package:goklay_customer/src/environment.dart';
import 'package:goklay_customer/src/l10n/customer_strings.dart';
import 'package:goklay_customer/src/screens/account_screen.dart';
import 'package:goklay_customer/src/screens/add_address_screen.dart';
import 'package:goklay_customer/src/screens/addresses_screen.dart';
import 'package:goklay_customer/src/screens/cart_screen.dart';
import 'package:goklay_customer/src/screens/home_screen.dart';
import 'package:goklay_customer/src/screens/item_screen.dart';
import 'package:goklay_customer/src/screens/language_screen.dart';
import 'package:goklay_customer/src/screens/main_shell.dart';
import 'package:goklay_customer/src/screens/notifications_screen.dart';
import 'package:goklay_customer/src/screens/order_screen.dart';
import 'package:goklay_customer/src/screens/orders_screen.dart';
import 'package:goklay_customer/src/screens/payment_screen.dart';
import 'package:goklay_customer/src/screens/place_order_screen.dart';
import 'package:goklay_customer/src/screens/placeholder_screen.dart';
import 'package:goklay_customer/src/screens/profile_screen.dart';
import 'package:goklay_customer/src/screens/review_cart_screen.dart';
import 'package:goklay_customer/src/screens/review_screen.dart';
import 'package:goklay_customer/src/screens/security_screen.dart';
import 'package:goklay_customer/src/screens/shop_reviews_screen.dart';
import 'package:goklay_customer/src/screens/shop_screen.dart';
import 'package:goklay_customer/src/screens/sign_in_options_screen.dart';
import 'package:goklay_customer/src/screens/support_screen.dart';
import 'package:goklay_customer/src/screens/tracking_screen.dart';
import 'package:goklay_customer/src/widgets/cards.dart';

import 'support/fixtures.dart';
import 'support/harness.dart';

/// Everything a signed-in customer's first screens ask for.
Map<String, Object? Function(SentRequest)> shellRoutes() =>
    <String, Object? Function(SentRequest)>{
      'GET /v1/me': (_) => profileJson(),
      'GET /v1/me/addresses': (_) => <String, Object?>{
        'addresses': <Object?>[addressJson()],
      },
      'GET /v1/discovery/merchants': (_) => searchJson(),
      'GET /v1/cart': (_) => cartJson(),
      'GET /v1/orders': (_) => <String, Object?>{
        'orders': <Object?>[orderJson()],
        'total': 1,
      },
    };

void main() {
  const CustomerStringsBn bn = CustomerStringsBn();
  const GoklayStringsBn core = GoklayStringsBn();
  late FakeBackend backend;

  setUp(() => backend = FakeBackend(shellRoutes()));

  Future<Dependencies> pumpApp(
    WidgetTester tester, {
    bool signedIn = true,
  }) async {
    final Dependencies dependencies = Dependencies(
      environment: CustomerEnvironment(
        apiBaseUrl: Uri.parse('http://api.test'),
      ),
      httpClient: backend.client,
      storage: harnessStorage(signedIn: signedIn),
    );
    await tester.pumpWidget(
      GoklayCustomerApp(
        environment: dependencies.environment,
        dependencies: dependencies,
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));
    await tester.pumpAndSettle();
    return dependencies;
  }

  group('the app', () {
    testWidgets('an unknown handset locale still opens in Bengali', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await pumpApp(tester);
      expect(
        tester.widget<MaterialApp>(find.byType(MaterialApp)).locale,
        const Locale('bn'),
      );
      dependencies.dispose();
    });

    testWidgets('a test-supplied home replaces the whole flow', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = Dependencies(
        environment: CustomerEnvironment(
          apiBaseUrl: Uri.parse('http://api.test'),
        ),
        httpClient: backend.client,
      );
      await tester.pumpWidget(
        GoklayCustomerApp(
          environment: dependencies.environment,
          dependencies: dependencies,
          home: const Text('replaced', textDirection: TextDirection.ltr),
        ),
      );
      await tester.pump();
      expect(find.text('replaced'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('with no graph given it builds its own', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        GoklayCustomerApp(
          environment: CustomerEnvironment.fromCompileTime(),
          home: const Text('own graph', textDirection: TextDirection.ltr),
        ),
      );
      await tester.pump();
      expect(find.text('own graph'), findsOneWidget);
    });
  });

  group('the sign-in flow', () {
    testWidgets('splash sends a signed-in customer straight to the tabs', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await pumpApp(tester);
      expect(find.byType(GoklaySplashScreen), findsNothing);
      expect(find.byType(MainShell), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('and everyone else through onboarding to the code', (
      WidgetTester tester,
    ) async {
      backend.routes['POST /v1/auth/otp/request'] = (_) => <String, Object?>{
        'expires_in': 300,
        'resend_after': 2,
      };
      backend.routes['POST /v1/auth/otp/verify'] = (_) => tokenPairJson();
      final Dependencies dependencies = await pumpApp(
        tester,
        signedIn: false,
      );

      await tester.tap(find.bySemanticsLabel(bn.skip));
      await tester.pumpAndSettle();
      expect(find.byType(SignInOptionsScreen), findsOneWidget);

      await tester.tap(find.bySemanticsLabel(bn.continueWithPhone));
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField), '01712345678');
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(core.sendCode));
      await tester.pumpAndSettle();

      await tester.enterText(find.byType(TextField), '123456');
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(core.verifyCode));
      await tester.pumpAndSettle();
      expect(find.text(core.verifiedTitle), findsOneWidget);

      await tester.tap(find.bySemanticsLabel(core.getStarted));
      await tester.pumpAndSettle();
      expect(find.byType(MainShell), findsOneWidget);
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
      final Dependencies dependencies = await pumpApp(
        tester,
        signedIn: false,
      );
      await tester.tap(find.bySemanticsLabel(bn.skip));
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(bn.continueWithPhone));
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField), '01712345678');
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(core.sendCode));
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField), '999999');
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(core.verifyCode));
      await tester.pumpAndSettle();

      expect(find.byType(VerificationResultScreen), findsOneWidget);
      expect(find.text('কোড মেলেনি'), findsOneWidget);
      await tester.tap(find.bySemanticsLabel(core.startOver));
      await tester.pumpAndSettle();
      expect(find.text(core.phoneLabel), findsOneWidget);
      dependencies.dispose();
    });
  });

  group('the tabs', () {
    testWidgets('each one opens its own screen', (WidgetTester tester) async {
      final Dependencies dependencies = await pumpApp(tester);
      expect(find.byType(HomeScreen), findsOneWidget);

      await tester.tap(find.text(bn.cartTitle));
      await tester.pumpAndSettle();
      expect(find.byType(CartScreen), findsOneWidget);

      await tester.tap(find.text(bn.activityTitle));
      await tester.pumpAndSettle();
      expect(find.byType(OrdersScreen), findsOneWidget);

      await tester.tap(find.text(bn.accountTitle));
      await tester.pumpAndSettle();
      expect(find.byType(AccountScreen), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('signing out from the account returns to sign-in', (
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
      expect(find.byType(SignInOptionsScreen), findsOneWidget);
      dependencies.dispose();
    });
  });

  group('the shopping route', () {
    testWidgets('home → shop → item → cart, and the cart tab re-reads', (
      WidgetTester tester,
    ) async {
      backend.routes['GET /v1/catalogue/mer-1/menu'] = (_) => menuJson();
      backend.routes['POST /v1/cart/items'] = (_) => cartJson();
      final Dependencies dependencies = await pumpApp(tester);

      await tester.tap(find.byType(ShopCard));
      await tester.pumpAndSettle();
      expect(find.byType(ShopScreen), findsOneWidget);

      await tester.tap(find.byType(ItemTile));
      await tester.pumpAndSettle();
      expect(find.byType(ItemScreen), findsOneWidget);

      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.addToCart),
      );
      // Adding pops back to the shop, and the cart tab is rebuilt behind it.
      expect(find.byType(ShopScreen), findsOneWidget);
      expect(backend.to('/v1/cart/items'), hasLength(1));

      await goBack(tester);
      await tester.tap(find.text(bn.cartTitle));
      await tester.pumpAndSettle();
      expect(find.byType(CartScreen), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a shop\'s reviews open from its menu', (
      WidgetTester tester,
    ) async {
      backend.routes['GET /v1/catalogue/mer-1/menu'] = (_) => menuJson();
      backend.routes['GET /v1/reviews'] =
          (_) => <String, Object?>{'reviews': <Object?>[reviewJson()]};
      backend.routes['GET /v1/ratings'] = (_) => ratingJson();
      final Dependencies dependencies = await pumpApp(tester);
      await tester.tap(find.byType(ShopCard));
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(bn.reviews));
      await tester.pumpAndSettle();
      expect(find.byType(ShopReviewsScreen), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('cart → review → place order → the order itself', (
      WidgetTester tester,
    ) async {
      backend.routes['PUT /v1/cart/address'] =
          (_) => cartJson(addressId: 'adr-1');
      backend.routes['POST /v1/orders'] = (_) => orderJson();
      backend.routes['GET /v1/orders/ord-1'] = (_) => orderJson();
      backend.routes['GET /v1/orders/ord-1/cancellation'] =
          (_) => cancellationJson();
      final Dependencies dependencies = await pumpApp(tester);

      await tester.tap(find.text(bn.cartTitle));
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(bn.reviewCart));
      await tester.pumpAndSettle();
      expect(find.byType(ReviewCartScreen), findsOneWidget);

      await tester.tap(find.bySemanticsLabel(bn.placeOrder));
      await tester.pumpAndSettle();
      expect(find.byType(PlaceOrderScreen), findsOneWidget);

      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.placeOrder),
      );
      expect(find.byType(OrderScreen), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('an order opens tracking, payment, a review and support', (
      WidgetTester tester,
    ) async {
      backend.routes['GET /v1/orders'] = (_) => <String, Object?>{
        'orders': <Object?>[orderJson(status: 'pending_payment')],
        'total': 1,
      };
      backend.routes['GET /v1/orders/ord-1'] =
          (_) => orderJson(status: 'pending_payment');
      backend.routes['GET /v1/orders/ord-1/cancellation'] =
          (_) => cancellationJson();
      backend.routes['POST /v1/payments/checkout'] = (_) => checkoutJson();
      backend.routes['GET /v1/me/support/tickets'] =
          (_) => <String, Object?>{'tickets': <Object?>[]};
      final Dependencies dependencies = await pumpApp(tester);

      await tester.tap(find.text(bn.activityTitle));
      await tester.pumpAndSettle();
      await tester.tap(find.byType(OrderTile));
      await tester.pumpAndSettle();

      await tester.tap(find.bySemanticsLabel(bn.payNow));
      await tester.pumpAndSettle();
      expect(find.byType(PaymentScreen), findsOneWidget);
      await goBack(tester);

      await tester.tap(find.bySemanticsLabel(bn.getHelp));
      await tester.pumpAndSettle();
      expect(find.byType(SupportScreen), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a live order opens tracking and a finished one a review', (
      WidgetTester tester,
    ) async {
      backend.routes['GET /v1/orders/ord-1'] = (_) => orderJson();
      backend.routes['GET /v1/orders/ord-1/cancellation'] =
          (_) => cancellationJson();
      backend.routes['GET /v1/track/ord-1'] = (_) => null;
      final Dependencies dependencies = await pumpApp(tester);
      await tester.tap(find.text(bn.activityTitle));
      await tester.pumpAndSettle();
      await tester.tap(find.byType(OrderTile));
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(bn.trackOrder));
      await tester.pump();
      await tester.pump();
      expect(find.byType(TrackingScreen), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a delivered order opens the review form', (
      WidgetTester tester,
    ) async {
      backend.routes['GET /v1/orders'] = (_) => <String, Object?>{
        'orders': <Object?>[
          orderJson(status: 'delivered', live: false, partnerId: 'ptn-1'),
        ],
        'total': 1,
      };
      backend.routes['GET /v1/orders/ord-1'] =
          (_) => orderJson(status: 'delivered', live: false);
      backend.routes['GET /v1/orders/ord-1/cancellation'] =
          (_) => cancellationJson(allowed: false);
      final Dependencies dependencies = await pumpApp(tester);
      await tester.tap(find.text(bn.activityTitle));
      await tester.pumpAndSettle();
      await tester.tap(find.byType(OrderTile));
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(bn.leaveReview));
      await tester.pumpAndSettle();
      expect(find.byType(ReviewScreen), findsOneWidget);
      dependencies.dispose();
    });
  });

  group('the account routes', () {
    testWidgets('every row reaches its screen', (WidgetTester tester) async {
      backend.routes['GET /v1/auth/sessions'] =
          (_) => <String, Object?>{'sessions': <Object?>[]};
      backend.routes['GET /v1/me/notifications'] = (_) => <Object?>[];
      backend.routes['GET /v1/me/support/tickets'] =
          (_) => <String, Object?>{'tickets': <Object?>[]};
      final Dependencies dependencies = await pumpApp(tester);
      await tester.tap(find.text(bn.accountTitle));
      await tester.pumpAndSettle();

      Future<void> open(String label, Type screen) async {
        await revealAndTap(tester, find.widgetWithText(SettingRow, label));
        expect(find.byType(screen), findsOneWidget);
        await goBack(tester);
      }

      await open(bn.personalInfo, ProfileScreen);
      await open(bn.savedAddresses, AddressesScreen);
      await open(bn.security, SecurityScreen);
      await open(bn.language, LanguageScreen);
      await open(bn.notifications, NotificationsScreen);
      await open(bn.support, SupportScreen);
      await open(bn.offersTitle, PlaceholderScreenView);
      dependencies.dispose();
    });

    testWidgets('the address book can add one, and cancelling adds none', (
      WidgetTester tester,
    ) async {
      backend.routes['POST /v1/me/addresses'] = (_) => addressJson();
      final Dependencies dependencies = await pumpApp(tester);
      await tester.tap(find.text(bn.accountTitle));
      await tester.pumpAndSettle();
      await revealAndTap(
        tester,
        find.widgetWithText(SettingRow, bn.savedAddresses),
      );
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.addAddress),
      );
      expect(find.byType(AddAddressScreen), findsOneWidget);
      await goBack(tester);
      expect(find.byType(AddressesScreen), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a saved address sends the customer back to the book', (
      WidgetTester tester,
    ) async {
      backend.routes['POST /v1/me/addresses'] = (_) => addressJson();
      final Dependencies dependencies = await pumpApp(tester);
      await tester.tap(find.text(bn.accountTitle));
      await tester.pumpAndSettle();
      await revealAndTap(
        tester,
        find.widgetWithText(SettingRow, bn.savedAddresses),
      );
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.addAddress),
      );
      for (final (String label, String value) in <(String, String)>[
        (bn.addressLabelField, 'বাসা'),
        (bn.recipientName, 'রিয়া'),
        (bn.recipientPhone, '01712345678'),
        (bn.addressLine1, 'রোড ৫'),
        ('lat', '23.7461'),
        ('lng', '90.3742'),
      ]) {
        await tester.enterText(
          find.descendant(
            of: find.widgetWithText(LabelledField, label),
            matching: find.byType(TextField),
          ),
          value,
        );
      }
      await tester.pumpAndSettle();
      await revealAndTap(tester, find.widgetWithText(GoklayButton, bn.save));
      expect(find.byType(AddressesScreen), findsOneWidget);
      expect(find.byType(AddAddressScreen), findsNothing);
      dependencies.dispose();
    });

    testWidgets('home offers the address form when there is none', (
      WidgetTester tester,
    ) async {
      backend.routes['GET /v1/me/addresses'] =
          (_) => <String, Object?>{'addresses': <Object?>[]};
      final Dependencies dependencies = await pumpApp(tester);
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(bn.addAddress));
      await tester.pumpAndSettle();
      expect(find.byType(AddAddressScreen), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('changing the language rebuilds the whole app in it', (
      WidgetTester tester,
    ) async {
      backend.routes['PATCH /v1/me'] = (_) => profileJson();
      final Dependencies dependencies = await pumpApp(tester);
      await tester.tap(find.text(bn.accountTitle));
      await tester.pumpAndSettle();
      await revealAndTap(tester, find.widgetWithText(SettingRow, bn.language));
      await tester.tap(find.widgetWithText(SettingRow, bn.english));
      await tester.pumpAndSettle();
      expect(
        tester.widget<MaterialApp>(find.byType(MaterialApp)).locale,
        const Locale('en'),
      );
      await goBack(tester);
      expect(find.text(const CustomerStringsEn().signOut), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('signing out everywhere also returns to sign-in', (
      WidgetTester tester,
    ) async {
      backend.routes['GET /v1/auth/sessions'] =
          (_) => <String, Object?>{'sessions': <Object?>[]};
      backend.routes['POST /v1/auth/logout-all'] = (_) => null;
      final Dependencies dependencies = await pumpApp(tester);
      await tester.tap(find.text(bn.accountTitle));
      await tester.pumpAndSettle();
      await revealAndTap(tester, find.widgetWithText(SettingRow, bn.security));
      await tester.tap(
        find.widgetWithText(GoklayButton, bn.signOutEverywhere),
      );
      await tester.pumpAndSettle();
      expect(find.byType(SignInOptionsScreen), findsOneWidget);
      dependencies.dispose();
    });
  });

  group('AppScope', () {
    testWidgets('it is found from anywhere below, and notices a swap', (
      WidgetTester tester,
    ) async {
      final Dependencies first = Dependencies(
        environment: CustomerEnvironment(
          apiBaseUrl: Uri.parse('http://api.test'),
        ),
        httpClient: backend.client,
      );
      late Dependencies seen;
      Widget scope(Dependencies dependencies) => AppScope(
        dependencies: dependencies,
        child: Builder(
          builder: (BuildContext context) {
            seen = AppScope.of(context);
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
  });
}
