import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/order.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/merchant_strings.dart';
import '../widgets/cards.dart';

/// The order board.
///
/// Every button on it comes from `next_actions` — the order state machine's
/// answer for this shop, right now. A board with its own copy of the
/// transition table would offer Accept on an order the customer cancelled a
/// second ago, and the server would refuse it in front of the shopkeeper.
class BoardScreen extends StatefulWidget {
  /// Creates the board for [merchantId].
  const BoardScreen({
    required this.merchantId,
    required this.onOrderSelected,
    this.showBack = true,
    super.key,
  });

  /// Whose board.
  final String merchantId;

  /// Called with the order the shopkeeper tapped.
  final void Function(MerchantOrder order) onOrderSelected;

  /// Whether to draw a back button.
  final bool showBack;

  @override
  State<BoardScreen> createState() => _BoardScreenState();
}

class _BoardScreenState extends State<BoardScreen> {
  final Store<MerchantOrderList> _orders = Store<MerchantOrderList>();
  final ActionRunner _runner = ActionRunner();
  bool _live = true;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _orders.dispose();
    _runner.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final Dependencies dependencies = MerchantScope.of(context);
    await _orders.load(() async {
      final ApiPage<MerchantOrderList> page = await dependencies.orders.list(
        merchantId: widget.merchantId,
        live: _live,
      );
      return page.toAsyncData();
    });
  }

  Future<void> _select(bool live) async {
    setState(() => _live = live);
    await _load();
  }

  Future<void> _move(Future<MerchantOrder> Function() call) async {
    await _runner.run(() async {
      await call();
    });
    await _load();
  }

  @override
  Widget build(BuildContext context) {
    final MerchantStrings strings = MerchantLocalizations.of(context);
    return GoklayScaffold(
      title: strings.boardTitle,
      showBack: widget.showBack,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => Column(
          children: <Widget>[
            Row(
              children: <Widget>[
                Expanded(
                  child: _Tab(
                    label: strings.boardLive,
                    selected: _live,
                    onTap: () => _select(true),
                  ),
                ),
                Expanded(
                  child: _Tab(
                    label: strings.boardPast,
                    selected: !_live,
                    onTap: () => _select(false),
                  ),
                ),
              ],
            ),
            if (_runner.error != null)
              Padding(
                padding: const EdgeInsets.all(GoklaySpacing.lg),
                child: Text(
                  ErrorView.messageFor(context, _runner.error!),
                  style: GoklayTextStyles.caption.copyWith(
                    color: GoklayColors.danger,
                  ),
                ),
              ),
            Expanded(
              child: AsyncView<MerchantOrderList>(
                store: _orders,
                onRetry: _load,
                builder:
                    (BuildContext context, MerchantOrderList orders) =>
                        orders.isEmpty
                        ? EmptyView(message: strings.boardEmpty)
                        : ListView(
                            children: <Widget>[
                              for (final MerchantOrder order in orders.orders)
                                _tile(strings, order),
                            ],
                          ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _tile(MerchantStrings strings, MerchantOrder order) {
    final Dependencies dependencies = MerchantScope.of(context);
    return GoklayCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          GoklayTapTarget(
            onTap: () => widget.onOrderSelected(order),
            semanticLabel: order.code,
            excludeChildSemantics: true,
            child: Row(
              children: <Widget>[
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: <Widget>[
                      Text(order.code, style: GoklayTextStyles.emphasis),
                      Text(
                        order.statusLabel,
                        style: GoklayTextStyles.caption.copyWith(
                          color: order.live
                              ? GoklayColors.brand
                              : GoklayColors.textSecondary,
                        ),
                      ),
                      Text(
                        order.paymentMethod == 'cash'
                            ? strings.payOnDelivery
                            : strings.paidOnline,
                        style: GoklayTextStyles.caption.copyWith(
                          color: GoklayColors.textSecondary,
                        ),
                      ),
                    ],
                  ),
                ),
                GoklayMoneyText(order.total),
              ],
            ),
          ),
          if (order.nextActions.isNotEmpty) ...<Widget>[
            const SizedBox(height: GoklaySpacing.sm),
            Wrap(
              spacing: GoklaySpacing.sm,
              runSpacing: GoklaySpacing.sm,
              children: <Widget>[
                if (order.allows(MerchantOrder.accepted))
                  GoklayButton(
                    label: strings.accept,
                    onPressed: _runner.isBusy
                        ? null
                        : () => _move(
                            () => dependencies.orders.accept(
                              merchantId: widget.merchantId,
                              orderId: order.id,
                            ),
                          ),
                  ),
                if (order.allows(MerchantOrder.preparing))
                  GoklayButton(
                    label: strings.startPreparing,
                    onPressed: _runner.isBusy
                        ? null
                        : () => _move(
                            () => dependencies.orders.preparing(
                              merchantId: widget.merchantId,
                              orderId: order.id,
                            ),
                          ),
                  ),
                if (order.allows(MerchantOrder.ready))
                  GoklayButton(
                    label: strings.markReady,
                    onPressed: _runner.isBusy
                        ? null
                        : () => _move(
                            () => dependencies.orders.ready(
                              merchantId: widget.merchantId,
                              orderId: order.id,
                            ),
                          ),
                  ),
                if (order.allows(MerchantOrder.rejected))
                  GoklayButton(
                    label: strings.reject,
                    variant: GoklayButtonVariant.outlined,
                    onPressed: _runner.isBusy
                        ? null
                        : () => widget.onOrderSelected(order),
                  ),
              ],
            ),
          ],
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
