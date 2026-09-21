import 'dart:ui';

/// The colour tokens, read from the Figma source `NlVjn8OuvmLjbm8z8TDVlR`.
///
/// Every value here is a Figma *variable*, not a colour someone sampled off a
/// screenshot — the names in the doc comments are the variable names as the
/// file defines them, so a change in Figma has one obvious landing place here.
///
/// The whole product uses seven colours. That is not an abridgement: the
/// design file defines seven and no screen uses an eighth.
abstract final class GoklayPalette {
  /// Figma `Black`. Primary text and icons.
  static const Color black = Color(0xFF0F0F0F);

  /// Figma `Gray`. Secondary text — captions, helper lines, inactive icons.
  static const Color gray = Color(0xFF656565);

  /// Figma `White`. Card and sheet surfaces, and text on a filled brand button.
  static const Color white = Color(0xFFFFFFFF);

  /// Figma `Soft Gray`. The page behind the cards.
  static const Color softGray = Color(0xFFF8F9FA);

  /// Figma `Primary Color`. The GoKlay green: filled buttons, active states,
  /// the selected tab.
  static const Color primary = Color(0xFF22874F);

  /// Figma `Emerald Green`. Success — a delivered order, a confirmed payment.
  static const Color emerald = Color(0xFF10B981);

  /// Figma `Soft Red`. Failure and destructive intent — a cancelled order, a
  /// failed delivery, the button that ends something.
  static const Color softRed = Color(0xFFFF5A5F);

  /// [primary] at 15% opacity, as the design uses it behind a promo code
  /// (`rgba(34,135,79,0.15)`, Figma node `1:289`).
  ///
  /// Written as an opaque colour rather than a translucent one because a
  /// translucent fill over an unknown surface has an unknown contrast ratio,
  /// and [GoklayContrast] has to be able to check it. This is [primary] at 15%
  /// composited onto [white], which is the only surface the design puts it on.
  static const Color primaryTint = Color(0xFFDEEDE5);
}

/// The palette above, named by the job each colour does.
///
/// Widgets reference these, never [GoklayPalette] directly: "the text is
/// secondary" survives a rebrand, "the text is `#656565`" does not.
abstract final class GoklayColors {
  /// The page behind everything.
  static const Color surface = GoklayPalette.softGray;

  /// Cards, sheets and the app bar — what sits on [surface].
  static const Color surfaceRaised = GoklayPalette.white;

  /// Headings, body copy, and anything the eye should land on first.
  static const Color textPrimary = GoklayPalette.black;

  /// Captions, helper text, timestamps, inactive icons.
  static const Color textSecondary = GoklayPalette.gray;

  /// Text and icons drawn on top of [brand].
  static const Color onBrand = GoklayPalette.white;

  /// The brand green: filled buttons, selected states, links.
  static const Color brand = GoklayPalette.primary;

  /// The brand green at 15% on white, for a badge or a code chip.
  static const Color brandSubtle = GoklayPalette.primaryTint;

  /// Something completed successfully.
  static const Color success = GoklayPalette.emerald;

  /// Something failed, or a control that destroys.
  static const Color danger = GoklayPalette.softRed;

  /// Hairlines and dividers. The design draws these as a 0.5px border in the
  /// brand colour on outlined controls, and otherwise relies on the card
  /// shadow, so this is used sparingly.
  static const Color border = GoklayPalette.gray;
}
