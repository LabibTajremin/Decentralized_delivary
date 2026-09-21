import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';

void main() {
  group('the theme is built from the tokens and nothing else', () {
    test('the colour scheme names the palette explicitly', () {
      final ThemeData theme = GoklayTheme.light();
      expect(theme.colorScheme.primary, GoklayColors.brand);
      expect(theme.colorScheme.onPrimary, GoklayColors.onBrand);
      expect(theme.colorScheme.error, GoklayColors.danger);
      expect(theme.colorScheme.surface, GoklayColors.surfaceRaised);
      expect(theme.colorScheme.onSurface, GoklayColors.textPrimary);
      expect(theme.colorScheme.brightness, Brightness.light);
      expect(theme.scaffoldBackgroundColor, GoklayColors.surface);
    });

    test('a seeded scheme would have invented colours; this one has not', () {
      // ColorScheme.fromSeed(seedColor: brand) produces a tonal palette whose
      // primary is *not* the brand colour. Proving that here is what keeps a
      // future refactor from reaching for the convenient constructor.
      final ColorScheme seeded = ColorScheme.fromSeed(
        seedColor: GoklayColors.brand,
      );
      expect(seeded.primary, isNot(GoklayColors.brand));
      expect(GoklayTheme.light().colorScheme.primary, GoklayColors.brand);
    });

    test('the default font is Poppins with the Bengali fallback', () {
      final ThemeData theme = GoklayTheme.light();
      expect(theme.textTheme.bodyMedium?.fontFamily, isNotNull);
      expect(
        theme.textTheme.bodyMedium?.fontFamilyFallback,
        contains(GoklayFonts.qualified(GoklayFonts.bengali)),
      );
    });

    test('Material widgets get a 48dp target of their own', () {
      expect(
        GoklayTheme.light().materialTapTargetSize,
        MaterialTapTargetSize.padded,
      );
    });

    test('the app bar sits flat on the surface', () {
      final AppBarThemeData bar = GoklayTheme.light().appBarTheme;
      expect(bar.backgroundColor, GoklayColors.surfaceRaised);
      expect(bar.foregroundColor, GoklayColors.textPrimary);
      expect(bar.elevation, 0);
      expect(bar.scrolledUnderElevation, 0);
    });

    test('dividers are the hairline the design draws', () {
      expect(GoklayTheme.light().dividerTheme.thickness, 0.5);
      expect(GoklayTheme.light().dividerTheme.color, GoklayColors.border);
    });
  });

  group('the Material text slots carry the designed scale', () {
    test('each slot maps to a style from the scale', () {
      final TextTheme text = GoklayTheme.textTheme;
      expect(text.displayLarge, GoklayTextStyles.display);
      expect(text.headlineLarge, GoklayTextStyles.display);
      expect(text.titleLarge, GoklayTextStyles.emphasis);
      expect(text.titleMedium, GoklayTextStyles.title);
      expect(text.bodyLarge, GoklayTextStyles.body);
      expect(text.bodyMedium, GoklayTextStyles.body);
      expect(text.bodySmall, GoklayTextStyles.caption);
      expect(text.labelLarge, GoklayTextStyles.label);
      expect(text.labelSmall, GoklayTextStyles.caption);
    });

    testWidgets('a stock widget picks the designed face up from the theme', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        MaterialApp(
          theme: GoklayTheme.light(),
          home: const Scaffold(body: Text('বাংলা')),
        ),
      );

      final TextStyle? style = tester
          .widget<RichText>(find.byType(RichText))
          .text
          .style;
      expect(
        style?.fontFamilyFallback,
        contains(GoklayFonts.qualified(GoklayFonts.bengali)),
      );
    });
  });
}
