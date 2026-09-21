import 'package:flutter/material.dart';

import '../api/models/merchant.dart';
import '../l10n/merchant_strings.dart';
import '../routes.dart';
import 'account_screen.dart';
import 'board_screen.dart';
import 'catalogue_screen.dart';
import 'shop_screen.dart';

/// The three tabs a shop with an approved record lives in.
class MerchantShell extends StatefulWidget {
  /// Creates the shell for [merchantId].
  const MerchantShell({
    required this.merchantId,
    required this.onSignedOut,
    required this.onLanguageChanged,
    super.key,
  });

  /// Whose shop.
  final String merchantId;

  /// Called when the owner signs out.
  final VoidCallback onSignedOut;

  /// Called when the language changes, so the whole app rebuilds in it.
  final void Function(Locale locale) onLanguageChanged;

  @override
  State<MerchantShell> createState() => _MerchantShellState();
}

class _MerchantShellState extends State<MerchantShell> {
  int _tab = 0;

  @override
  Widget build(BuildContext context) {
    final MerchantStrings strings = MerchantLocalizations.of(context);
    return Scaffold(
      body: IndexedStack(
        index: _tab,
        children: <Widget>[
          BoardScreen(
            merchantId: widget.merchantId,
            showBack: false,
            onOrderSelected: (order) => MerchantRoutes.order(
              context,
              merchantId: widget.merchantId,
              order: order,
            ),
          ),
          CatalogueScreen(
            merchantId: widget.merchantId,
            showBack: false,
            onAddItem: (capabilities) => MerchantRoutes.item(
              context,
              merchantId: widget.merchantId,
              capabilities: capabilities,
            ),
          ),
          ShopScreen(
            showBack: false,
            onEditDetails: (Merchant shop) =>
                MerchantRoutes.details(context, shop: shop),
            onHours: (Merchant shop) => MerchantRoutes.hours(context, shop),
            onHoliday: (Merchant shop) =>
                MerchantRoutes.holiday(context, shop),
            onAddDocument: (Merchant shop) =>
                MerchantRoutes.document(context, shop),
          ),
          MerchantAccountScreen(
            showBack: false,
            onLanguage: () => MerchantRoutes.language(
              context,
              onChanged: widget.onLanguageChanged,
            ),
            onSignedOut: widget.onSignedOut,
          ),
        ],
      ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _tab,
        onDestinationSelected: (int index) => setState(() => _tab = index),
        destinations: <Widget>[
          NavigationDestination(
            icon: const Icon(Icons.receipt_long_outlined),
            label: strings.boardTitle,
          ),
          NavigationDestination(
            icon: const Icon(Icons.menu_book_outlined),
            label: strings.catalogueTitle,
          ),
          NavigationDestination(
            icon: const Icon(Icons.storefront_outlined),
            label: strings.shopTitle,
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
