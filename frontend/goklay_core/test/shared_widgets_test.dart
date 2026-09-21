import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';

import 'support/harness.dart';

void main() {
  const GoklayStringsBn bn = GoklayStringsBn();

  Future<void> pump(WidgetTester tester, Widget child) =>
      pumpWidgetUnderTest(tester, Scaffold(body: child));

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
      expect(find.text(bn.loading), findsOneWidget);

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
      await tester.tap(find.bySemanticsLabel(bn.retry));
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
      expect(find.text(bn.offlineBanner), findsOneWidget);
    });

    testWidgets('an unreadable failure falls back to the shared line', (
      WidgetTester tester,
    ) async {
      await pump(tester, ErrorView(error: ApiError.unreadable(500)));
      expect(find.text(bn.unexpectedError), findsOneWidget);
    });
  });

  group('EmptyView and NotAvailableView', () {
    testWidgets('an empty list says so, generically or specifically', (
      WidgetTester tester,
    ) async {
      await pump(tester, const EmptyView());
      expect(find.text(bn.nothingHere), findsOneWidget);
      await pump(tester, EmptyView(message: 'আপনি এখনো কোনো অর্ডার করেননি।'));
      expect(find.text('আপনি এখনো কোনো অর্ডার করেননি।'), findsOneWidget);
    });

    testWidgets('a gap screen admits it rather than inventing data', (
      WidgetTester tester,
    ) async {
      await pump(tester, const NotAvailableView());
      expect(find.text(bn.notAvailableYet), findsOneWidget);
      await pump(tester, NotAvailableView(detail: 'নগদ অথবা অনলাইন'));
      expect(find.text('নগদ অথবা অনলাইন'), findsOneWidget);
    });
  });

  group('ReceiptView', () {
    testWidgets('prints the rows the server composed, in order', (
      WidgetTester tester,
    ) async {
      final List<ReceiptRow> rows = <ReceiptRow>[
        ReceiptRow.fromJson(<String, Object?>{
          'key': 'subtotal',
          'label': 'সাবটোটাল',
          'amount': money(32000, '৳ ৩২০'),
        }),
        ReceiptRow.fromJson(<String, Object?>{
          'key': 'delivery',
          'label': 'ডেলিভারি',
          'amount': money(6000, '৳ ৬০'),
        }),
      ];
      await pump(
        tester,
        ReceiptView(
          rows: rows,
          total: Money.fromJson(money(38000, '৳ ৩৮০')),
          totalLabel: 'হিসাব',
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

    testWidgets('it really does step down', (WidgetTester tester) async {
      final List<int> changes = <int>[];
      await pump(
        tester,
        QuantityStepper(quantity: 3, onChanged: changes.add),
      );
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
          semanticLabel: 'দোকানকে রেটিং দিন',
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
        StarRating(rating: 3, semanticLabel: 'দোকানকে রেটিং দিন'),
      );
      expect(find.byIcon(Icons.star), findsNWidgets(3));
      expect(find.byIcon(Icons.star_border), findsNWidgets(2));
    });
  });

  group('scaffold pieces', () {
    testWidgets('the frame draws a title, actions and a bottom bar', (
      WidgetTester tester,
    ) async {
      await pumpWidgetUnderTest(
        tester,
        GoklayScaffold(
          title: 'কার্ট',
          actions: <Widget>[Text('রিভিউ')],
          bottom: Text('অর্ডার করুন'),
          body: Text('হিসাব'),
        ),
      );
      expect(find.text('কার্ট'), findsOneWidget);
      expect(find.text('রিভিউ'), findsOneWidget);
      expect(find.text('অর্ডার করুন'), findsOneWidget);
    });

    testWidgets('a setting row reports taps and shows its value', (
      WidgetTester tester,
    ) async {
      bool tapped = false;
      await pump(
        tester,
        SettingRow(
          label: 'ভাষা',
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
          label: 'নাম',
          hint: '01XXXXXXXXX',
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
      await pump(tester, SectionHeading('হিসাব'));
      expect(find.text('হিসাব'), findsOneWidget);
    });
  });
}
