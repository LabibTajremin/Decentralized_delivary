import 'package:flutter/widgets.dart';

import '../tokens/dimensions.dart';

/// Guarantees that whatever it wraps can be hit by a finger, without changing
/// how that thing is drawn.
///
/// The Figma buttons are 12pt of padding around a 14pt label, which comes out
/// near 41dp — under the 48dp floor. The fix is not to redraw the design
/// bigger: it is to let the button paint at its designed size and make the
/// *touch* area 48dp around it. A rider with cold hands on a cracked screen
/// hits the target; the screen still looks like the file it came from.
///
/// Every interactive widget in this library goes through here. A widget that
/// does not is a widget the accessibility test will catch.
class GoklayTapTarget extends StatelessWidget {
  /// Wraps [child] in a hit area of at least [GoklayA11y.minTapTarget] square.
  const GoklayTapTarget({
    required this.child,
    this.onTap,
    this.semanticLabel,
    this.excludeChildSemantics = false,
    super.key,
  });

  /// What is drawn. Painted at its own size, never stretched to the floor.
  final Widget child;

  /// Called when the target is tapped. A null callback renders the child as
  /// a disabled control: still laid out, but not hittable.
  final VoidCallback? onTap;

  /// What a screen reader announces. Supply this whenever [child] is an icon
  /// with no text of its own.
  final String? semanticLabel;

  /// Whether to hide [child]'s own semantics behind [semanticLabel]. Set this
  /// for a control whose child would otherwise be read out twice.
  final bool excludeChildSemantics;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      enabled: onTap != null,
      label: semanticLabel,
      excludeSemantics: excludeChildSemantics,
      child: GestureDetector(
        onTap: onTap,
        behavior: HitTestBehavior.opaque,
        child: ConstrainedBox(
          constraints: const BoxConstraints(
            minWidth: GoklayA11y.minTapTarget,
            minHeight: GoklayA11y.minTapTarget,
          ),
          child: Center(
            widthFactor: 1,
            heightFactor: 1,
            child: child,
          ),
        ),
      ),
    );
  }
}
