import 'package:flutter/widgets.dart';

import 'colors.dart';

/// The font families, and the reason there are three of them.
///
/// The Figma source uses two: Manrope for the display face and Poppins for
/// everything else. Neither contains a single Bengali glyph — checked, not
/// assumed — and Bengali is this product's default language (1.4). Shipping
/// only those two means every Bengali screen renders through whatever the
/// device happens to substitute, which on a low-end Android is either a
/// different font per line or a row of empty boxes.
///
/// So Noto Sans Bengali ships alongside them and is set as the fallback on
/// every style. Latin text keeps the faces the design chose; Bengali text
/// falls through to a face that can actually draw it. Noto Sans Bengali covers
/// Latin too, so "GoKlay" inside a Bengali sentence stays in one face instead
/// of switching mid-line.
abstract final class GoklayFonts {
  /// The package the font assets live in. Fonts declared by a package are
  /// addressed as `packages/<name>/<family>`, which [TextStyle.package] does.
  static const String package = 'goklay_core';

  /// The display face. Figma uses it at one weight, Bold, and only for the
  /// largest text on a screen.
  static const String display = 'Manrope';

  /// The UI face: every title, label, button and paragraph.
  static const String text = 'Poppins';

  /// The Bengali face, and the fallback under both of the above.
  static const String bengali = 'NotoSansBengali';

  /// The fallback list applied to every style in [GoklayTextStyles].
  static const List<String> fallback = <String>[bengali];

  /// A family name as the engine addresses it once the asset ships inside a
  /// package. [TextStyle.package] does this for a single style; [ThemeData]
  /// takes a bare string and needs it spelled out.
  static String qualified(String family) => 'packages/$package/$family';
}

/// The type scale, read from the Figma source `NlVjn8OuvmLjbm8z8TDVlR`.
///
/// Each style records the node it was read from, so any of them can be checked
/// against the design without guessing which screen it came from. Sizes are in
/// logical pixels at the design's 430pt frame width.
abstract final class GoklayTextStyles {
  static TextStyle _style({
    required String family,
    required double size,
    required FontWeight weight,
    required Color color,
    double? height,
  }) {
    return TextStyle(
      fontFamily: family,
      fontFamilyFallback: GoklayFonts.fallback,
      package: GoklayFonts.package,
      fontSize: size,
      fontWeight: weight,
      height: height,
      color: color,
    );
  }

  /// Manrope Bold 30. The one line at the top of an onboarding or empty
  /// screen. Figma node `1:72`.
  static final TextStyle display = _style(
    family: GoklayFonts.display,
    size: 30,
    weight: FontWeight.w700,
    color: GoklayColors.textPrimary,
  );

  /// Poppins Medium 16. A card's heading, a section title, a list row's
  /// primary line. Figma node `1:286`.
  static final TextStyle title = _style(
    family: GoklayFonts.text,
    size: 16,
    weight: FontWeight.w500,
    color: GoklayColors.textPrimary,
  );

  /// Poppins SemiBold 16. Text that is the point of its own container — a
  /// promo code, an order code, a total. Figma node `1:290`.
  static final TextStyle emphasis = _style(
    family: GoklayFonts.text,
    size: 16,
    weight: FontWeight.w600,
    color: GoklayColors.textPrimary,
  );

  /// Poppins Light 16 / 20. Running paragraph text. Figma node `1:73`.
  static final TextStyle body = _style(
    family: GoklayFonts.text,
    size: 16,
    weight: FontWeight.w300,
    height: 20 / 16,
    color: GoklayColors.textPrimary,
  );

  /// Poppins Regular 12 / 15. Captions, helper lines, timestamps. Figma node
  /// `1:287`.
  static final TextStyle caption = _style(
    family: GoklayFonts.text,
    size: 12,
    weight: FontWeight.w400,
    height: 15 / 12,
    color: GoklayColors.textSecondary,
  );

  /// Poppins Medium 14. The label inside a button. Figma nodes `1:298`
  /// (outlined) and `1:300` (filled).
  static final TextStyle label = _style(
    family: GoklayFonts.text,
    size: 14,
    weight: FontWeight.w500,
    color: GoklayColors.textPrimary,
  );

  /// Every style above, for the tests that hold the whole scale to a rule.
  static List<TextStyle> get all => <TextStyle>[
    display,
    title,
    emphasis,
    body,
    caption,
    label,
  ];
}
