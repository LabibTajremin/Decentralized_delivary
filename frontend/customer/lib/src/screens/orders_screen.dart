import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/endpoints/order_api.dart';
import '../api/models/order.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/customer_strings.dart';
import '../widgets/cards.dart';

/// `03__Activity` (`1:2059`).
///
/// Two tabs, and `live=true` is the server's own filter rather than a local
/// one. Which statuses count as "still going" is the order state machine's
/// business (P11) and changes there, not here.
class OrdersScreen extends StatefulWidget {
  /// Creates the orders list.
  const OrdersScreen({
    required this.onOrderSelected,
    this.showBack = true,
    super.key,
  });

  /// Called with the order the customer tapped.
  final void Function(Order order) onOrderSelected;

  /// Whether to draw a back button.
  final bool showBack;

  @override
  State<OrdersScreen> createState() => _OrdersScreenState();
}

class _OrdersScreenState extends State<OrdersScreen> {
  final Store<OrderList> _orders = Store<OrderList>();
  bool _live = true;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _orders.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final Dependencies dependencies = AppScope.of(context);
    await _orders.load(() async {
      final ApiPage<OrderList> page = await dependencies.orders.list(
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
    final CustomerStrings strings = CustomerLocalizations.of(context);
    return GoklayScaffold(
      title: strings.activityTitle,
      showBack: widget.showBack,
      body: Column(
        children: <Widget>[
          Row(
            children: <Widget>[
              Expanded(
                child: _Tab(
                  label: strings.ordersLive,
                  selected: _live,
                  onTap: () => _select(true),
                ),
              ),
              Expanded(
                child: _Tab(
                  label: strings.ordersPast,
                  selected: !_live,
                  onTap: () => _select(false),
                ),
              ),
            ],
          ),
          Expanded(
            child: AsyncView<OrderList>(
              store: _orders,
              onRetry: _load,
              builder: (BuildContext context, OrderList orders) =>
                  orders.isEmpty
                  ? EmptyView(message: strings.noOrders)
                  : ListView(
                      children: <Widget>[
                        for (final Order order in orders.orders)
                          OrderTile(
                            order: order,
                            onTap: () => widget.onOrderSelected(order),
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
