import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/api_page.dart';
import '../api/models/cart.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/customer_strings.dart';
import '../state/async_value.dart';
import '../state/store.dart';
import '../widgets/async_view.dart';
import '../widgets/cards.dart';
import '../widgets/messages.dart';
import '../widgets/receipt_view.dart';
import '../widgets/scaffold.dart';

/// `05__View Cart` (`1:8568`).
///
/// Every change round-trips. The cart is revalidated against the live shop on
/// every read and write — a price can move, an item can sell out, the shop can
/// close — so the quantity on screen is the one the server just confirmed
/// rather than an optimistic local count a rejected change would leave wrong.
///
/// The checkout button reads [Cart.orderable] and nothing else. That flag
/// folds in the minimum order, the shop's hours, its holiday mode, stock, the
/// delivery address and D3's division rule; an app that tried to infer it
/// would get a different answer from the one checkout enforces (2.9).
class CartScreen extends StatefulWidget {
  /// Creates the cart.
  const CartScreen({required this.onCheckout, this.showBack = true, super.key});

  /// Called with the cart when the customer moves on to review it.
  final void Function(Cart cart) onCheckout;

  /// Whether to draw a back button.
  final bool showBack;

  @override
  State<CartScreen> createState() => _CartScreenState();
}

class _CartScreenState extends State<CartScreen> {
  final Store<Cart> _cart = Store<Cart>();
  final ActionRunner _runner = ActionRunner();

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _cart.dispose();
    _runner.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final Dependencies dependencies = AppScope.of(context);
    await _cart.load(() async {
      final ApiPage<Cart> page = await dependencies.cart.cart();
      return page.toAsyncData();
    });
  }

  Future<void> _change(Future<Cart> Function() call) async {
    await _runner.run(() async {
      final Cart updated = await call();
      _cart.emit(AsyncData<Cart>(updated));
    });
  }

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    final Dependencies dependencies = AppScope.of(context);
    return CustomerScaffold(
      title: strings.cartTitle,
      showBack: widget.showBack,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => AsyncView<Cart>(
          store: _cart,
          onRetry: _load,
          builder: (BuildContext context, Cart cart) => _body(
            strings: strings,
            cart: cart,
            onQuantity: (CartLine line, int quantity) => _change(
              () => dependencies.cart.setQuantity(
                lineId: line.id,
                quantity: quantity,
              ),
            ),
            onRemove: (CartLine line) =>
                _change(() => dependencies.cart.removeLine(line.id)),
          ),
        ),
      ),
      bottom: ListenableBuilder(
        listenable: Listenable.merge(<Listenable>[_cart, _runner]),
        builder: (BuildContext context, _) {
          final Cart? cart = _cart.state.valueOrNull;
          if (cart == null || cart.isEmpty) {
            return const SizedBox.shrink();
          }
          return Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            mainAxisSize: MainAxisSize.min,
            children: <Widget>[
              if (!cart.orderable && cart.blockerText.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.only(bottom: GoklaySpacing.sm),
                  child: Text(
                    cart.blockerText,
                    style: GoklayTextStyles.caption.copyWith(
                      color: GoklayColors.danger,
                    ),
                  ),
                ),
              GoklayButton(
                label: strings.reviewCart,
                expand: true,
                onPressed: _runner.isBusy
                    ? null
                    : () => widget.onCheckout(cart),
              ),
            ],
          );
        },
      ),
    );
  }

  Widget _body({
    required CustomerStrings strings,
    required Cart cart,
    required void Function(CartLine line, int quantity) onQuantity,
    required void Function(CartLine line) onRemove,
  }) {
    if (cart.isEmpty) {
      return EmptyView(message: strings.cartEmpty);
    }
    return ListView(
      children: <Widget>[
        Padding(
          padding: const EdgeInsets.fromLTRB(
            GoklaySpacing.lg,
            GoklaySpacing.lg,
            GoklaySpacing.lg,
            0,
          ),
          child: Text(cart.merchantName, style: GoklayTextStyles.emphasis),
        ),
        for (final CartLine line in cart.lines)
          CartLineTile(
            line: line,
            enabled: !_runner.isBusy,
            onQuantityChanged: (int quantity) => onQuantity(line, quantity),
            onRemove: () => onRemove(line),
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
        if (cart.pricing.notice.isNotEmpty)
          Padding(
            padding: const EdgeInsets.all(GoklaySpacing.lg),
            child: Text(
              cart.pricing.notice,
              style: GoklayTextStyles.caption.copyWith(
                color: GoklayColors.brand,
              ),
            ),
          ),
        Padding(
          padding: const EdgeInsets.all(GoklaySpacing.lg),
          child: ReceiptView(
            rows: cart.pricing.rows,
            total: cart.pricing.total,
            totalLabel: strings.receipt,
          ),
        ),
      ],
    );
  }
}
