import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/order.dart';
import '../api/models/support.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/customer_strings.dart';
import '../state/store.dart';
import '../widgets/messages.dart';
import '../widgets/scaffold.dart';
import '../widgets/star_rating.dart';

/// Leaving a review on a finished order.
///
/// Two subjects, and only the ones the order really had: the shop always, the
/// rider only once one collected it. P16 refuses a review whose subject was
/// not on the order, so offering a rider who never existed would produce a
/// `403` the customer could do nothing about.
///
/// Each subject is submitted separately, because they are separate reviews on
/// the server and one may be accepted while the other is refused as a
/// duplicate.
class ReviewScreen extends StatefulWidget {
  /// Creates the form for [order].
  const ReviewScreen({required this.order, super.key});

  /// The order being reviewed.
  final Order order;

  @override
  State<ReviewScreen> createState() => _ReviewScreenState();
}

class _ReviewScreenState extends State<ReviewScreen> {
  final ActionRunner _runner = ActionRunner();
  final TextEditingController _comment = TextEditingController();
  int _shopStars = 0;
  int _riderStars = 0;
  bool _sent = false;

  @override
  void dispose() {
    _runner.dispose();
    _comment.dispose();
    super.dispose();
  }

  bool get _canSubmit => _shopStars > 0 || _riderStars > 0;

  Future<void> _submit() async {
    final Dependencies dependencies = AppScope.of(context);
    final String? partnerId = widget.order.partnerId;
    setState(() => _sent = false);
    final bool ok = await _runner.run(() async {
      if (_shopStars > 0) {
        await dependencies.support.submitReview(
          orderId: widget.order.id,
          subject: ReviewSubject.merchant,
          subjectId: widget.order.merchantId,
          rating: _shopStars,
          comment: _comment.text.trim(),
        );
      }
      if (_riderStars > 0 && partnerId != null) {
        await dependencies.support.submitReview(
          orderId: widget.order.id,
          subject: ReviewSubject.partner,
          subjectId: partnerId,
          rating: _riderStars,
          comment: _comment.text.trim(),
        );
      }
    });
    if (ok && mounted) {
      setState(() => _sent = true);
    }
  }

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    final bool hasRider = widget.order.partnerId != null;
    return CustomerScaffold(
      title: strings.reviewTitle,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => ListView(
          padding: const EdgeInsets.all(GoklaySpacing.lg),
          children: <Widget>[
            Text(widget.order.code, style: GoklayTextStyles.title),
            const SizedBox(height: GoklaySpacing.lg),
            Text(strings.rateShop, style: GoklayTextStyles.emphasis),
            StarRating(
              rating: _shopStars,
              semanticLabel: strings.rateShop,
              onChanged: (int stars) => setState(() => _shopStars = stars),
            ),
            if (hasRider) ...<Widget>[
              const SizedBox(height: GoklaySpacing.lg),
              Text(strings.rateRider, style: GoklayTextStyles.emphasis),
              StarRating(
                rating: _riderStars,
                semanticLabel: strings.rateRider,
                onChanged: (int stars) => setState(() => _riderStars = stars),
              ),
            ],
            LabelledField(
              label: strings.reviewComment,
              controller: _comment,
              maxLines: 3,
            ),
            if (_runner.error != null)
              Padding(
                padding: const EdgeInsets.only(top: GoklaySpacing.sm),
                child: Text(
                  ErrorView.messageFor(context, _runner.error!),
                  style: GoklayTextStyles.caption.copyWith(
                    color: GoklayColors.danger,
                  ),
                ),
              ),
            if (_sent)
              Padding(
                padding: const EdgeInsets.only(top: GoklaySpacing.sm),
                child: Text(
                  strings.reviewThanks,
                  style: GoklayTextStyles.caption.copyWith(
                    color: GoklayColors.brand,
                  ),
                ),
              ),
            const SizedBox(height: GoklaySpacing.lg),
            GoklayButton(
              label: strings.submitReview,
              expand: true,
              onPressed: _canSubmit && !_runner.isBusy ? _submit : null,
            ),
          ],
        ),
      ),
    );
  }
}
