import 'package:flutter/material.dart';
import '../a11y/tap_target.dart';
import '../l10n/goklay_strings.dart';
import '../tokens/colors.dart';
import '../tokens/dimensions.dart';
import '../tokens/typography.dart';


/// Minus, a number, plus.
///
/// It reports the quantity it was asked for and holds none of its own. On the
/// cart that matters: the server revalidates and reprices on every change, so
/// the number on screen has to be the server's answer rather than an optimistic
/// local count that a rejected change would leave wrong.
class QuantityStepper extends StatelessWidget {
  /// Creates a stepper.
  const QuantityStepper({
    required this.quantity,
    required this.onChanged,
    this.minimum = 1,
    this.maximum = 20,
    this.enabled = true,
    super.key,
  });

  /// What to show.
  final int quantity;

  /// Called with the new quantity.
  final ValueChanged<int> onChanged;

  /// The fewest allowed. Zero on the cart, where it means "remove".
  final int minimum;

  /// The most allowed. The cart endpoint caps this at 20 and refuses more;
  /// the stepper stops there so the customer is not offered a change the
  /// server will reject.
  final int maximum;

  /// Whether the buttons respond.
  final bool enabled;

  @override
  Widget build(BuildContext context) {
    final GoklayStrings strings = GoklayLocalizations.of(context);
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: <Widget>[
        _StepperButton(
          icon: Icons.remove,
          semanticLabel: strings.decreaseQuantity,
          onPressed: enabled && quantity > minimum
              ? () => onChanged(quantity - 1)
              : null,
        ),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: GoklaySpacing.md),
          child: Text('$quantity', style: GoklayTextStyles.emphasis),
        ),
        _StepperButton(
          icon: Icons.add,
          semanticLabel: strings.increaseQuantity,
          onPressed: enabled && quantity < maximum
              ? () => onChanged(quantity + 1)
              : null,
        ),
      ],
    );
  }
}

class _StepperButton extends StatelessWidget {
  const _StepperButton({
    required this.icon,
    required this.semanticLabel,
    required this.onPressed,
  });

  final IconData icon;
  final String semanticLabel;
  final VoidCallback? onPressed;

  @override
  Widget build(BuildContext context) {
    return Opacity(
      opacity: onPressed == null ? 0.4 : 1,
      child: GoklayTapTarget(
        onTap: onPressed,
        semanticLabel: semanticLabel,
        excludeChildSemantics: true,
        child: Container(
          decoration: const BoxDecoration(
            color: GoklayColors.brandSubtle,
            borderRadius: BorderRadius.all(GoklayRadii.card),
          ),
          padding: const EdgeInsets.all(GoklaySpacing.xs),
          child: Icon(icon, size: 18, color: GoklayColors.brand),
        ),
      ),
    );
  }
}
