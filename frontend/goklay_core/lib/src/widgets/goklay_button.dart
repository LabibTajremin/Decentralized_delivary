import 'package:flutter/widgets.dart';

import '../a11y/tap_target.dart';
import '../tokens/colors.dart';
import '../tokens/dimensions.dart';
import '../tokens/typography.dart';

/// Which of the two buttons in the design this is.
enum GoklayButtonVariant {
  /// Solid brand green. The action the screen wants you to take.
  filled,

  /// A hairline brand outline on the surface. The lesser of two actions,
  /// drawn beside a [filled] one (Figma node `1:296`).
  outlined,
}

/// The button, exactly as the Figma source draws it and as tall as a finger
/// needs it to be.
///
/// The design paints a pill 12pt above and below a 14pt label, which lands
/// near 41dp. This paints that, and then lets [GoklayTapTarget] make the touch
/// area 48dp — so the screen matches the file while the control matches WCAG.
///
/// [label] is passed in, never composed here. Every word on a button that says
/// anything about an order, a price or a rule arrives from the server already
/// written in the user's language (2.9).
class GoklayButton extends StatelessWidget {
  /// Creates a button. A null [onPressed] renders it disabled.
  const GoklayButton({
    required this.label,
    required this.onPressed,
    this.variant = GoklayButtonVariant.filled,
    this.expand = false,
    super.key,
  });

  /// The text on the button, already in the user's language.
  final String label;

  /// Called on tap. Null disables the button and dims it.
  final VoidCallback? onPressed;

  /// Filled or outlined.
  final GoklayButtonVariant variant;

  /// Whether to stretch to the width of the parent rather than hug [label].
  final bool expand;

  /// Whether this button is currently interactive.
  bool get isEnabled => onPressed != null;

  @override
  Widget build(BuildContext context) {
    final bool filled = variant == GoklayButtonVariant.filled;
    final Color foreground = filled
        ? GoklayColors.onBrand
        : GoklayColors.brand;

    final Widget pill = Container(
      padding: const EdgeInsets.symmetric(
        horizontal: 39,
        vertical: GoklaySpacing.md,
      ),
      decoration: BoxDecoration(
        color: filled ? GoklayColors.brand : null,
        borderRadius: const BorderRadius.all(GoklayRadii.pill),
        border: filled
            ? null
            : Border.all(color: GoklayColors.brand, width: 0.5),
      ),
      child: Text(
        label,
        textAlign: TextAlign.center,
        style: GoklayTextStyles.label.copyWith(color: foreground),
      ),
    );

    return Opacity(
      opacity: isEnabled ? 1.0 : 0.4,
      child: GoklayTapTarget(
        onTap: onPressed,
        semanticLabel: label,
        excludeChildSemantics: true,
        child: expand
            ? SizedBox(width: double.infinity, child: pill)
            : pill,
      ),
    );
  }
}
