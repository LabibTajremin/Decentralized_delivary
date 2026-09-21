import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/job.dart';
import '../api/models/partner.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/partner_strings.dart';
import '../widgets/cards.dart';

/// What the rider should be looking at right now (ALG-08).
///
/// The list is the server's: bounded, nearest-pickup first, already filtered
/// by the rider's own distance choice (D4). A client that filtered a global
/// list by its own radius would be deciding visibility, which 2.9 forbids and
/// which would need a copy of `dispatch.partner_radius` to do at all.
///
/// An empty feed is never just empty. `reason` says `offline`, `at_capacity`
/// or `nothing_nearby`, and `notice` is the sentence — so a rider who forgot
/// to go on shift is told that rather than left staring at a blank screen.
class FeedScreen extends StatefulWidget {
  /// Creates the feed.
  const FeedScreen({required this.onJobSelected, this.showBack = true, super.key});

  /// Called with a job the rider tapped.
  final void Function(DeliveryJob job) onJobSelected;

  /// Whether to draw a back button.
  final bool showBack;

  @override
  State<FeedScreen> createState() => _FeedScreenState();
}

class _FeedScreenState extends State<FeedScreen> {
  final Store<PartnerFeed> _feed = Store<PartnerFeed>();
  final ActionRunner _runner = ActionRunner();

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _feed.dispose();
    _runner.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final Dependencies dependencies = PartnerScope.of(context);
    await _feed.load(() async {
      final ApiPage<PartnerFeed> page = await dependencies.partner.feed();
      return page.toAsyncData();
    });
  }

  Future<void> _setShift(DeliveryPartner partner) async {
    final Dependencies dependencies = PartnerScope.of(context);
    await _runner.run(
      () => dependencies.partner.setAvailability(
        partner.isOnShift
            ? DeliveryPartner.offline
            : DeliveryPartner.available,
      ),
    );
    await _load();
  }

  Future<void> _answer(
    DeliveryJob job,
    Future<DeliveryJob> Function() call,
  ) async {
    await _runner.run(() async {
      await call();
    });
    await _load();
  }

  @override
  Widget build(BuildContext context) {
    final PartnerStrings strings = PartnerLocalizations.of(context);
    return GoklayScaffold(
      title: strings.feedTitle,
      showBack: widget.showBack,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => AsyncView<PartnerFeed>(
          store: _feed,
          onRetry: _load,
          builder: (BuildContext context, PartnerFeed feed) =>
              _body(strings, feed),
        ),
      ),
    );
  }

  Widget _body(PartnerStrings strings, PartnerFeed feed) {
    final Dependencies dependencies = PartnerScope.of(context);
    return ListView(
      children: <Widget>[
        GoklayCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Text(feed.partner.name, style: GoklayTextStyles.emphasis),
              Text(
                feed.partner.availabilityLabel,
                style: GoklayTextStyles.body.copyWith(
                  color: feed.partner.isOnShift
                      ? GoklayColors.brand
                      : GoklayColors.textSecondary,
                ),
              ),
              Text(
                feed.partner.preferenceLabel,
                style: GoklayTextStyles.caption.copyWith(
                  color: GoklayColors.textSecondary,
                ),
              ),
              const SizedBox(height: GoklaySpacing.sm),
              GoklayButton(
                label: feed.partner.isOnShift
                    ? strings.goOffShift
                    : strings.goOnShift,
                variant: feed.partner.isOnShift
                    ? GoklayButtonVariant.outlined
                    : GoklayButtonVariant.filled,
                expand: true,
                onPressed: _runner.isBusy
                    ? null
                    : () => _setShift(feed.partner),
              ),
            ],
          ),
        ),
        if (feed.notice.isNotEmpty)
          Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: GoklaySpacing.lg,
              vertical: GoklaySpacing.sm,
            ),
            child: Text(feed.notice, style: GoklayTextStyles.body),
          ),
        if (_runner.error != null)
          Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: GoklaySpacing.lg,
            ),
            child: Text(
              ErrorView.messageFor(context, _runner.error!),
              style: GoklayTextStyles.caption.copyWith(
                color: GoklayColors.danger,
              ),
            ),
          ),
        for (final DeliveryJob job in feed.jobs)
          GoklayCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                GoklayTapTarget(
                  onTap: () => widget.onJobSelected(job),
                  semanticLabel: job.code,
                  excludeChildSemantics: true,
                  child: Row(
                    children: <Widget>[
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: <Widget>[
                            Text(
                              job.code,
                              style: GoklayTextStyles.emphasis,
                            ),
                            Text(
                              job.pickup.name,
                              style: GoklayTextStyles.body,
                            ),
                            Text(
                              '${strings.toPickup} ${job.toPickup}',
                              style: GoklayTextStyles.caption.copyWith(
                                color: GoklayColors.textSecondary,
                              ),
                            ),
                          ],
                        ),
                      ),
                      Column(
                        crossAxisAlignment: CrossAxisAlignment.end,
                        children: <Widget>[
                          Text(
                            job.distance,
                            style: GoklayTextStyles.caption,
                          ),
                          Text(
                            job.bandLabel,
                            style: GoklayTextStyles.caption.copyWith(
                              color: GoklayColors.brand,
                            ),
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
                if (job.isOffer) ...<Widget>[
                  const SizedBox(height: GoklaySpacing.sm),
                  Row(
                    children: <Widget>[
                      GoklayButton(
                        label: strings.acceptJob,
                        onPressed: _runner.isBusy
                            ? null
                            : () => _answer(
                                job,
                                () => dependencies.partner.accept(job.id),
                              ),
                      ),
                      const SizedBox(width: GoklaySpacing.sm),
                      GoklayButton(
                        label: strings.declineJob,
                        variant: GoklayButtonVariant.outlined,
                        onPressed: _runner.isBusy
                            ? null
                            : () => _answer(
                                job,
                                () => dependencies.partner.decline(job.id),
                              ),
                      ),
                    ],
                  ),
                ],
              ],
            ),
          ),
      ],
    );
  }
}
