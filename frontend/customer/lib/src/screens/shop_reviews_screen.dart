import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/support.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/customer_strings.dart';
import '../widgets/cards.dart';

/// What other customers said about a shop.
///
/// The average comes from `GET /v1/ratings` — computed by the server over
/// every review, not averaged here from the page of reviews on screen. Those
/// two numbers would differ the moment the list was paged, and the one the
/// customer should see is the one the shop is actually judged on.
class ShopReviewsScreen extends StatefulWidget {
  /// Creates the screen for a shop.
  const ShopReviewsScreen({
    required this.merchantId,
    required this.merchantName,
    super.key,
  });

  /// Whose reviews.
  final String merchantId;

  /// Its name, for the heading.
  final String merchantName;

  @override
  State<ShopReviewsScreen> createState() => _ShopReviewsScreenState();
}

class _ShopReviewsScreenState extends State<ShopReviewsScreen> {
  final Store<List<Review>> _reviews = Store<List<Review>>();
  Rating? _rating;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _reviews.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final Dependencies dependencies = AppScope.of(context);
    await _reviews.load(() async {
      final ApiPage<List<Review>> page = await dependencies.support.reviews(
        subject: ReviewSubject.merchant,
        subjectId: widget.merchantId,
      );
      return page.toAsyncData();
    });
    try {
      final ApiPage<Rating> rating = await dependencies.support.rating(
        subject: ReviewSubject.merchant,
        subjectId: widget.merchantId,
      );
      if (mounted) {
        setState(() => _rating = rating.value);
      }
    } on ApiError {
      // The reviews loaded; a missing aggregate is not a reason to blank
      // them. The header simply says there is no rating yet.
      if (mounted) {
        setState(() => _rating = null);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    final Rating? rating = _rating;
    return GoklayScaffold(
      title: widget.merchantName,
      body: AsyncView<List<Review>>(
        store: _reviews,
        onRetry: _load,
        builder: (BuildContext context, List<Review> reviews) => ListView(
          children: <Widget>[
            Padding(
              padding: const EdgeInsets.all(GoklaySpacing.lg),
              child: rating != null && rating.hasRatings
                  ? Row(
                      children: <Widget>[
                        StarRating(
                          rating: rating.average.round(),
                          semanticLabel: strings.rateShop,
                        ),
                        const SizedBox(width: GoklaySpacing.sm),
                        Text(
                          '${rating.average} (${rating.count})',
                          style: GoklayTextStyles.caption,
                        ),
                      ],
                    )
                  : Text(
                      strings.noRatingYet,
                      style: GoklayTextStyles.caption.copyWith(
                        color: GoklayColors.textSecondary,
                      ),
                    ),
            ),
            if (reviews.isEmpty)
              EmptyView(message: strings.noReviews)
            else
              for (final Review review in reviews)
                GoklayCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: <Widget>[
                      StarRating(
                        rating: review.rating,
                        semanticLabel: strings.reviewTitle,
                      ),
                      if (review.comment.isNotEmpty)
                        Text(
                          review.comment,
                          style: GoklayTextStyles.body,
                        ),
                    ],
                  ),
                ),
          ],
        ),
      ),
    );
  }
}
