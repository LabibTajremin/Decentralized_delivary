import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/api_page.dart';
import '../api/models/support.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/customer_strings.dart';
import '../state/store.dart';
import '../widgets/async_view.dart';
import '../widgets/cards.dart';
import '../widgets/messages.dart';
import '../widgets/scaffold.dart';

/// Support tickets: the customer's own, and a form to raise another.
///
/// A ticket is always against an order, which is why the form appears with one
/// in hand rather than as a free-standing contact page. P16 checks the caller
/// was on that order.
///
/// A resolved ticket says whether it was refunded, and stops there. **What**
/// was refunded is the payment's business and the payment endpoint states it;
/// an amount repeated here would be a second figure to disagree with the
/// first.
class SupportScreen extends StatefulWidget {
  /// Creates the screen. [orderId] shows the form for that order; null lists
  /// existing tickets alone.
  const SupportScreen({this.orderId, super.key});

  /// The order a new ticket would be about.
  final String? orderId;

  @override
  State<SupportScreen> createState() => _SupportScreenState();
}

class _SupportScreenState extends State<SupportScreen> {
  final Store<List<SupportTicket>> _tickets = Store<List<SupportTicket>>();
  final ActionRunner _runner = ActionRunner();
  final TextEditingController _subject = TextEditingController();

  @override
  void initState() {
    super.initState();
    _subject.addListener(_onTyped);
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _tickets.dispose();
    _runner.dispose();
    _subject
      ..removeListener(_onTyped)
      ..dispose();
    super.dispose();
  }

  void _onTyped() => setState(() {});

  Future<void> _load() async {
    final Dependencies dependencies = AppScope.of(context);
    await _tickets.load(() async {
      final ApiPage<List<SupportTicket>> page = await dependencies.support
          .tickets();
      return page.toAsyncData();
    });
  }

  Future<void> _raise() async {
    final String? orderId = widget.orderId;
    if (orderId == null) {
      return;
    }
    final Dependencies dependencies = AppScope.of(context);
    final bool ok = await _runner.run(() async {
      await dependencies.support.raiseTicket(
        orderId: orderId,
        subject: _subject.text.trim(),
      );
    });
    if (ok && mounted) {
      _subject.clear();
      await _load();
    }
  }

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    return CustomerScaffold(
      title: strings.supportTitle,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => AsyncView<List<SupportTicket>>(
          store: _tickets,
          onRetry: _load,
          builder: (BuildContext context, List<SupportTicket> tickets) =>
              ListView(
                children: <Widget>[
                  if (widget.orderId != null)
                    Padding(
                      padding: const EdgeInsets.all(GoklaySpacing.lg),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: <Widget>[
                          LabelledField(
                            label: strings.ticketSubject,
                            controller: _subject,
                            maxLines: 2,
                          ),
                          if (_runner.error != null)
                            Padding(
                              padding: const EdgeInsets.only(
                                top: GoklaySpacing.sm,
                              ),
                              child: Text(
                                ErrorView.messageFor(
                                  context,
                                  _runner.error!,
                                ),
                                style: GoklayTextStyles.caption.copyWith(
                                  color: GoklayColors.danger,
                                ),
                              ),
                            ),
                          const SizedBox(height: GoklaySpacing.sm),
                          GoklayButton(
                            label: strings.raiseTicket,
                            expand: true,
                            onPressed:
                                _subject.text.trim().isEmpty ||
                                    _runner.isBusy
                                ? null
                                : _raise,
                          ),
                        ],
                      ),
                    ),
                  if (tickets.isEmpty)
                    EmptyView(message: strings.noTickets)
                  else
                    for (final SupportTicket ticket in tickets)
                      GoklayCard(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: <Widget>[
                            Text(
                              ticket.subject,
                              style: GoklayTextStyles.emphasis,
                            ),
                            Text(
                              ticket.isOpen
                                  ? strings.ticketOpen
                                  : strings.ticketResolved,
                              style: GoklayTextStyles.caption.copyWith(
                                color: ticket.isOpen
                                    ? GoklayColors.brand
                                    : GoklayColors.textSecondary,
                              ),
                            ),
                            if (ticket.note.isNotEmpty)
                              Text(
                                ticket.note,
                                style: GoklayTextStyles.caption,
                              ),
                          ],
                        ),
                      ),
                ],
              ),
        ),
      ),
    );
  }
}
