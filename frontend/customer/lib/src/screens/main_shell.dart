import 'package:flutter/material.dart';

import '../l10n/customer_strings.dart';
import '../routes.dart';
import 'account_screen.dart';
import 'cart_screen.dart';
import 'home_screen.dart';
import 'orders_screen.dart';

/// The four tabs a signed-in customer lives in.
///
/// The design's bottom bar is Home, Offers, Activity, Account. Offers has no
/// backend — `docs/design-gaps.md` says why — and spending one of four tab
/// slots on a screen that can only say "not available yet" would be worse than
/// moving it into Account, which is what this does. The cart takes the slot,
/// because it is the one screen the customer returns to constantly and the one
/// the design otherwise reaches only from inside a shop.
class MainShell extends StatefulWidget {
  /// Creates the shell.
  const MainShell({
    required this.onSignedOut,
    required this.onLanguageChanged,
    super.key,
  });

  /// Called when the customer signs out, from either the account screen or the
  /// device list.
  final VoidCallback onSignedOut;

  /// Called when the language changes, so the whole app rebuilds in it.
  final void Function(Locale locale) onLanguageChanged;

  @override
  State<MainShell> createState() => _MainShellState();
}

class _MainShellState extends State<MainShell> {
  int _tab = 0;

  /// Rebuilds the cart tab after something was added to it from a shop.
  ///
  /// The cart screen reads the server on build, so a new key is the whole
  /// mechanism: there is no local cart to keep in step, which is what stops
  /// the app and the server ever disagreeing about what is in it.
  int _cartGeneration = 0;

  void _cartChanged() => setState(() => _cartGeneration += 1);

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    return Scaffold(
      body: IndexedStack(
        index: _tab,
        children: <Widget>[
          HomeScreen(
            onShopSelected: (merchant) =>
                CustomerRoutes.shop(context, merchant, onCartChanged: _cartChanged),
            onAddAddress: () => CustomerRoutes.addAddress(context),
          ),
          CartScreen(
            key: ValueKey<int>(_cartGeneration),
            showBack: false,
            onCheckout: (cart) => CustomerRoutes.reviewCart(context, cart),
          ),
          OrdersScreen(
            showBack: false,
            onOrderSelected: (order) =>
                CustomerRoutes.order(context, order.id),
          ),
          AccountScreen(
            showBack: false,
            onProfile: () => CustomerRoutes.profile(context),
            onAddresses: () => CustomerRoutes.addresses(context),
            onSecurity: () => CustomerRoutes.security(
              context,
              onSignedOut: widget.onSignedOut,
            ),
            onLanguage: () => CustomerRoutes.language(
              context,
              onChanged: widget.onLanguageChanged,
            ),
            onNotifications: () => CustomerRoutes.notifications(context),
            onSupport: () => CustomerRoutes.support(context),
            onPlaceholder: (screen) =>
                CustomerRoutes.placeholder(context, screen),
            onSignedOut: widget.onSignedOut,
          ),
        ],
      ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _tab,
        onDestinationSelected: (int index) => setState(() => _tab = index),
        destinations: <Widget>[
          NavigationDestination(
            icon: const Icon(Icons.storefront_outlined),
            label: 'GoKlay',
          ),
          NavigationDestination(
            icon: const Icon(Icons.shopping_bag_outlined),
            label: strings.cartTitle,
          ),
          NavigationDestination(
            icon: const Icon(Icons.receipt_long_outlined),
            label: strings.activityTitle,
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
