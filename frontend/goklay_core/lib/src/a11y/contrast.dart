import 'dart:ui';

import '../tokens/dimensions.dart';

/// WCAG 2.1 contrast, so "is this readable" is a number the tests can assert
/// rather than an opinion someone holds in a review.
///
/// The design file was drawn for a sighted designer on a colour-managed
/// monitor. A rider reading a delivery address on a cheap phone in Dhaka
/// sunlight is the case that actually has to work, and the only defence
/// against slowly losing that is a test over the real theme.
abstract final class GoklayContrast {
  /// The contrast ratio between two opaque colours, from 1.0 (identical) to
  /// 21.0 (black on white).
  ///
  /// Both colours must be opaque. A translucent colour has no ratio of its
  /// own — it depends on whatever is behind it — so the caller composites it
  /// first, which is exactly why [GoklayPalette.primaryTint] is stored as an
  /// opaque colour.
  static double ratio(Color a, Color b) {
    final double la = a.computeLuminance();
    final double lb = b.computeLuminance();
    final double lighter = la > lb ? la : lb;
    final double darker = la > lb ? lb : la;
    return (lighter + 0.05) / (darker + 0.05);
  }

  /// Whether [foreground] on [background] clears WCAG AA.
  ///
  /// Pass `large: true` for text at 18pt and above, or 14pt and above when
  /// bold, which AA lets through at 3:1 instead of 4.5:1.
  static bool meetsAA(Color foreground, Color background, {bool large = false}) {
    final double required =
        large ? GoklayA11y.minContrastLarge : GoklayA11y.minContrastBody;
    return ratio(foreground, background) >= required;
  }
}
