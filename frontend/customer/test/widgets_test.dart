import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_customer/src/api/models/cart.dart';
import 'package:goklay_customer/src/api/models/catalogue.dart';
import 'package:goklay_customer/src/api/models/discovery.dart';
import 'package:goklay_customer/src/api/models/order.dart';
import 'package:goklay_customer/src/dependencies.dart';
import 'package:goklay_customer/src/l10n/customer_strings.dart';
import 'package:goklay_customer/src/widgets/cards.dart';

import 'support/fixtures.dart';
import 'support/harness.dart';

void main() {
  const CustomerStringsBn bn = CustomerStringsBn();
  const GoklayStringsBn core = GoklayStringsBn();
  late FakeBackend backend;
  late Dependencies dependencies;

  setUp(() async {
    backend = FakeBackend(<String, Object? Function(SentRequest)>{});
    dependencies = await harnessDependencies(backend);
  });

  tearDown(() => dependencies.dispose());

  Future<void> pump(WidgetTester tester, Widget child) =>
      pumpScreen(tester, Scaffold(body: child), dependencies: dependencies);

  group('cards', () {
    testWidgets('a shop card paints the server\'s strings, not numbers', (
      WidgetTester tester,
    ) async {
      bool tapped = false;
      await pump(
        tester,
        ShopCard(
          merchant: DiscoveryMerchant.fromJson(merchantJson()),
          onTap: () => tapped = true,
        ),
      );
      expect(find.text('১.২ কিমি'), findsOneWidget);
      expect(find.text('খোলা আছে'), findsOneWidget);
      expect(find.text('৳ ৬০'), findsOneWidget);
      await tester.tap(find.byType(ShopCard));
      await tester.pumpAndSettle();
      expect(tapped, isTrue);
    });

    testWidgets('an unorderable item cannot be opened', (
      WidgetTester tester,
    ) async {
      bool tapped = false;
      await pump(
        tester,
        ItemTile(
          item: PublicItem.fromJson(itemJson(orderable: false)),
          onTap: () => tapped = true,
        ),
      );
      await tester.tap(find.text('কাচ্চি'));
      await tester.pumpAndSettle();
      expect(tapped, isFalse);
    });

    testWidgets('an orderable item can', (WidgetTester tester) async {
      bool tapped = false;
      await pump(
        tester,
        ItemTile(
          item: PublicItem.fromJson(itemJson()),
          onTap: () => tapped = true,
        ),
      );
      await tester.tap(find.byType(ItemTile));
      await tester.pumpAndSettle();
      expect(tapped, isTrue);
    });

    testWidgets('a cart line shows its options, note and issue', (
      WidgetTester tester,
    ) async {
      final Cart cart = Cart.fromJson(cartJson(lineHasIssue: true));
      final List<int> quantities = <int>[];
      bool removed = false;
      await pump(
        tester,
        CartLineTile(
          line: cart.lines.single,
          onQuantityChanged: quantities.add,
          onRemove: () => removed = true,
        ),
      );
      expect(find.text('বড়'), findsOneWidget);
      expect(find.text('ঝাল কম'), findsOneWidget);
      expect(find.text('স্টক শেষ'), findsOneWidget);
      await tester.tap(find.bySemanticsLabel(core.increaseQuantity));
      await tester.tap(find.bySemanticsLabel(bn.removeLine));
      await tester.pumpAndSettle();
      expect(quantities, <int>[2]);
      expect(removed, isTrue);
    });

    testWidgets('a disabled cart line ignores both controls', (
      WidgetTester tester,
    ) async {
      final Cart cart = Cart.fromJson(cartJson());
      bool touched = false;
      await pump(
        tester,
        CartLineTile(
          line: cart.lines.single,
          enabled: false,
          onQuantityChanged: (_) => touched = true,
          onRemove: () => touched = true,
        ),
      );
      await tester.tap(find.bySemanticsLabel(bn.removeLine));
      await tester.pumpAndSettle();
      expect(touched, isFalse);
    });

    testWidgets('an order tile shows the code and the status sentence', (
      WidgetTester tester,
    ) async {
      bool tapped = false;
      await pump(
        tester,
        OrderTile(
          order: Order.fromJson(orderJson()),
          onTap: () => tapped = true,
        ),
      );
      expect(find.text('GK-7F3K'), findsOneWidget);
      expect(find.text('রান্না হচ্ছে'), findsOneWidget);
      await tester.tap(find.byType(OrderTile));
      await tester.pumpAndSettle();
      expect(tapped, isTrue);
    });

    testWidgets('a card with no tap handler is inert', (
      WidgetTester tester,
    ) async {
      await pump(tester, const GoklayCard(child: Text('flat')));
      expect(find.byType(GoklayTapTarget), findsNothing);
    });
  });

}
