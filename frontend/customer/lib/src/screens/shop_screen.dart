import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/catalogue.dart';
import '../api/models/discovery.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/customer_strings.dart';
import '../widgets/cards.dart';

/// `03__Shop Details` (`1:8113`, and its pharmacy and grocery twins).
///
/// One screen for all three verticals. The design draws it three times because
/// the items differ; the screen does not, because nothing about it does.
class ShopScreen extends StatefulWidget {
  /// Creates the screen for [merchant].
  const ShopScreen({
    required this.merchant,
    required this.onItemSelected,
    required this.onReviews,
    super.key,
  });

  /// The shop, as discovery described it — which is where the delivery quote
  /// and the opening status on this screen come from.
  final DiscoveryMerchant merchant;

  /// Called with an item the customer tapped.
  final void Function(PublicItem item) onItemSelected;

  /// Called to open this shop's reviews.
  final VoidCallback onReviews;

  @override
  State<ShopScreen> createState() => _ShopScreenState();
}

class _ShopScreenState extends State<ShopScreen> {
  final Store<Menu> _menu = Store<Menu>();

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _menu.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final Dependencies dependencies = AppScope.of(context);
    await _menu.load(() async {
      final ApiPage<Menu> page = await dependencies.catalogue.menu(
        widget.merchant.id,
      );
      return page.toAsyncData();
    });
  }

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    return GoklayScaffold(
      title: widget.merchant.name,
      actions: <Widget>[
        GoklayTapTarget(
          onTap: widget.onReviews,
          semanticLabel: strings.reviews,
          excludeChildSemantics: true,
          child: Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: GoklaySpacing.sm,
            ),
            child: Text(strings.reviews, style: GoklayTextStyles.caption),
          ),
        ),
      ],
      body: AsyncView<Menu>(
        store: _menu,
        onRetry: _load,
        builder: (BuildContext context, Menu menu) => _body(strings, menu),
      ),
    );
  }

  Widget _body(CustomerStrings strings, Menu menu) {
    if (menu.isEmpty) {
      return const EmptyView();
    }
    return ListView(
      children: <Widget>[
        Padding(
          padding: const EdgeInsets.symmetric(
            horizontal: GoklaySpacing.lg,
            vertical: GoklaySpacing.sm,
          ),
          child: Text(
            widget.merchant.openStatus,
            style: GoklayTextStyles.caption.copyWith(
              color: widget.merchant.isOpenNow
                  ? GoklayColors.brand
                  : GoklayColors.danger,
            ),
          ),
        ),
        if (menu.combos.isNotEmpty) ...<Widget>[
          SectionHeading(strings.combos),
          for (final Combo combo in menu.combos)
            GoklayCard(
              child: Row(
                children: <Widget>[
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: <Widget>[
                        Text(combo.name, style: GoklayTextStyles.emphasis),
                        for (final ComboLine line in combo.lines)
                          Text(
                            line.name,
                            style: GoklayTextStyles.caption.copyWith(
                              color: GoklayColors.textSecondary,
                            ),
                          ),
                      ],
                    ),
                  ),
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: <Widget>[
                      GoklayMoneyText(combo.price),
                      if (combo.savings != null)
                        GoklayMoneyText(
                          combo.savings!,
                          style: GoklayTextStyles.caption.copyWith(
                            color: GoklayColors.brand,
                          ),
                        ),
                    ],
                  ),
                ],
              ),
            ),
        ],
        for (final Category category in menu.categories) ...<Widget>[
          SectionHeading(category.name),
          for (final PublicItem item in menu.itemsIn(category))
            ItemTile(
              item: item,
              onTap: () => widget.onItemSelected(item),
            ),
        ],
      ],
    );
  }
}
