import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';

Widget _host(Widget child) => MaterialApp(
  theme: GoklayTheme.light(),
  home: Scaffold(body: Center(child: child)),
);

BoxDecoration _decorationOf(WidgetTester tester) {
  final Container container = tester.widget<Container>(
    find.descendant(
      of: find.byType(GoklayButton),
      matching: find.byType(Container),
    ),
  );
  return container.decoration! as BoxDecoration;
}

void main() {
  group('GoklayButton', () {
    testWidgets('paints its label and calls back on tap', (
      WidgetTester tester,
    ) async {
      int taps = 0;
      await tester.pumpWidget(
        _host(GoklayButton(label: 'অর্ডার করুন', onPressed: () => taps++)),
      );

      expect(find.text('অর্ডার করুন'), findsOneWidget);
      await tester.tap(find.byType(GoklayButton));
      expect(taps, 1);
    });

    testWidgets('is at least 48dp tall however small the design draws it', (
      WidgetTester tester,
    ) async {
      // The Figma pill is 12pt of padding around a 14pt label — about 41dp.
      // The painted pill stays that size; the target does not.
      await tester.pumpWidget(
        _host(GoklayButton(label: 'ok', onPressed: () {})),
      );
      expect(
        tester.getSize(find.byType(GoklayButton)).height,
        greaterThanOrEqualTo(GoklayA11y.minTapTarget),
      );
    });

    testWidgets('filled is brand green with no border', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        _host(GoklayButton(label: 'ok', onPressed: () {})),
      );
      final BoxDecoration decoration = _decorationOf(tester);
      expect(decoration.color, GoklayColors.brand);
      expect(decoration.border, isNull);
      expect(
        decoration.borderRadius,
        const BorderRadius.all(GoklayRadii.pill),
      );
    });

    testWidgets('outlined is a hairline with no fill', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        _host(
          GoklayButton(
            label: 'ok',
            onPressed: () {},
            variant: GoklayButtonVariant.outlined,
          ),
        ),
      );
      final BoxDecoration decoration = _decorationOf(tester);
      expect(decoration.color, isNull);
      expect(decoration.border, isNotNull);
      expect(decoration.border!.top.color, GoklayColors.brand);
      expect(decoration.border!.top.width, 0.5);
    });

    testWidgets('the label takes the colour that reads on its background', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        _host(GoklayButton(label: 'ok', onPressed: () {})),
      );
      expect(
        tester.widget<Text>(find.text('ok')).style?.color,
        GoklayColors.onBrand,
      );

      await tester.pumpWidget(
        _host(
          GoklayButton(
            label: 'ok',
            onPressed: () {},
            variant: GoklayButtonVariant.outlined,
          ),
        ),
      );
      expect(
        tester.widget<Text>(find.text('ok')).style?.color,
        GoklayColors.brand,
      );
    });

    testWidgets('a disabled button dims and does not fire', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        _host(const GoklayButton(label: 'ok', onPressed: null)),
      );

      final GoklayButton button = tester.widget<GoklayButton>(
        find.byType(GoklayButton),
      );
      expect(button.isEnabled, isFalse);
      expect(
        tester.widget<Opacity>(find.byType(Opacity)).opacity,
        lessThan(1.0),
      );

      // Tapping must not throw and must not do anything.
      await tester.tap(find.byType(GoklayButton));
      await tester.pump();
    });

    testWidgets('expand stretches to the parent, hugging otherwise', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        _host(
          SizedBox(
            width: 300,
            child: GoklayButton(
              label: 'ok',
              onPressed: () {},
              expand: true,
            ),
          ),
        ),
      );
      expect(tester.getSize(find.byType(GoklayButton)).width, 300);

      // Without expand, and without a parent forcing a width, it hugs its
      // label instead of filling the screen.
      await tester.pumpWidget(
        _host(GoklayButton(label: 'ok', onPressed: () {})),
      );
      expect(tester.getSize(find.byType(GoklayButton)).width, lessThan(300));
    });

    testWidgets('a screen reader hears one button, not a button and a word', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        _host(GoklayButton(label: 'অর্ডার করুন', onPressed: () {})),
      );
      expect(find.bySemanticsLabel('অর্ডার করুন'), findsOneWidget);
    });
  });

  group('GoklayMoneyText', () {
    const Money money = Money(minor: 25000, currency: 'BDT', display: '৳ ২৫০');

    testWidgets('paints the string the server formatted, untouched', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(_host(const GoklayMoneyText(money)));
      expect(find.text('৳ ২৫০'), findsOneWidget);
      // Never the raw integer.
      expect(find.text('25000'), findsNothing);
    });

    testWidgets('defaults to the emphasis style and accepts another', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(_host(const GoklayMoneyText(money)));
      expect(
        tester.widget<Text>(find.text('৳ ২৫০')).style,
        GoklayTextStyles.emphasis,
      );

      await tester.pumpWidget(
        _host(GoklayMoneyText(money, style: GoklayTextStyles.caption)),
      );
      expect(
        tester.widget<Text>(find.text('৳ ২৫০')).style,
        GoklayTextStyles.caption,
      );
    });
  });
}
