import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/cart.dart';
import '../api/models/order.dart';
import '../app_scope.dart';
import '../l10n/customer_strings.dart';

/// `08__Place Order` and `09__Place Order Active` (`1:10282`, `1:10457`) —
/// one screen, the second being its in-flight state.
///
/// Two payment methods, because P13 has two. There are no saved cards and no
/// wallet; `docs/design-gaps.md` records the frames that show them.
///
/// The idempotency key is generated once, when the screen is built, and reused
/// for every attempt from it. That is the whole defence against a double tap
/// or a retried request placing two orders: the server returns the order it
/// already made rather than making another.
class PlaceOrderScreen extends StatefulWidget {
  /// Creates the screen.
  const PlaceOrderScreen({
    required this.cart,
    required this.address,
    required this.onPlaced,
    super.key,
  });

  /// The cart being ordered, for the receipt.
  final Cart cart;

  /// Where it goes.
  final Address address;

  /// Called with the order the server created.
  final void Function(Order order) onPlaced;

  @override
  State<PlaceOrderScreen> createState() => _PlaceOrderScreenState();
}

class _PlaceOrderScreenState extends State<PlaceOrderScreen> {
  final ActionRunner _runner = ActionRunner();

  /// Cash on delivery.
  static const String cash = 'cash';

  /// Pay online.
  static const String online = 'online';

  String _method = cash;
  late final String _idempotencyKey = _keyFor(widget.cart);

  @override
  void dispose() {
    _runner.dispose();
    super.dispose();
  }

  /// A key that is stable for this attempt and different for the next cart.
  ///
  /// The cart id alone would be reused if the customer emptied the cart and
  /// built it again under the same id; the clock alone would change on a
  /// rebuild. Together they identify one attempt at one cart.
  static String _keyFor(Cart cart) =>
      '${cart.id}-${DateTime.now().microsecondsSinceEpoch}';

  Future<void> _place() async {
    Order? order;
    final bool placed = await _runner.run(() async {
      order = await AppScope.of(context).orders.place(
        addressId: widget.address.id,
        paymentMethod: _method,
        idempotencyKey: _idempotencyKey,
      );
    });
    if (placed && order != null && mounted) {
      widget.onPlaced(order!);
    }
  }

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    return GoklayScaffold(
      title: strings.placeOrderTitle,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => ListView(
          children: <Widget>[
            SectionHeading(strings.deliveryAddress),
            Padding(
              padding: const EdgeInsets.symmetric(
                horizontal: GoklaySpacing.lg,
              ),
              child: Text(
                widget.address.singleLine,
                style: GoklayTextStyles.body,
              ),
            ),
            SectionHeading(strings.paymentMethod),
            _MethodRow(
              label: strings.payCash,
              selected: _method == cash,
              onTap: _runner.isBusy
                  ? null
                  : () => setState(() => _method = cash),
            ),
            _MethodRow(
              label: strings.payOnline,
              selected: _method == online,
              onTap: _runner.isBusy
                  ? null
                  : () => setState(() => _method = online),
            ),
            SectionHeading(strings.receipt),
            Padding(
              padding: const EdgeInsets.symmetric(
                horizontal: GoklaySpacing.lg,
              ),
              child: ReceiptView(
                rows: widget.cart.pricing.rows,
                total: widget.cart.pricing.total,
                totalLabel: strings.receipt,
              ),
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
          ],
        ),
      ),
      bottom: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => GoklayButton(
          label: strings.placeOrder,
          expand: true,
          onPressed: _runner.isBusy ? null : _place,
        ),
      ),
    );
  }
}

class _MethodRow extends StatelessWidget {
  const _MethodRow({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final bool selected;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return GoklayTapTarget(
      onTap: onTap,
      semanticLabel: label,
      child: Container(
        color: GoklayColors.surfaceRaised,
        padding: const EdgeInsets.symmetric(
          horizontal: GoklaySpacing.lg,
          vertical: GoklaySpacing.md,
        ),
        child: Row(
          children: <Widget>[
            Icon(
              selected ? Icons.check_circle : Icons.circle_outlined,
              size: 20,
              color: selected
                  ? GoklayColors.brand
                  : GoklayColors.textSecondary,
            ),
            const SizedBox(width: GoklaySpacing.sm),
            Text(label, style: GoklayTextStyles.body),
          ],
        ),
      ),
    );
  }
}
