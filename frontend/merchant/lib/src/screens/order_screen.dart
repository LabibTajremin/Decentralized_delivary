import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/order.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/merchant_strings.dart';

/// One order, in the detail a kitchen needs: the lines, the notes, and the
/// transitions the server will currently accept.
///
/// The rejection reason is the shop's own words, typed here and shown to the
/// customer. It is not a code the app picks from a list, because the reason a
/// shop cannot fill an order is not something a list can anticipate.
class OrderScreen extends StatefulWidget {
  /// Creates the screen.
  const OrderScreen({
    required this.merchantId,
    required this.orderId,
    super.key,
  });

  /// Whose order.
  final String merchantId;

  /// Which order.
  final String orderId;

  @override
  State<OrderScreen> createState() => _OrderScreenState();
}

class _OrderScreenState extends State<OrderScreen> {
  final Store<MerchantOrder> _order = Store<MerchantOrder>();
  final ActionRunner _runner = ActionRunner();
  final TextEditingController _reason = TextEditingController();

  @override
  void initState() {
    super.initState();
    _reason.addListener(_onTyped);
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _order.dispose();
    _runner.dispose();
    _reason
      ..removeListener(_onTyped)
      ..dispose();
    super.dispose();
  }

  void _onTyped() => setState(() {});

  Future<void> _load() async {
    final Dependencies dependencies = MerchantScope.of(context);
    await _order.load(() async {
      final ApiPage<MerchantOrder> page = await dependencies.orders.order(
        merchantId: widget.merchantId,
        orderId: widget.orderId,
      );
      return page.toAsyncData();
    });
  }

  Future<void> _move(Future<MerchantOrder> Function() call) async {
    await _runner.run(() async {
      final MerchantOrder updated = await call();
      _order.emit(AsyncData<MerchantOrder>(updated));
    });
  }

  @override
  Widget build(BuildContext context) {
    final MerchantStrings strings = MerchantLocalizations.of(context);
    return GoklayScaffold(
      title: strings.boardTitle,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => AsyncView<MerchantOrder>(
          store: _order,
          onRetry: _load,
          builder: (BuildContext context, MerchantOrder order) =>
              _body(strings, order),
        ),
      ),
    );
  }

  Widget _body(MerchantStrings strings, MerchantOrder order) {
    final Dependencies dependencies = MerchantScope.of(context);
    return ListView(
      children: <Widget>[
        Padding(
          padding: const EdgeInsets.all(GoklaySpacing.lg),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Text(order.code, style: GoklayTextStyles.title),
              Text(order.statusLabel, style: GoklayTextStyles.body),
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
        SectionHeading(strings.orderLines),
        for (final OrderLine line in order.lines)
          Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: GoklaySpacing.lg,
              vertical: GoklaySpacing.xxs,
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Row(
                  children: <Widget>[
                    Expanded(
                      child: Text(
                        '${line.quantity} × ${line.name}',
                        style: GoklayTextStyles.body,
                      ),
                    ),
                    GoklayMoneyText(
                      line.lineTotal,
                      style: GoklayTextStyles.body,
                    ),
                  ],
                ),
                if (line.options.isNotEmpty)
                  Text(
                    line.options.join(', '),
                    style: GoklayTextStyles.caption.copyWith(
                      color: GoklayColors.textSecondary,
                    ),
                  ),
                if (line.note.isNotEmpty)
                  Text(
                    line.note,
                    style: GoklayTextStyles.caption.copyWith(
                      color: GoklayColors.brand,
                    ),
                  ),
              ],
            ),
          ),
        SectionHeading(strings.destination),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: GoklaySpacing.lg),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Text(order.destination.name, style: GoklayTextStyles.body),
              Text(
                order.destination.singleLine,
                style: GoklayTextStyles.caption.copyWith(
                  color: GoklayColors.textSecondary,
                ),
              ),
            ],
          ),
        ),
        SectionHeading(strings.receipt),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: GoklaySpacing.lg),
          child: ReceiptView(
            rows: order.receipt,
            total: order.total,
            totalLabel: strings.receipt,
          ),
        ),
        Padding(
          padding: const EdgeInsets.all(GoklaySpacing.lg),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: <Widget>[
              if (order.allows(MerchantOrder.accepted))
                GoklayButton(
                  label: strings.accept,
                  expand: true,
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
                  expand: true,
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
                  expand: true,
                  onPressed: _runner.isBusy
                      ? null
                      : () => _move(
                          () => dependencies.orders.ready(
                            merchantId: widget.merchantId,
                            orderId: order.id,
                          ),
                        ),
                ),
              if (order.allows(MerchantOrder.rejected)) ...<Widget>[
                const SizedBox(height: GoklaySpacing.sm),
                LabelledField(
                  label: strings.rejectReason,
                  controller: _reason,
                  maxLines: 2,
                ),
                GoklayButton(
                  label: strings.reject,
                  variant: GoklayButtonVariant.outlined,
                  expand: true,
                  onPressed:
                      _runner.isBusy || _reason.text.trim().isEmpty
                      ? null
                      : () => _move(
                          () => dependencies.orders.reject(
                            merchantId: widget.merchantId,
                            orderId: order.id,
                            reason: _reason.text.trim(),
                          ),
                        ),
                ),
              ],
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
            ],
          ),
        ),
      ],
    );
  }
}
