import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/job.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/partner_strings.dart';
import '../widgets/cards.dart';

/// The rider's own deliveries, current and past.
class JobsScreen extends StatefulWidget {
  /// Creates the list.
  const JobsScreen({
    required this.onJobSelected,
    this.showBack = true,
    super.key,
  });

  /// Called with the job the rider tapped.
  final void Function(DeliveryJob job) onJobSelected;

  /// Whether to draw a back button.
  final bool showBack;

  @override
  State<JobsScreen> createState() => _JobsScreenState();
}

class _JobsScreenState extends State<JobsScreen> {
  final Store<DeliveryJobList> _jobs = Store<DeliveryJobList>();
  bool _live = true;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _jobs.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final Dependencies dependencies = PartnerScope.of(context);
    await _jobs.load(() async {
      final ApiPage<DeliveryJobList> page = await dependencies.partner.jobs(
        live: _live,
      );
      return page.toAsyncData();
    });
  }

  Future<void> _select(bool live) async {
    setState(() => _live = live);
    await _load();
  }

  @override
  Widget build(BuildContext context) {
    final PartnerStrings strings = PartnerLocalizations.of(context);
    return GoklayScaffold(
      title: strings.jobsTitle,
      showBack: widget.showBack,
      body: Column(
        children: <Widget>[
          Row(
            children: <Widget>[
              Expanded(
                child: _Tab(
                  label: strings.jobsLive,
                  selected: _live,
                  onTap: () => _select(true),
                ),
              ),
              Expanded(
                child: _Tab(
                  label: strings.jobsPast,
                  selected: !_live,
                  onTap: () => _select(false),
                ),
              ),
            ],
          ),
          Expanded(
            child: AsyncView<DeliveryJobList>(
              store: _jobs,
              onRetry: _load,
              builder: (BuildContext context, DeliveryJobList jobs) =>
                  jobs.isEmpty
                  ? EmptyView(message: strings.noJobs)
                  : ListView(
                      children: <Widget>[
                        for (final DeliveryJob job in jobs.jobs)
                          GoklayCard(
                            child: GoklayTapTarget(
                              onTap: () => widget.onJobSelected(job),
                              semanticLabel: job.code,
                              excludeChildSemantics: true,
                              child: Row(
                                children: <Widget>[
                                  Expanded(
                                    child: Column(
                                      crossAxisAlignment:
                                          CrossAxisAlignment.start,
                                      children: <Widget>[
                                        Text(
                                          job.code,
                                          style: GoklayTextStyles.emphasis,
                                        ),
                                        Text(
                                          job.statusLabel,
                                          style: GoklayTextStyles.caption
                                              .copyWith(
                                                color: job.live
                                                    ? GoklayColors.brand
                                                    : GoklayColors
                                                          .textSecondary,
                                              ),
                                        ),
                                      ],
                                    ),
                                  ),
                                  Text(
                                    job.distance,
                                    style: GoklayTextStyles.caption,
                                  ),
                                ],
                              ),
                            ),
                          ),
                      ],
                    ),
            ),
          ),
        ],
      ),
    );
  }
}

class _Tab extends StatelessWidget {
  const _Tab({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GoklayTapTarget(
      onTap: onTap,
      semanticLabel: label,
      excludeChildSemantics: true,
      child: Container(
        alignment: Alignment.center,
        padding: const EdgeInsets.symmetric(vertical: GoklaySpacing.md),
        decoration: BoxDecoration(
          color: GoklayColors.surfaceRaised,
          border: Border(
            bottom: BorderSide(
              width: 2,
              color: selected
                  ? GoklayColors.brand
                  : GoklayColors.surfaceRaised,
            ),
          ),
        ),
        child: Text(
          label,
          style: GoklayTextStyles.body.copyWith(
            color: selected
                ? GoklayColors.brand
                : GoklayColors.textSecondary,
          ),
        ),
      ),
    );
  }
}
