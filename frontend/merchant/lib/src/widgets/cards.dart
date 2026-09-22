import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

/// A rounded white card, the one container shape the design uses.
///
/// The merchant app's copy of the customer app's card. It is four lines and
/// carries no behaviour, which is why it is not in `goklay_core`: a shared
/// widget earns its place by holding a decision, and this one holds a radius.
class GoklayCard extends StatelessWidget {
  /// Creates a card.
  const GoklayCard({required this.child, super.key});

  /// What is in it.
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.symmetric(
        horizontal: GoklaySpacing.lg,
        vertical: GoklaySpacing.xs,
      ),
      padding: const EdgeInsets.all(GoklaySpacing.md),
      decoration: const BoxDecoration(
        color: GoklayColors.surfaceRaised,
        borderRadius: BorderRadius.all(GoklayRadii.card),
        boxShadow: GoklayElevation.card,
      ),
      child: child,
    );
  }
}
