import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/job.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/partner_strings.dart';
import '../widgets/cards.dart';

/// One delivery: where to collect it, where it goes, and the one thing to do
/// next.
///
/// **This is the screen the offline queue exists for.** A rider taps
/// "collected" at a counter with no signal and "delivered" in a stairwell with
/// none either, and those two taps must arrive in that order or not at all —
/// the order state machine refuses `delivered` from an order that never
/// reached `picked_up`. So a tap that cannot reach the server is queued
/// rather than lost, the queue replays strictly oldest-first, and it stops at
/// the first action the network still cannot carry.
class JobScreen extends StatefulWidget {
  /// Creates the screen over [job].
  const JobScreen({required this.job, super.key});

  /// The job as the list last saw it.
  final DeliveryJob job;

  @override
  State<JobScreen> createState() => _JobScreenState();
}

class _JobScreenState extends State<JobScreen> {
  final ActionRunner _runner = ActionRunner();
  final TextEditingController _reason = TextEditingController();
  late DeliveryJob _job = widget.job;
  bool _queued = false;

  @override
  void initState() {
    super.initState();
    _reason.addListener(_onTyped);
  }

  @override
  void dispose() {
    _reason
      ..removeListener(_onTyped)
      ..dispose();
    _runner.dispose();
    super.dispose();
  }

  void _onTyped() => setState(() {});

  /// Sends a transition, queueing it when the device is offline.
  ///
  /// Only the offline case is queued. A refusal — a `409` because another
  /// rider took the job — is shown, because it will be refused just as firmly
  /// in an hour and a queue that retried it would never drain.
  Future<void> _move(
    Future<DeliveryJob> Function() call, {
    required String path,
    Object? body,
  }) async {
    final Dependencies dependencies = PartnerScope.of(context);
    setState(() => _queued = false);
    await _runner.run(() async {
      try {
        final DeliveryJob updated = await call();
        if (mounted) {
          setState(() => _job = updated);
        }
      } on ApiError catch (error) {
        if (!error.isOffline) {
          rethrow;
        }
        dependencies.outbox.enqueue(
          QueuedAction(
            id: '${_job.id}-$path',
            method: 'POST',
            path: path,
            body: body,
          ),
        );
        if (mounted) {
          setState(() => _queued = true);
        }
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final PartnerStrings strings = PartnerLocalizations.of(context);
    final GoklayStrings core = GoklayLocalizations.of(context);
    final Dependencies dependencies = PartnerScope.of(context);
    return GoklayScaffold(
      title: _job.code,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => ListView(
          children: <Widget>[
            GoklayCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Text(_job.statusLabel, style: GoklayTextStyles.title),
                  Text(
                    '${strings.wholeDistance} ${_job.distance}',
                    style: GoklayTextStyles.caption,
                  ),
                  Text(
                    _job.bandLabel,
                    style: GoklayTextStyles.caption.copyWith(
                      color: GoklayColors.brand,
                    ),
                  ),
                  if (_job.reason.isNotEmpty)
                    Text(
                      _job.reason,
                      style: GoklayTextStyles.caption.copyWith(
                        color: GoklayColors.danger,
                      ),
                    ),
                ],
              ),
            ),
            GoklayCard(
              child: PlaceRow(
                heading: strings.pickup,
                name: _job.pickup.name,
                singleLine: _job.pickup.singleLine,
                phone: _job.pickup.phone,
              ),
            ),
            GoklayCard(
              child: PlaceRow(
                heading: strings.dropOff,
                name: _job.destination.name,
                singleLine: _job.destination.singleLine,
                phone: _job.destination.phone,
              ),
            ),
            if (_queued)
              Padding(
                padding: const EdgeInsets.symmetric(
                  horizontal: GoklaySpacing.lg,
                ),
                child: Text(
                  core.queuedOffline,
                  style: GoklayTextStyles.caption.copyWith(
                    color: GoklayColors.brand,
                  ),
                ),
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
            Padding(
              padding: const EdgeInsets.all(GoklaySpacing.lg),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: <Widget>[
                  if (_job.isOffer) ...<Widget>[
                    GoklayButton(
                      label: strings.acceptJob,
                      expand: true,
                      onPressed: _runner.isBusy
                          ? null
                          : () => _move(
                              () => dependencies.partner.accept(_job.id),
                              path: '/v1/partner/jobs/${_job.id}/accept',
                            ),
                    ),
                    const SizedBox(height: GoklaySpacing.sm),
                    GoklayButton(
                      label: strings.declineJob,
                      variant: GoklayButtonVariant.outlined,
                      expand: true,
                      onPressed: _runner.isBusy
                          ? null
                          : () => _move(
                              () => dependencies.partner.decline(_job.id),
                              path: '/v1/partner/jobs/${_job.id}/decline',
                            ),
                    ),
                  ],
                  if (_job.awaitsCollection)
                    GoklayButton(
                      label: strings.collect,
                      expand: true,
                      onPressed: _runner.isBusy
                          ? null
                          : () => _move(
                              () => dependencies.partner.collect(_job.id),
                              path: '/v1/partner/jobs/${_job.id}/collect',
                            ),
                    ),
                  if (_job.awaitsDelivery)
                    GoklayButton(
                      label: strings.deliver,
                      expand: true,
                      onPressed: _runner.isBusy
                          ? null
                          : () => _move(
                              () => dependencies.partner.deliver(_job.id),
                              path: '/v1/partner/jobs/${_job.id}/deliver',
                            ),
                    ),
                  if (_job.awaitsCollection || _job.awaitsDelivery) ...<Widget>[
                    const SizedBox(height: GoklaySpacing.sm),
                    LabelledField(
                      label: strings.failReason,
                      controller: _reason,
                      maxLines: 2,
                    ),
                    GoklayButton(
                      label: strings.failJob,
                      variant: GoklayButtonVariant.outlined,
                      expand: true,
                      onPressed:
                          _runner.isBusy || _reason.text.trim().isEmpty
                          ? null
                          : () => _move(
                              () => dependencies.partner.fail(
                                jobId: _job.id,
                                reason: _reason.text.trim(),
                              ),
                              path: '/v1/partner/jobs/${_job.id}/fail',
                              body: <String, Object?>{
                                'reason': _reason.text.trim(),
                              },
                            ),
                    ),
                  ],
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
