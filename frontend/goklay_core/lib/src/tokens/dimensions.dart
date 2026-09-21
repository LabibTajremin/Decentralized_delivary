import 'package:flutter/painting.dart';

/// Spacing steps, read from the gaps and padding in the Figma source.
///
/// These are the values the design actually uses, not a tidied-up scale: the
/// file contains 4, 6, 8, 10, 12, 16 and 24 and nothing between them. Inventing
/// a smooth ramp here and rounding screens onto it would quietly redraw the
/// design.
abstract final class GoklaySpacing {
  /// 4 — the gap between two adjacent buttons (Figma node `1:296`).
  static const double xxs = 4;

  /// 6 — the gap between a heading and the line under it (`1:71`).
  static const double xs = 6;

  /// 8 — padding inside a chip, and the gap between two cards (`1:283`).
  static const double sm = 8;

  /// 10 — the gap between the two text blocks of a card header (`1:285`).
  static const double smPlus = 10;

  /// 12 — padding inside a card, and a button's vertical padding (`1:284`).
  static const double md = 12;

  /// 16 — the screen's side gutter.
  static const double lg = 16;

  /// 24 — the gap between the sections of a card (`1:284`).
  static const double xl = 24;
}

/// Corner radii, read from the Figma source.
abstract final class GoklayRadii {
  /// 4 — cards, chips, input fields (Figma nodes `1:284`, `1:289`).
  static const Radius card = Radius.circular(4);

  /// 100 — buttons. Large enough to read as a pill at any button height
  /// (Figma nodes `1:297`, `1:299`).
  static const Radius pill = Radius.circular(100);
}

/// The card shadow, read from the Figma source.
abstract final class GoklayElevation {
  /// `0px 0px 1px rgba(0,0,0,0.25)` — the only shadow in the design file
  /// (Figma node `1:284`). It separates a card from the page without
  /// pretending the card is floating above it.
  static const List<BoxShadow> card = <BoxShadow>[
    BoxShadow(color: Color(0x40000000), blurRadius: 1),
  ];
}

/// The accessibility floor. Not a design token — a rule the design has to
/// clear.
///
/// The Figma buttons are drawn 12pt above and below a 14pt label, which lands
/// around 41dp tall. That is under the 48dp both Material and WCAG 2.2 ask
/// for, so the foundation pads the *touch* target up to 48dp while leaving the
/// painted button exactly as designed. The control looks like the design and
/// behaves like an accessible one; `GoklayTapTarget` is how, and
/// `tap_target_test.dart` is why it cannot regress.
abstract final class GoklayA11y {
  /// The minimum width and height of anything a finger is meant to hit.
  static const double minTapTarget = 48;

  /// WCAG 2.1 AA for body text: 4.5:1 against its background.
  static const double minContrastBody = 4.5;

  /// WCAG 2.1 AA for text at 18pt+, or 14pt+ bold: 3:1.
  static const double minContrastLarge = 3.0;
}

/// Animation durations. Short, because every one of them is on the critical
/// path of a screen that has a 2-second cold-start budget.
abstract final class GoklayMotion {
  /// A control reacting to a touch.
  static const Duration fast = Duration(milliseconds: 120);

  /// A sheet, a banner, or a screen transition.
  static const Duration normal = Duration(milliseconds: 240);
}
