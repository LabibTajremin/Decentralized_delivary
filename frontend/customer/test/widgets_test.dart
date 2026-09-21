import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_customer/src/api/models/cart.dart';
import 'package:goklay_customer/src/api/models/catalogue.dart';
import 'package:goklay_customer/src/api/models/discovery.dart';
import 'package:goklay_customer/src/api/models/order.dart';
import 'package:goklay_customer/src/dependencies.dart';
import 'package:goklay_customer/src/l10n/customer_strings.dart';
import 'package:goklay_customer/src/state/async_value.dart';
import 'package:goklay_customer/src/state/store.dart';
import 'package:goklay_customer/src/widgets/async_view.dart';
import 'package:goklay_customer/src/widgets/cards.dart';
import 'package:goklay_customer/src/widgets/messages.dart';
import 'package:goklay_customer/src/widgets/quantity_stepper.dart';
import 'package:goklay_customer/src/widgets/receipt_view.dart';
import 'package:goklay_customer/src/widgets/scaffold.dart';
import 'package:goklay_customer/src/widgets/star_rating.dart';

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

  group('AsyncView', () {
    testWidgets('paints the spinner, then the value', (
      WidgetTester tester,
    ) async {
      final Store<String> store = Store<String>();
      await pump(
        tester,
        AsyncView<String>(
          store: store,
          builder: (_, String value) => Text(value),
        ),
      );
      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      expect(find.text(core.loading), findsOneWidget);

      store.emit(const AsyncData<String>('ready'));
      await tester.pumpAndSettle();
      expect(find.text('ready'), findsOneWidget);
      store.dispose();
    });

    testWidgets('a cached value is labelled as saved', (
      WidgetTester tester,
    ) async {
      final Store<String> store = Store<String>()
        ..emit(const AsyncData<String>('ready', fromCache: true));
      await pump(
        tester,
        AsyncView<String>(
          store: store,
          builder: (_, String value) => Text(value),
        ),
      );
      expect(find.text(bn.showingSaved), findsOneWidget);
      expect(find.byType(CachedBanner), findsOneWidget);
      store.dispose();
    });

    testWidgets('a failure shows the server\'s sentence and retries', (
      WidgetTester tester,
    ) async {
      final Store<String> store = Store<String>()
        ..emit(
          const AsyncFailure<String>(
            ApiError(
              statusCode: 503,
              code: 'config_unavailable',
              message: 'পরে দেখুন',
            ),
          ),
        );
      int retries = 0;
      await pump(
        tester,
        AsyncView<String>(
          store: store,
          onRetry: () async => retries += 1,
          builder: (_, String value) => Text(value),
        ),
      );
      expect(find.text('পরে দেখুন'), findsOneWidget);
      await tester.tap(find.bySemanticsLabel(core.retry));
      await tester.pumpAndSettle();
      expect(retries, 1);
      store.dispose();
    });

    testWidgets('without onRetry there is no retry button', (
      WidgetTester tester,
    ) async {
      final Store<String> store = Store<String>()
        ..emit(AsyncFailure<String>(ApiError.offline()));
      await pump(
        tester,
        AsyncView<String>(
          store: store,
          builder: (_, String value) => Text(value),
        ),
      );
      expect(find.byType(GoklayButton), findsNothing);
      store.dispose();
    });
  });

  group('ErrorView', () {
    testWidgets('offline with no message falls back to the offline line', (
      WidgetTester tester,
    ) async {
      await pump(tester, ErrorView(error: ApiError.offline()));
      expect(find.text(core.offlineBanner), findsOneWidget);
    });

    testWidgets('an unreadable failure falls back to the shared line', (
      WidgetTester tester,
    ) async {
      await pump(tester, ErrorView(error: ApiError.unreadable(500)));
      expect(find.text(core.unexpectedError), findsOneWidget);
    });
  });

  group('EmptyView and NotAvailableView', () {
    testWidgets('an empty list says so, generically or specifically', (
      WidgetTester tester,
    ) async {
      await pump(tester, const EmptyView());
      expect(find.text(bn.nothingHere), findsOneWidget);
      await pump(tester, EmptyView(message: bn.noOrders));
      expect(find.text(bn.noOrders), findsOneWidget);
    });

    testWidgets('a gap screen admits it rather than inventing data', (
      WidgetTester tester,
    ) async {
      await pump(tester, const NotAvailableView());
      expect(find.text(bn.notAvailableYet), findsOneWidget);
      await pump(tester, NotAvailableView(detail: bn.paymentMethodsBody));
      expect(find.text(bn.paymentMethodsBody), findsOneWidget);
    });
  });

  group('ReceiptView', () {
    testWidgets('prints the rows the server composed, in order', (
      WidgetTester tester,
    ) async {
      final List<ReceiptRow> rows = <ReceiptRow>[
        for (final Map<String, Object?> row in receiptJson())
          ReceiptRow.fromJson(row),
      ];
      await pump(
        tester,
        ReceiptView(
          rows: rows,
          total: Money.fromJson(money(38000, '৳ ৩৮০')),
          totalLabel: bn.receipt,
        ),
      );
      expect(find.text('সাবটোটাল'), findsOneWidget);
      expect(find.text('৳ ৩২০'), findsOneWidget);
      expect(find.text('৳ ৩৮০'), findsOneWidget);
    });

    testWidgets('with no total there is no rule and no total row', (
      WidgetTester tester,
    ) async {
      await pump(tester, const ReceiptView(rows: <ReceiptRow>[]));
      expect(find.byType(Divider), findsNothing);
    });

    testWidgets('a total with no label still prints', (
      WidgetTester tester,
    ) async {
      await pump(
        tester,
        ReceiptView(
          rows: const <ReceiptRow>[],
          total: Money.fromJson(money(100, '৳ ১')),
        ),
      );
      expect(find.text('৳ ১'), findsOneWidget);
    });
  });

  group('QuantityStepper', () {
    testWidgets('steps up and down, and stops at both ends', (
      WidgetTester tester,
    ) async {
      final List<int> changes = <int>[];
      await pump(
        tester,
        QuantityStepper(
          quantity: 1,
          onChanged: changes.add,
        ),
      );
      await tester.tap(find.bySemanticsLabel(bn.increaseQuantity));
      await tester.tap(find.bySemanticsLabel(bn.decreaseQuantity));
      await tester.pumpAndSettle();
      expect(changes, <int>[2]);
    });

    testWidgets('the cart\'s ceiling of 20 is not offered past', (
      WidgetTester tester,
    ) async {
      final List<int> changes = <int>[];
      await pump(
        tester,
        QuantityStepper(quantity: 20, onChanged: changes.add),
      );
      await tester.tap(find.bySemanticsLabel(bn.increaseQuantity));
      await tester.pumpAndSettle();
      expect(changes, isEmpty);
    });

    testWidgets('a disabled stepper does nothing', (
      WidgetTester tester,
    ) async {
      final List<int> changes = <int>[];
      await pump(
        tester,
        QuantityStepper(quantity: 2, enabled: false, onChanged: changes.add),
      );
      await tester.tap(find.bySemanticsLabel(bn.increaseQuantity));
      await tester.tap(find.bySemanticsLabel(bn.decreaseQuantity));
      await tester.pumpAndSettle();
      expect(changes, isEmpty);
    });

    testWidgets('its buttons clear the 48dp floor', (
      WidgetTester tester,
    ) async {
      await pump(
        tester,
        QuantityStepper(quantity: 2, onChanged: (_) {}),
      );
      final Size size = tester.getSize(
        find.bySemanticsLabel(bn.increaseQuantity),
      );
      expect(size.width, greaterThanOrEqualTo(GoklayA11y.minTapTarget));
      expect(size.height, greaterThanOrEqualTo(GoklayA11y.minTapTarget));
    });
  });

  group('StarRating', () {
    testWidgets('reports the star that was tapped, one-based', (
      WidgetTester tester,
    ) async {
      int? tapped;
      await pump(
        tester,
        StarRating(
          rating: 0,
          semanticLabel: bn.rateShop,
          onChanged: (int star) => tapped = star,
        ),
      );
      await tester.tap(find.bySemanticsLabel('4'));
      await tester.pumpAndSettle();
      expect(tapped, 4);
    });

    testWidgets('read-only stars do not respond', (
      WidgetTester tester,
    ) async {
      await pump(
        tester,
        StarRating(rating: 3, semanticLabel: bn.rateShop),
      );
      expect(find.byIcon(Icons.star), findsNWidgets(3));
      expect(find.byIcon(Icons.star_border), findsNWidgets(2));
    });
  });

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
      await tester.tap(find.bySemanticsLabel(bn.increaseQuantity));
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

  group('scaffold pieces', () {
    testWidgets('the frame draws a title, actions and a bottom bar', (
      WidgetTester tester,
    ) async {
      await pumpScreen(
        tester,
        CustomerScaffold(
          title: bn.cartTitle,
          actions: <Widget>[Text(bn.reviews)],
          bottom: Text(bn.placeOrder),
          body: Text(bn.receipt),
        ),
        dependencies: dependencies,
      );
      expect(find.text(bn.cartTitle), findsOneWidget);
      expect(find.text(bn.reviews), findsOneWidget);
      expect(find.text(bn.placeOrder), findsOneWidget);
    });

    testWidgets('a setting row reports taps and shows its value', (
      WidgetTester tester,
    ) async {
      bool tapped = false;
      await pump(
        tester,
        SettingRow(
          label: bn.language,
          value: 'bn',
          onTap: () => tapped = true,
        ),
      );
      expect(find.text('bn'), findsOneWidget);
      await tester.tap(find.byType(SettingRow));
      await tester.pumpAndSettle();
      expect(tapped, isTrue);
    });

    testWidgets('a labelled field types into its controller', (
      WidgetTester tester,
    ) async {
      final TextEditingController controller = TextEditingController();
      await pump(
        tester,
        LabelledField(
          label: bn.nameLabel,
          hint: bn.phoneHint,
          controller: controller,
        ),
      );
      await tester.enterText(find.byType(TextField), 'রিয়া');
      expect(controller.text, 'রিয়া');
      controller.dispose();
    });

    testWidgets('a section heading prints its text', (
      WidgetTester tester,
    ) async {
      await pump(tester, SectionHeading(bn.receipt));
      expect(find.text(bn.receipt), findsOneWidget);
    });
  });
}
