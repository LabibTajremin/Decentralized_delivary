import 'package:flutter/painting.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';

/// Composites [foreground] at [opacity] over [background], the way a
/// translucent fill actually resolves on screen.
Color _composite(Color foreground, Color background, double opacity) {
  int channel(int f, int b) => (f * opacity + b * (1 - opacity)).round();
  return Color.fromARGB(
    0xFF,
    channel(
      (foreground.r * 255).round(),
      (background.r * 255).round(),
    ),
    channel(
      (foreground.g * 255).round(),
      (background.g * 255).round(),
    ),
    channel(
      (foreground.b * 255).round(),
      (background.b * 255).round(),
    ),
  );
}

void main() {
  group('the palette is the Figma variables, unchanged', () {
    test('every colour matches the value the design file defines', () {
      // Read from the Figma variables on 2026-09-21. If the design changes,
      // this test is the thing that notices.
      expect(GoklayPalette.black, const Color(0xFF0F0F0F));
      expect(GoklayPalette.gray, const Color(0xFF656565));
      expect(GoklayPalette.white, const Color(0xFFFFFFFF));
      expect(GoklayPalette.softGray, const Color(0xFFF8F9FA));
      expect(GoklayPalette.primary, const Color(0xFF22874F));
      expect(GoklayPalette.emerald, const Color(0xFF10B981));
      expect(GoklayPalette.softRed, const Color(0xFFFF5A5F));
    });

    test('the tint is the brand at 15% on white, not a colour someone picked', () {
      expect(
        GoklayPalette.primaryTint,
        _composite(GoklayPalette.primary, GoklayPalette.white, 0.15),
      );
    });

    test('the semantic names point at the palette, not at new colours', () {
      const List<Color> semantic = <Color>[
        GoklayColors.surface,
        GoklayColors.surfaceRaised,
        GoklayColors.textPrimary,
        GoklayColors.textSecondary,
        GoklayColors.onBrand,
        GoklayColors.brand,
        GoklayColors.brandSubtle,
        GoklayColors.success,
        GoklayColors.danger,
        GoklayColors.border,
      ];
      const List<Color> palette = <Color>[
        GoklayPalette.black,
        GoklayPalette.gray,
        GoklayPalette.white,
        GoklayPalette.softGray,
        GoklayPalette.primary,
        GoklayPalette.primaryTint,
        GoklayPalette.emerald,
        GoklayPalette.softRed,
      ];
      for (final Color color in semantic) {
        expect(
          palette,
          contains(color),
          reason: 'a semantic role resolved to a colour outside the palette',
        );
      }
    });
  });

  group('typography', () {
    test('the scale matches the sizes and weights read from Figma', () {
      expect(GoklayTextStyles.display.fontSize, 30);
      expect(GoklayTextStyles.display.fontWeight, FontWeight.w700);

      expect(GoklayTextStyles.title.fontSize, 16);
      expect(GoklayTextStyles.title.fontWeight, FontWeight.w500);

      expect(GoklayTextStyles.emphasis.fontSize, 16);
      expect(GoklayTextStyles.emphasis.fontWeight, FontWeight.w600);

      expect(GoklayTextStyles.body.fontSize, 16);
      expect(GoklayTextStyles.body.fontWeight, FontWeight.w300);
      expect(GoklayTextStyles.body.height, closeTo(20 / 16, 0.0001));

      expect(GoklayTextStyles.caption.fontSize, 12);
      expect(GoklayTextStyles.caption.fontWeight, FontWeight.w400);
      expect(GoklayTextStyles.caption.height, closeTo(15 / 12, 0.0001));

      expect(GoklayTextStyles.label.fontSize, 14);
      expect(GoklayTextStyles.label.fontWeight, FontWeight.w500);
    });

    test('the display face is Manrope and everything else is Poppins', () {
      expect(
        GoklayTextStyles.display.fontFamily,
        GoklayFonts.qualified(GoklayFonts.display),
      );
      for (final TextStyle style in <TextStyle>[
        GoklayTextStyles.title,
        GoklayTextStyles.emphasis,
        GoklayTextStyles.body,
        GoklayTextStyles.caption,
        GoklayTextStyles.label,
      ]) {
        expect(style.fontFamily, GoklayFonts.qualified(GoklayFonts.text));
      }
    });

    test('every style falls back to the Bengali face', () {
      // This is the one that matters. Manrope and Poppins contain no Bengali
      // glyphs at all, and Bengali is the default language — a style that
      // loses this fallback renders the whole app as empty boxes for most of
      // its users.
      for (final TextStyle style in GoklayTextStyles.all) {
        expect(
          style.fontFamilyFallback,
          contains(GoklayFonts.qualified(GoklayFonts.bengali)),
          reason: 'a style with no Bengali fallback will render tofu',
        );
      }
    });

    test('qualified() addresses a font shipped inside this package', () {
      expect(
        GoklayFonts.qualified('Poppins'),
        'packages/goklay_core/Poppins',
      );
    });

    test('all() really is every style in the scale', () {
      expect(GoklayTextStyles.all, hasLength(6));
      expect(GoklayTextStyles.all, contains(GoklayTextStyles.display));
      expect(GoklayTextStyles.all, contains(GoklayTextStyles.caption));
    });
  });

  group('dimensions', () {
    test('the spacing steps are the ones the design actually uses', () {
      expect(GoklaySpacing.xxs, 4);
      expect(GoklaySpacing.xs, 6);
      expect(GoklaySpacing.sm, 8);
      expect(GoklaySpacing.smPlus, 10);
      expect(GoklaySpacing.md, 12);
      expect(GoklaySpacing.lg, 16);
      expect(GoklaySpacing.xl, 24);
    });

    test('cards are 4 and buttons are pills', () {
      expect(GoklayRadii.card, const Radius.circular(4));
      expect(GoklayRadii.pill, const Radius.circular(100));
    });

    test('there is exactly one shadow, and it is barely there', () {
      expect(GoklayElevation.card, hasLength(1));
      expect(GoklayElevation.card.single.blurRadius, 1);
      expect(GoklayElevation.card.single.color, const Color(0x40000000));
    });

    test('the accessibility floor is the WCAG one', () {
      expect(GoklayA11y.minTapTarget, 48);
      expect(GoklayA11y.minContrastBody, 4.5);
      expect(GoklayA11y.minContrastLarge, 3.0);
    });

    test('motion is short enough to stay out of the way', () {
      expect(GoklayMotion.fast, const Duration(milliseconds: 120));
      expect(GoklayMotion.normal, const Duration(milliseconds: 240));
      expect(GoklayMotion.normal.inMilliseconds, lessThan(300));
    });
  });
}
