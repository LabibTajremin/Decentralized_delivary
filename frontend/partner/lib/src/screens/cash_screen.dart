import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/ledger.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/partner_strings.dart';
import '../widgets/cards.dart';

/// The rider's cash-on-delivery ledger.
///
/// Both totals come from the server, which recomputes them from the
/// collections rather than storing them. The screen does not add [Ledger.held]
/// up to check: a rider and an office disagreeing about how much cash is in a
/// bag is exactly the argument a second calculation causes (2.9).
class CashScreen extends StatefulWidget {
  /// Creates the ledger screen.
  const CashScreen({this.showBack = true, super.key});

  /// Whether to draw a back button.
  final bool showBack;

  @override
  State<CashScreen> createState() => _CashScreenState();
}

class _CashScreenState extends State<CashScreen> {
  final Store<Ledger> _ledger = Store<Ledger>();

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _ledger.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final Dependencies dependencies = PartnerScope.of(context);
    await _ledger.load(() async {
      final ApiPage<Ledger> page = await dependencies.partner.ledger();
      return page.toAsyncData();
    });
  }

  @override
  Widget build(BuildContext context) {
    final PartnerStrings strings = PartnerLocalizations.of(context);
    return GoklayScaffold(
      title: strings.cashTitle,
      showBack: widget.showBack,
      body: AsyncView<Ledger>(
        store: _ledger,
        onRetry: _load,
        builder: (BuildContext context, Ledger ledger) => ListView(
          children: <Widget>[
            GoklayCard(
              child: Column(
                children: <Widget>[
                  Row(
                    children: <Widget>[
                      Expanded(
                        child: Text(
                          strings.outstanding,
                          style: GoklayTextStyles.emphasis,
                        ),
                      ),
                      GoklayMoneyText(ledger.outstanding),
                    ],
                  ),
                  const SizedBox(height: GoklaySpacing.xxs),
                  Row(
                    children: <Widget>[
                      Expanded(
                        child: Text(
                          strings.remitted,
                          style: GoklayTextStyles.body,
                        ),
                      ),
                      GoklayMoneyText(
                        ledger.remitted,
                        style: GoklayTextStyles.body,
                      ),
                    ],
                  ),
                ],
              ),
            ),
            SectionHeading(strings.heldCollections),
            if (ledger.isSettled)
              EmptyView(message: strings.settled)
            else
              for (final Collection collection in ledger.held)
                GoklayCard(
                  child: Row(
                    children: <Widget>[
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: <Widget>[
                            Text(
                              collection.orderId,
                              style: GoklayTextStyles.body,
                            ),
                            Text(
                              collection.statusLabel,
                              style: GoklayTextStyles.caption.copyWith(
                                color: GoklayColors.textSecondary,
                              ),
                            ),
                          ],
                        ),
                      ),
                      GoklayMoneyText(collection.amount),
                    ],
                  ),
                ),
          ],
        ),
      ),
    );
  }
}
