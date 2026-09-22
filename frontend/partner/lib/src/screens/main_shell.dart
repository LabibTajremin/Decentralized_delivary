import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/job.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/partner_strings.dart';
import '../widgets/cards.dart';
import 'account_screen.dart';
import 'cash_screen.dart';
import 'feed_screen.dart';
import 'job_screen.dart';
import 'jobs_screen.dart';

/// The four tabs a signed-up rider lives in, under the outbox banner.
///
/// The banner is above the tabs rather than inside one because the queue is
/// not a screen's business: a tap made on the job screen and still unsent has
/// to be visible from wherever the rider happens to be.
class PartnerShell extends StatefulWidget {
  /// Creates the shell.
  const PartnerShell({
    required this.onSignedOut,
    required this.onLanguageChanged,
    super.key,
  });

  /// Called when the rider signs out.
  final VoidCallback onSignedOut;

  /// Called when the language changes.
  final void Function(Locale locale) onLanguageChanged;

  @override
  State<PartnerShell> createState() => _PartnerShellState();
}

class _PartnerShellState extends State<PartnerShell> {
  final ActionRunner _flush = ActionRunner();
  int _tab = 0;
  FlushReport? _lastFlush;

  @override
  void dispose() {
    _flush.dispose();
    super.dispose();
  }

  /// Replays the outbox, oldest first.
  ///
  /// Refused actions come back in the report rather than staying queued: a
  /// `409` will be a `409` in an hour, and a queue that retried it forever
  /// would never drain. They are shown, because the rider believed they had
  /// happened.
  Future<void> _send() async {
    final Dependencies dependencies = PartnerScope.of(context);
    await _flush.run(() async {
      final FlushReport report = await dependencies.flushOutbox();
      if (mounted) {
        setState(() => _lastFlush = report);
      }
    });
  }

  void _openJob(DeliveryJob job) {
    Navigator.of(context).push<void>(
      MaterialPageRoute<void>(
        builder: (BuildContext context) => JobScreen(job: job),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final PartnerStrings strings = PartnerLocalizations.of(context);
    final Dependencies dependencies = PartnerScope.of(context);
    return Scaffold(
      body: Column(
        children: <Widget>[
          SafeArea(
            bottom: false,
            child: ListenableBuilder(
              listenable: _flush,
              builder: (BuildContext context, _) => Column(
                children: <Widget>[
                  OutboxBanner(
                    pending: dependencies.outbox.pending.length,
                    label: strings.waitingToSend,
                    sendLabel: strings.sendNow,
                    onSend: _flush.isBusy ? null : _send,
                  ),
                  if (_lastFlush != null && _lastFlush!.rejected.isNotEmpty)
                    Container(
                      width: double.infinity,
                      color: GoklayColors.surfaceRaised,
                      padding: const EdgeInsets.symmetric(
                        horizontal: GoklaySpacing.lg,
                        vertical: GoklaySpacing.sm,
                      ),
                      child: Text(
                        strings.queuedActionRefused,
                        style: GoklayTextStyles.caption.copyWith(
                          color: GoklayColors.danger,
                        ),
                      ),
                    ),
                ],
              ),
            ),
          ),
          Expanded(
            child: IndexedStack(
              index: _tab,
              children: <Widget>[
                FeedScreen(showBack: false, onJobSelected: _openJob),
                JobsScreen(showBack: false, onJobSelected: _openJob),
                const CashScreen(showBack: false),
                PartnerAccountScreen(
                  showBack: false,
                  onLanguageChanged: widget.onLanguageChanged,
                  onSignedOut: widget.onSignedOut,
                ),
              ],
            ),
          ),
        ],
      ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _tab,
        onDestinationSelected: (int index) => setState(() => _tab = index),
        destinations: <Widget>[
          NavigationDestination(
            icon: const Icon(Icons.explore_outlined),
            label: strings.feedTitle,
          ),
          NavigationDestination(
            icon: const Icon(Icons.local_shipping_outlined),
            label: strings.jobsTitle,
          ),
          NavigationDestination(
            icon: const Icon(Icons.payments_outlined),
            label: strings.cashTitle,
          ),
          NavigationDestination(
            icon: const Icon(Icons.person_outline),
            label: strings.accountTitle,
          ),
        ],
      ),
    );
  }
}
