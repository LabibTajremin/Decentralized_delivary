import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/api_page.dart';
import '../api/models/account.dart';
import '../api/models/cart.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/customer_strings.dart';
import '../state/store.dart';
import '../widgets/async_view.dart';
import '../widgets/cards.dart';
import '../widgets/messages.dart';
import '../widgets/receipt_view.dart';
import '../widgets/scaffold.dart';

/// `07__Review Cart` (`1:10108`).
///
/// Choosing the address is what makes the bill real: until the cart has a
/// destination there is no distance, and without a distance there is no
/// delivery fee to quote. So picking one is a `PUT /v1/cart/address` and the
/// receipt below redraws from the cart that comes back — the app never adjusts
/// a figure itself.
class ReviewCartScreen extends StatefulWidget {
  /// Creates the screen over [cart].
  const ReviewCartScreen({
    required this.cart,
    required this.onContinue,
    super.key,
  });

  /// The cart as the previous screen last saw it.
  final Cart cart;

  /// Called with the address-bound cart and the chosen address.
  final void Function(Cart cart, Address address) onContinue;

  @override
  State<ReviewCartScreen> createState() => _ReviewCartScreenState();
}

class _ReviewCartScreenState extends State<ReviewCartScreen> {
  final Store<List<Address>> _addresses = Store<List<Address>>();
  final ActionRunner _runner = ActionRunner();
  late Cart _cart = widget.cart;
  Address? _address;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _addresses.dispose();
    _runner.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final Dependencies dependencies = AppScope.of(context);
    await _addresses.load(() async {
      final ApiPage<List<Address>> page = await dependencies.account
          .addresses();
      return page.toAsyncData();
    });
    final List<Address>? loaded = _addresses.state.valueOrNull;
    if (loaded == null || loaded.isEmpty || !mounted) {
      return;
    }
    await _choose(
      loaded.firstWhere(
        (Address address) => address.id == _cart.addressId,
        orElse: () => loaded.firstWhere(
          (Address address) => address.isDefault,
          orElse: () => loaded.first,
        ),
      ),
    );
  }

  Future<void> _choose(Address address) async {
    final Dependencies dependencies = AppScope.of(context);
    await _runner.run(() async {
      final Cart updated = await dependencies.cart.setAddress(
        addressId: address.id,
        lat: address.lat,
        lng: address.lng,
      );
      if (mounted) {
        setState(() {
          _cart = updated;
          _address = address;
        });
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    final Address? chosen = _address;
    return CustomerScaffold(
      title: strings.reviewCartTitle,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => AsyncView<List<Address>>(
          store: _addresses,
          onRetry: _load,
          builder: (BuildContext context, List<Address> addresses) =>
              addresses.isEmpty
              ? EmptyView(message: strings.noAddresses)
              : _body(strings, addresses),
        ),
      ),
      bottom: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => GoklayButton(
          label: strings.placeOrder,
          expand: true,
          onPressed: chosen != null && _cart.orderable && !_runner.isBusy
              ? () => widget.onContinue(_cart, chosen)
              : null,
        ),
      ),
    );
  }

  Widget _body(CustomerStrings strings, List<Address> addresses) {
    return ListView(
      children: <Widget>[
        SectionHeading(strings.deliveryAddress),
        for (final Address address in addresses)
          GoklayCard(
            onTap: _runner.isBusy ? null : () => _choose(address),
            semanticLabel: address.label,
            child: Row(
              children: <Widget>[
                Icon(
                  _address?.id == address.id
                      ? Icons.check_circle
                      : Icons.circle_outlined,
                  size: 20,
                  color: _address?.id == address.id
                      ? GoklayColors.brand
                      : GoklayColors.textSecondary,
                ),
                const SizedBox(width: GoklaySpacing.sm),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: <Widget>[
                      Text(address.label, style: GoklayTextStyles.emphasis),
                      Text(
                        address.singleLine,
                        style: GoklayTextStyles.caption.copyWith(
                          color: GoklayColors.textSecondary,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        SectionHeading(strings.cartTitle),
        for (final CartLine line in _cart.lines)
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
        if (!_cart.orderable && _cart.blockerText.isNotEmpty)
          Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: GoklaySpacing.lg,
            ),
            child: Text(
              _cart.blockerText,
              style: GoklayTextStyles.caption.copyWith(
                color: GoklayColors.danger,
              ),
            ),
          ),
        Padding(
          padding: const EdgeInsets.all(GoklaySpacing.lg),
          child: ReceiptView(
            rows: _cart.pricing.rows,
            total: _cart.pricing.total,
            totalLabel: strings.receipt,
          ),
        ),
      ],
    );
  }
}
