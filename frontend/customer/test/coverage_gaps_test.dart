import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_customer/src/api/models/account.dart';
import 'package:goklay_customer/src/api/models/cart.dart';
import 'package:goklay_customer/src/dependencies.dart';
import 'package:goklay_customer/src/environment.dart';
import 'package:goklay_customer/src/l10n/customer_strings.dart';
import 'package:goklay_customer/src/screens/account_screen.dart';
import 'package:goklay_customer/src/screens/add_address_screen.dart';
import 'package:goklay_customer/src/screens/addresses_screen.dart';
import 'package:goklay_customer/src/screens/home_screen.dart';
import 'package:goklay_customer/src/screens/orders_screen.dart';
import 'package:goklay_customer/src/screens/place_order_screen.dart';
import 'package:goklay_customer/src/screens/review_cart_screen.dart';
import 'package:goklay_customer/src/screens/tracking_screen.dart';
import 'package:goklay_customer/src/widgets/messages.dart';
import 'package:goklay_customer/src/widgets/quantity_stepper.dart';
import 'package:goklay_customer/src/widgets/scaffold.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

import 'support/fixtures.dart';
import 'support/harness.dart';

/// The paths the main suites leave untried: the second of a pair of buttons,
/// the chip nobody tapped, the fallback inside a fallback. They are small, and
/// each one is a control a customer really does use — which is why they are
/// tested rather than excluded.
void main() {
  const CustomerStringsBn bn = CustomerStringsBn();
  late FakeBackend backend;

  setUp(() => backend = FakeBackend(<String, Object? Function(SentRequest)>{}));

  void route(String key, Object? Function(SentRequest) handler) =>
      backend.routes[key] = handler;

  testWidgets('the stepper really does step down', (
    WidgetTester tester,
  ) async {
    final Dependencies dependencies = await harnessDependencies(backend);
    final List<int> changes = <int>[];
    await pumpScreen(
      tester,
      Scaffold(
        body: QuantityStepper(quantity: 3, onChanged: changes.add),
      ),
      dependencies: dependencies,
    );
    await tester.tap(find.bySemanticsLabel(bn.decreaseQuantity));
    await tester.pumpAndSettle();
    expect(changes, <int>[2]);
    dependencies.dispose();
  });

  testWidgets('every type chip filters, not just the two the flow uses', (
    WidgetTester tester,
  ) async {
    route('GET /v1/me/addresses',
        (_) => <String, Object?>{'addresses': <Object?>[addressJson()]});
    route('GET /v1/discovery/merchants', (_) => searchJson());
    final Dependencies dependencies = await harnessDependencies(backend);
    await pumpScreen(
      tester,
      HomeScreen(onShopSelected: (_) {}, onAddAddress: () {}),
      dependencies: dependencies,
    );
    await tester.pumpAndSettle();
    for (final (String label, String type) in <(String, String)>[
      (bn.typeRestaurant, 'restaurant'),
      (bn.typeGrocery, 'grocery'),
    ]) {
      await tester.tap(find.bySemanticsLabel(label));
      await tester.pumpAndSettle();
      expect(
        backend.to('/v1/discovery/merchants').last.url.queryParameters['type'],
        type,
      );
    }
    dependencies.dispose();
  });

  testWidgets('the live tab can be chosen again after the past one', (
    WidgetTester tester,
  ) async {
    route('GET /v1/orders',
        (_) => <String, Object?>{'orders': <Object?>[], 'total': 0});
    final Dependencies dependencies = await harnessDependencies(backend);
    await pumpScreen(
      tester,
      OrdersScreen(onOrderSelected: (_) {}),
      dependencies: dependencies,
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text(bn.ordersPast));
    await tester.pumpAndSettle();
    await tester.tap(find.text(bn.ordersLive));
    await tester.pumpAndSettle();
    expect(
      backend.to('/v1/orders').last.url.queryParameters['live'],
      'true',
    );
    dependencies.dispose();
  });

  testWidgets('cash can be chosen back after online', (
    WidgetTester tester,
  ) async {
    route('POST /v1/orders', (_) => orderJson());
    final Dependencies dependencies = await harnessDependencies(backend);
    await pumpScreen(
      tester,
      PlaceOrderScreen(
        cart: Cart.fromJson(cartJson(addressId: 'adr-1')),
        address: Address.fromJson(addressJson()),
        onPlaced: (_) {},
      ),
      dependencies: dependencies,
    );
    await tester.tap(find.text(bn.payOnline));
    await tester.pumpAndSettle();
    await tester.tap(find.text(bn.payCash));
    await tester.pumpAndSettle();
    await revealAndTap(
      tester,
      find.widgetWithText(GoklayButton, bn.placeOrder),
    );
    expect(
      (backend.to('/v1/orders').single.body! as Map<String, Object?>)[
          'payment_method'],
      'cash',
    );
    dependencies.dispose();
  });

  testWidgets(
    'an unbound cart falls through to the default, not the first address',
    (WidgetTester tester) async {
      route('GET /v1/me/addresses', (_) => <String, Object?>{
        'addresses': <Object?>[
          addressJson(id: 'adr-1', label: 'অফিস', isDefault: false),
          addressJson(id: 'adr-2', isDefault: true),
        ],
      });
      route('PUT /v1/cart/address', (_) => cartJson(addressId: 'adr-2'));
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        ReviewCartScreen(
          cart: Cart.fromJson(cartJson()),
          onContinue: (_, _) {},
        ),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      expect(
        (backend.to('/v1/cart/address').single.body!
            as Map<String, Object?>)['address_id'],
        'adr-2',
      );
      dependencies.dispose();
    },
  );

  testWidgets(
    'with no default at all it falls back to the first address',
    (WidgetTester tester) async {
      route('GET /v1/me/addresses', (_) => <String, Object?>{
        'addresses': <Object?>[
          addressJson(id: 'adr-1', label: 'অফিস', isDefault: false),
          addressJson(id: 'adr-2', label: 'বাসা', isDefault: false),
        ],
      });
      route('PUT /v1/cart/address', (_) => cartJson(addressId: 'adr-1'));
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        ReviewCartScreen(
          cart: Cart.fromJson(cartJson()),
          onContinue: (_, _) {},
        ),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      expect(
        (backend.to('/v1/cart/address').single.body!
            as Map<String, Object?>)['address_id'],
        'adr-1',
      );
      dependencies.dispose();
    },
  );

  testWidgets('a stream failure that is not an ApiError still shows', (
    WidgetTester tester,
  ) async {
    // The transport turns network faults into ApiError, but the body stream
    // itself can fail mid-frame — a reset connection under a proxy — and that
    // error arrives as whatever the socket threw. The screen still has to say
    // something rather than spin forever.
    final http.Client breaking = MockClient.streaming((
      http.BaseRequest _,
      http.ByteStream _,
    ) async => http.StreamedResponse(
      () async* {
        yield utf8.encode('data: {"order_id":"ord-1","live":true}\n\n');
        throw StateError('connection reset');
      }(),
      200,
    ));
    final Dependencies dependencies = Dependencies(
      environment: CustomerEnvironment(
        apiBaseUrl: Uri.parse('http://api.test'),
      ),
      httpClient: breaking,
    );
    await pumpScreen(
      tester,
      const TrackingScreen(orderId: 'ord-1'),
      dependencies: dependencies,
    );
    await tester.runAsync(
      () => Future<void>.delayed(const Duration(milliseconds: 20)),
    );
    await tester.pump();
    expect(find.byType(ErrorView), findsOneWidget);
    dependencies.dispose();
  });

  testWidgets('the account rows for the gap screens each open one', (
    WidgetTester tester,
  ) async {
    route('GET /v1/me', (_) => profileJson());
    final Dependencies dependencies = await harnessDependencies(backend);
    final List<PlaceholderScreen> opened = <PlaceholderScreen>[];
    await pumpScreen(
      tester,
      AccountScreen(
        onProfile: () {},
        onAddresses: () {},
        onSecurity: () {},
        onLanguage: () {},
        onNotifications: () {},
        onSupport: () {},
        onPlaceholder: opened.add,
        onSignedOut: () {},
      ),
      dependencies: dependencies,
    );
    await tester.pumpAndSettle();
    for (final String label in <String>[
      bn.promosTitle,
      bn.referralTitle,
      bn.paymentMethodsTitle,
      bn.safetyTitle,
      bn.permissionsTitle,
    ]) {
      await revealAndTap(tester, find.widgetWithText(SettingRow, label));
    }
    expect(opened, <PlaceholderScreen>[
      PlaceholderScreen.promos,
      PlaceholderScreen.referral,
      PlaceholderScreen.paymentMethods,
      PlaceholderScreen.safety,
      PlaceholderScreen.permissions,
    ]);
    dependencies.dispose();
  });

  testWidgets('a saved address answers the address book that it was', (
    WidgetTester tester,
  ) async {
    route('GET /v1/me/addresses',
        (_) => <String, Object?>{'addresses': <Object?>[]});
    route('POST /v1/me/addresses', (_) => addressJson());
    final Dependencies dependencies = await harnessDependencies(backend);
    await pumpScreen(
      tester,
      Builder(
        builder: (BuildContext context) => AddressesScreen(
          onAdd: () async {
            final bool? saved = await Navigator.of(context).push<bool>(
              MaterialPageRoute<bool>(
                builder: (BuildContext inner) => AddAddressScreen(
                  onSaved: (Address _) => Navigator.of(inner).pop(true),
                ),
              ),
            );
            return saved ?? false;
          },
        ),
      ),
      dependencies: dependencies,
    );
    await tester.pumpAndSettle();
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
    expect(backend.to('/v1/me/addresses'), hasLength(3));
    dependencies.dispose();
  });
}
