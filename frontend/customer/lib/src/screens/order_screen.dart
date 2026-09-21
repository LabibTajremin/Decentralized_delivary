import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/api_page.dart';
import '../api/models/cart.dart';
import '../api/models/order.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/customer_strings.dart';
import '../state/async_value.dart';
import '../state/store.dart';
import '../widgets/async_view.dart';
import '../widgets/messages.dart';
import '../widgets/receipt_view.dart';
import '../widgets/scaffold.dart';

/// One order: its timeline, its receipt, and whatever can still be done to it.
///
/// Two flags decide the whole screen and neither is inferred.
/// [Order.nextActions] is the set of transitions the server will currently
/// accept, and [OrderCancellation.allowed] is whether the free-cancellation
/// window is still open — with [OrderCancellation.text] already written for
/// when it is not. The countdown is displayed, not enforced: the screen
/// re-reads the endpoint rather than deciding for itself that the window has
/// shut.
class OrderScreen extends StatefulWidget {
  /// Creates the screen for [orderId].
  const OrderScreen({
    required this.orderId,
    required this.onTrack,
    required this.onPay,
    required this.onReview,
    required this.onSupport,
    super.key,
  });

  /// Which order.
  final String orderId;

  /// Called to open live tracking.
  final void Function(Order order) onTrack;

  /// Called to open the payment screen.
  final void Function(Order order) onPay;

  /// Called to leave a review.
  final void Function(Order order) onReview;

  /// Called to raise a support ticket.
  final void Function(Order order) onSupport;

  @override
  State<OrderScreen> createState() => _OrderScreenState();
}

class _OrderScreenState extends State<OrderScreen> {
  final Store<Order> _order = Store<Order>();
  final ActionRunner _runner = ActionRunner();
  OrderCancellation? _cancellation;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _order.dispose();
    _runner.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final Dependencies dependencies = AppScope.of(context);
    await _order.load(() async {
      final ApiPage<Order> page = await dependencies.orders.order(
        widget.orderId,
      );
      return page.toAsyncData();
    });
    await _loadCancellation();
  }

  Future<void> _loadCancellation() async {
    final Dependencies dependencies = AppScope.of(context);
    try {
      final ApiPage<OrderCancellation> page = await dependencies.orders
          .cancellation(widget.orderId);
      if (mounted) {
        setState(() => _cancellation = page.value);
      }
    } on ApiError {
      // The order itself loaded; not being able to say whether it can be
      // cancelled is not a reason to blank the receipt. The button simply
      // does not appear.
      if (mounted) {
        setState(() => _cancellation = null);
      }
    }
  }

  Future<void> _cancel() async {
    final Dependencies dependencies = AppScope.of(context);
    await _runner.run(() async {
      final Order updated = await dependencies.orders.cancel(
        orderId: widget.orderId,
      );
      _order.emit(AsyncData<Order>(updated));
    });
    await _loadCancellation();
  }

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    return CustomerScaffold(
      title: strings.orderTitle,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => AsyncView<Order>(
          store: _order,
          onRetry: _load,
          builder: (BuildContext context, Order order) =>
              _body(strings, order),
        ),
      ),
    );
  }

  Widget _body(CustomerStrings strings, Order order) {
    final OrderCancellation? cancellation = _cancellation;
    return ListView(
      children: <Widget>[
        Padding(
          padding: const EdgeInsets.all(GoklaySpacing.lg),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Text(order.code, style: GoklayTextStyles.title),
              const SizedBox(height: GoklaySpacing.xxs),
              Text(
                order.statusLabel,
                style: GoklayTextStyles.body.copyWith(
                  color: order.live
                      ? GoklayColors.brand
                      : GoklayColors.textSecondary,
                ),
              ),
            ],
          ),
        ),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: GoklaySpacing.lg),
          child: Wrap(
            spacing: GoklaySpacing.sm,
            runSpacing: GoklaySpacing.sm,
            children: <Widget>[
              if (order.awaitsPayment)
                GoklayButton(
                  label: strings.payNow,
                  onPressed: () => widget.onPay(order),
                ),
              if (order.isTrackable)
                GoklayButton(
                  label: strings.trackOrder,
                  variant: GoklayButtonVariant.outlined,
                  onPressed: () => widget.onTrack(order),
                ),
              if (!order.live)
                GoklayButton(
                  label: strings.leaveReview,
                  variant: GoklayButtonVariant.outlined,
                  onPressed: () => widget.onReview(order),
                ),
              GoklayButton(
                label: strings.getHelp,
                variant: GoklayButtonVariant.outlined,
                onPressed: () => widget.onSupport(order),
              ),
            ],
          ),
        ),
        if (cancellation != null) _cancelSection(strings, cancellation),
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
        SectionHeading(strings.deliveryAddress),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: GoklaySpacing.lg),
          child: Text(
            order.destination.singleLine,
            style: GoklayTextStyles.body,
          ),
        ),
        SectionHeading(strings.orderTimeline),
        for (final OrderEvent event in order.events)
          Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: GoklaySpacing.lg,
              vertical: GoklaySpacing.xxs,
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Text(event.label, style: GoklayTextStyles.body),
                if (event.reason.isNotEmpty)
                  Text(
                    event.reason,
                    style: GoklayTextStyles.caption.copyWith(
                      color: GoklayColors.textSecondary,
                    ),
                  ),
              ],
            ),
          ),
        SectionHeading(strings.cartTitle),
        for (final OrderLine line in order.lines)
          Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: GoklaySpacing.lg,
              vertical: GoklaySpacing.xxs,
            ),
            child: Row(
              children: <Widget>[
                Expanded(
                  child: Text(
                    '${line.quantity} × ${line.name}',
                    style: GoklayTextStyles.body,
                  ),
                ),
                GoklayMoneyText(line.lineTotal, style: GoklayTextStyles.body),
              ],
            ),
          ),
        SectionHeading(strings.receipt),
        Padding(
          padding: const EdgeInsets.fromLTRB(
            GoklaySpacing.lg,
            0,
            GoklaySpacing.lg,
            GoklaySpacing.xl,
          ),
          child: ReceiptView(
            rows: <ReceiptRow>[...order.receipt],
            total: order.total,
            totalLabel: strings.receipt,
          ),
        ),
      ],
    );
  }

  Widget _cancelSection(
    CustomerStrings strings,
    OrderCancellation cancellation,
  ) {
    return Padding(
      padding: const EdgeInsets.all(GoklaySpacing.lg),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          if (cancellation.text.isNotEmpty)
            Text(
              cancellation.text,
              style: GoklayTextStyles.caption.copyWith(
                color: GoklayColors.textSecondary,
              ),
            ),
          if (cancellation.allowed) ...<Widget>[
            const SizedBox(height: GoklaySpacing.sm),
            GoklayButton(
              label: strings.cancelOrder,
              variant: GoklayButtonVariant.outlined,
              onPressed: _runner.isBusy ? null : _cancel,
            ),
          ],
        ],
      ),
    );
  }
}
