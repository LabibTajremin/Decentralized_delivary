import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

/// A rounded white card, the one container shape the design uses.
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

/// A strip saying that taps are waiting for a connection.
///
/// It is the rider's only window into the outbox, and it exists because the
/// alternative — a tap that appears to have worked and silently has not — is
/// how a delivery gets marked twice or not at all.
class OutboxBanner extends StatelessWidget {
  /// Creates the banner.
  const OutboxBanner({
    required this.pending,
    required this.label,
    required this.sendLabel,
    required this.onSend,
    super.key,
  });

  /// How many taps are waiting.
  final int pending;

  /// What to call the wait.
  final String label;

  /// What to call the button.
  final String sendLabel;

  /// Called to try again now.
  final VoidCallback? onSend;

  @override
  Widget build(BuildContext context) {
    if (pending == 0) {
      return const SizedBox.shrink();
    }
    return Container(
      width: double.infinity,
      color: GoklayColors.brandSubtle,
      padding: const EdgeInsets.symmetric(
        horizontal: GoklaySpacing.lg,
        vertical: GoklaySpacing.sm,
      ),
      child: Row(
        children: <Widget>[
          Expanded(
            child: Text(
              '$label ($pending)',
              style: GoklayTextStyles.caption.copyWith(
                color: GoklayColors.textPrimary,
              ),
            ),
          ),
          GoklayTapTarget(
            onTap: onSend,
            semanticLabel: sendLabel,
            excludeChildSemantics: true,
            child: Text(
              sendLabel,
              style: GoklayTextStyles.caption.copyWith(
                color: GoklayColors.brand,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// A row of a place: who, where, and a number to call.
class PlaceRow extends StatelessWidget {
  /// Creates a place row.
  const PlaceRow({
    required this.heading,
    required this.name,
    required this.singleLine,
    required this.phone,
    super.key,
  });

  /// What this end of the delivery is called.
  final String heading;

  /// Who is there.
  final String name;

  /// The address, as the server composed it.
  final String singleLine;

  /// A number to call.
  final String phone;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        Text(
          heading,
          style: GoklayTextStyles.caption.copyWith(
            color: GoklayColors.textSecondary,
          ),
        ),
        Text(name, style: GoklayTextStyles.emphasis),
        Text(singleLine, style: GoklayTextStyles.body),
        if (phone.isNotEmpty)
          Text(
            phone,
            style: GoklayTextStyles.caption.copyWith(
              color: GoklayColors.brand,
            ),
          ),
      ],
    );
  }
}
