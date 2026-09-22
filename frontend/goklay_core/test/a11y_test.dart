import 'dart:ui' show Tristate;

import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';

void main() {
  group('contrast maths', () {
    test('identical colours have no contrast, black on white has the most', () {
      expect(
        GoklayContrast.ratio(GoklayPalette.white, GoklayPalette.white),
        closeTo(1.0, 0.001),
      );
      expect(
        GoklayContrast.ratio(const Color(0xFF000000), GoklayPalette.white),
        closeTo(21.0, 0.01),
      );
    });

    test('the ratio does not depend on which colour is named first', () {
      final double forwards = GoklayContrast.ratio(
        GoklayColors.brand,
        GoklayColors.surfaceRaised,
      );
      final double backwards = GoklayContrast.ratio(
        GoklayColors.surfaceRaised,
        GoklayColors.brand,
      );
      expect(forwards, closeTo(backwards, 0.000001));
    });

    test('meetsAA applies the larger threshold to small text', () {
      // A pair that lands between the two thresholds proves they are actually
      // different: legal for a heading, not for a caption.
      const Color mid = Color(0xFF949494);
      final double ratio = GoklayContrast.ratio(mid, GoklayPalette.white);
      expect(ratio, greaterThan(GoklayA11y.minContrastLarge));
      expect(ratio, lessThan(GoklayA11y.minContrastBody));

      expect(GoklayContrast.meetsAA(mid, GoklayPalette.white), isFalse);
      expect(
        GoklayContrast.meetsAA(mid, GoklayPalette.white, large: true),
        isTrue,
      );
    });
  });

  group('every colour pair this library paints clears WCAG AA', () {
    // These are not every combination the palette could produce — they are the
    // ones the theme and the widgets in this package actually render. A pair
    // nothing paints is not a promise worth keeping.
    const List<(String, Color, Color)> pairs = <(String, Color, Color)>[
      ('body text on the page', GoklayColors.textPrimary, GoklayColors.surface),
      (
        'body text on a card',
        GoklayColors.textPrimary,
        GoklayColors.surfaceRaised,
      ),
      (
        'caption on the page',
        GoklayColors.textSecondary,
        GoklayColors.surface,
      ),
      (
        'caption on a card',
        GoklayColors.textSecondary,
        GoklayColors.surfaceRaised,
      ),
      (
        'label on a filled button',
        GoklayColors.onBrand,
        GoklayColors.brand,
      ),
      (
        'label on an outlined button',
        GoklayColors.brand,
        GoklayColors.surfaceRaised,
      ),
    ];

    for (final (String what, Color fg, Color bg) in pairs) {
      test(what, () {
        final double ratio = GoklayContrast.ratio(fg, bg);
        expect(
          ratio,
          greaterThanOrEqualTo(GoklayA11y.minContrastBody),
          reason:
              '$what is $ratio:1, under the ${GoklayA11y.minContrastBody}:1 '
              'WCAG AA floor for text',
        );
      });
    }
  });

  group('the combinations the design uses that small text cannot', () {
    // Recorded as tests rather than as a comment nobody reads. Each of these
    // is a real pairing from the Figma file that does not clear AA at the size
    // the file draws it. Asserting the limit here is what stops a future
    // screen quietly repeating it.

    test('brand green on its own tint is a large-text pairing only', () {
      // The design sets a promo code in Poppins SemiBold 16 in brand green on
      // a brand-15% chip (Figma node 1:290). That is 3.73:1, and WCAG only
      // relaxes to 3:1 at 18.66px for bold — 16px does not reach it. Paint the
      // code in textPrimary on this chip, or in brand on plain white.
      expect(
        GoklayContrast.meetsAA(
          GoklayColors.brand,
          GoklayColors.brandSubtle,
        ),
        isFalse,
      );
      expect(
        GoklayContrast.meetsAA(
          GoklayColors.brand,
          GoklayColors.brandSubtle,
          large: true,
        ),
        isTrue,
      );
      // The way out, for when the code has to stay at 16px.
      expect(
        GoklayContrast.meetsAA(
          GoklayColors.textPrimary,
          GoklayColors.brandSubtle,
        ),
        isTrue,
      );
    });
    test('Soft Red is a status colour, legal only at large sizes', () {
      expect(
        GoklayContrast.meetsAA(
          GoklayColors.danger,
          GoklayColors.surfaceRaised,
        ),
        isFalse,
        reason: 'if this now passes, the palette changed — revisit the rule',
      );
      expect(
        GoklayContrast.meetsAA(
          GoklayColors.danger,
          GoklayColors.surfaceRaised,
          large: true,
        ),
        isTrue,
      );
    });

    test('Emerald is a fill, never text', () {
      expect(
        GoklayContrast.meetsAA(
          GoklayColors.success,
          GoklayColors.surfaceRaised,
          large: true,
        ),
        isFalse,
        reason:
            'Emerald does not clear AA at any text size on white. Use it as a '
            'fill behind onBrand text, or as a dot beside a label.',
      );
    });
  });

  group('tap targets', () {
    testWidgets('a tiny child still gets a 48dp hit area', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        const Directionality(
          textDirection: TextDirection.ltr,
          child: Center(
            child: GoklayTapTarget(
              child: SizedBox(width: 8, height: 8),
            ),
          ),
        ),
      );

      final Size size = tester.getSize(find.byType(GoklayTapTarget));
      expect(size.width, greaterThanOrEqualTo(GoklayA11y.minTapTarget));
      expect(size.height, greaterThanOrEqualTo(GoklayA11y.minTapTarget));
    });

    testWidgets('the child is painted at its own size, not stretched', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        const Directionality(
          textDirection: TextDirection.ltr,
          child: Center(
            child: GoklayTapTarget(
              child: SizedBox(width: 8, height: 8, child: Placeholder()),
            ),
          ),
        ),
      );

      expect(
        tester.getSize(find.byType(Placeholder)),
        const Size(8, 8),
      );
    });

    testWidgets('a large child is not shrunk to the floor', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        const Directionality(
          textDirection: TextDirection.ltr,
          child: Center(
            child: GoklayTapTarget(
              child: SizedBox(width: 200, height: 60),
            ),
          ),
        ),
      );

      expect(
        tester.getSize(find.byType(GoklayTapTarget)),
        const Size(200, 60),
      );
    });

    testWidgets('taps reach the callback', (WidgetTester tester) async {
      int taps = 0;
      await tester.pumpWidget(
        Directionality(
          textDirection: TextDirection.ltr,
          child: Center(
            child: GoklayTapTarget(
              onTap: () => taps++,
              child: const SizedBox(width: 8, height: 8),
            ),
          ),
        ),
      );

      // Tapping the padding, not the child, still counts: that padding is the
      // whole point of the widget.
      await tester.tapAt(
        tester.getTopLeft(find.byType(GoklayTapTarget)) +
            const Offset(2, 2),
      );
      expect(taps, 1);
    });

    testWidgets('a null callback reads as a disabled button', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        const Directionality(
          textDirection: TextDirection.ltr,
          child: Center(
            child: GoklayTapTarget(
              semanticLabel: 'Place order',
              excludeChildSemantics: true,
              child: SizedBox(width: 8, height: 8),
            ),
          ),
        ),
      );

      final SemanticsNode node = tester.getSemantics(
        find.byType(GoklayTapTarget),
      );
      expect(node.label, 'Place order');
      expect(node.flagsCollection.isButton, isTrue);
      // isEnabled is tri-state, not a bool: a node can be enabled, disabled,
      // or have no notion of enabledness at all. A disabled control has to be
      // the middle one — "not applicable" would tell a screen reader nothing.
      expect(node.flagsCollection.isEnabled, Tristate.isFalse);
    });
  });
}
