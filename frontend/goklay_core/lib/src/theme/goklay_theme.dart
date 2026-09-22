import 'package:flutter/material.dart';

import '../tokens/colors.dart';
import '../tokens/typography.dart';

/// The one [ThemeData] all three apps use.
///
/// Built by naming every colour explicitly rather than with
/// `ColorScheme.fromSeed`. Seeding generates a tonal palette from one input,
/// which would produce a dozen colours the Figma file never defined and quietly
/// make the apps a different product from the design. The design has seven
/// colours; so does this.
abstract final class GoklayTheme {
  /// The light theme. There is no dark theme yet: the Figma source does not
  /// define one, and inventing it here would be inventing design.
  static ThemeData light() {
    const ColorScheme scheme = ColorScheme(
      brightness: Brightness.light,
      primary: GoklayColors.brand,
      onPrimary: GoklayColors.onBrand,
      secondary: GoklayColors.brand,
      onSecondary: GoklayColors.onBrand,
      error: GoklayColors.danger,
      onError: GoklayColors.onBrand,
      surface: GoklayColors.surfaceRaised,
      onSurface: GoklayColors.textPrimary,
    );

    return ThemeData(
      useMaterial3: true,
      colorScheme: scheme,
      scaffoldBackgroundColor: GoklayColors.surface,
      fontFamily: GoklayFonts.qualified(GoklayFonts.text),
      fontFamilyFallback: <String>[
        GoklayFonts.qualified(GoklayFonts.bengali),
      ],
      // Material's own floor, so a stock IconButton or Checkbox is padded to
      // 48dp the same way GoklayTapTarget pads ours.
      materialTapTargetSize: MaterialTapTargetSize.padded,
      textTheme: textTheme,
      appBarTheme: const AppBarThemeData(
        backgroundColor: GoklayColors.surfaceRaised,
        foregroundColor: GoklayColors.textPrimary,
        elevation: 0,
        scrolledUnderElevation: 0,
        centerTitle: false,
      ),
      dividerTheme: const DividerThemeData(
        color: GoklayColors.border,
        thickness: 0.5,
        space: 0.5,
      ),
    );
  }

  /// The type scale mapped onto Material's slots, so a stock widget that reads
  /// `Theme.of(context).textTheme` gets the designed face rather than Roboto.
  static TextTheme get textTheme => TextTheme(
    displayLarge: GoklayTextStyles.display,
    headlineLarge: GoklayTextStyles.display,
    titleLarge: GoklayTextStyles.emphasis,
    titleMedium: GoklayTextStyles.title,
    bodyLarge: GoklayTextStyles.body,
    bodyMedium: GoklayTextStyles.body,
    bodySmall: GoklayTextStyles.caption,
    labelLarge: GoklayTextStyles.label,
    labelSmall: GoklayTextStyles.caption,
  );
}
