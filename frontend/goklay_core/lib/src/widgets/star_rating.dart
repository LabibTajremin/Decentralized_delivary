import 'package:flutter/material.dart';
import '../a11y/tap_target.dart';
import '../tokens/colors.dart';

/// Five stars, either read-only or tappable.
///
/// The average a subject has is [Rating.average], computed by the server over
/// every review. This paints a value; it never derives one from a list.
class StarRating extends StatelessWidget {
  /// Creates a rating display. [onChanged] makes it an input.
  const StarRating({
    required this.rating,
    required this.semanticLabel,
    this.onChanged,
    super.key,
  });

  /// How many stars are filled, one to five.
  final int rating;

  /// What a screen reader calls the whole control.
  final String semanticLabel;

  /// Called with the star tapped, one-based. Null makes it read-only.
  final ValueChanged<int>? onChanged;

  /// How many stars there are.
  static const int maxStars = 5;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      label: semanticLabel,
      value: '$rating',
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: <Widget>[
          for (int star = 1; star <= maxStars; star++)
            GoklayTapTarget(
              onTap: onChanged == null ? null : () => onChanged!(star),
              semanticLabel: '$star',
              excludeChildSemantics: true,
              child: Icon(
                star <= rating ? Icons.star : Icons.star_border,
                size: 24,
                color: star <= rating
                    ? GoklayColors.brand
                    : GoklayColors.textSecondary,
              ),
            ),
        ],
      ),
    );
  }
}
