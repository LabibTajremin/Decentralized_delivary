import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/cart.dart';
import '../api/models/catalogue.dart';
import '../api/models/discovery.dart';
import '../api/models/order.dart';
import '../l10n/customer_strings.dart';
import 'quantity_stepper.dart';

/// A rounded white card, the one container shape the design uses.
class GoklayCard extends StatelessWidget {
  /// Creates a card.
  const GoklayCard({required this.child, this.onTap, this.semanticLabel, super.key});

  /// What is in it.
  final Widget child;

  /// Called on tap. Null makes the card inert.
  final VoidCallback? onTap;

  /// What a screen reader calls it.
  final String? semanticLabel;

  @override
  Widget build(BuildContext context) {
    final Widget box = Container(
      margin: const EdgeInsets.symmetric(
        horizontal: GoklaySpacing.lg,
        vertical: GoklaySpacing.xs,
      ),
      padding: const EdgeInsets.all(GoklaySpacing.md),
      decoration: const BoxDecoration(
        color: GoklayColors.surfaceRaised,
        borderRadius: BorderRadius.all(GoklayRadii.card),
        boxShadow: GoklayElevation.card,
      ),
      child: child,
    );
    if (onTap == null) {
      return box;
    }
    return GoklayTapTarget(
      onTap: onTap,
      semanticLabel: semanticLabel,
      child: box,
    );
  }
}

/// One shop on the home screen.
///
/// Distance and opening hours arrive as finished strings, and whether the card
/// is enabled is [DiscoveryMerchant.isOpenNow] — the server's answer, folding
/// in the shop's hours and its holiday mode. Nothing here is computed from a
/// timestamp.
class ShopCard extends StatelessWidget {
  /// Creates a shop card.
  const ShopCard({required this.merchant, required this.onTap, super.key});

  /// The shop.
  final DiscoveryMerchant merchant;

  /// Called on tap.
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GoklayCard(
      onTap: onTap,
      semanticLabel: merchant.name,
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Text(merchant.name, style: GoklayTextStyles.emphasis),
                const SizedBox(height: GoklaySpacing.xxs),
                Text(
                  merchant.areaName,
                  style: GoklayTextStyles.caption.copyWith(
                    color: GoklayColors.textSecondary,
                  ),
                ),
                const SizedBox(height: GoklaySpacing.xxs),
                Text(
                  merchant.openStatus,
                  style: GoklayTextStyles.caption.copyWith(
                    color: merchant.isOpenNow
                        ? GoklayColors.brand
                        : GoklayColors.textSecondary,
                  ),
                ),
              ],
            ),
          ),
          Column(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: <Widget>[
              Text(merchant.distance, style: GoklayTextStyles.caption),
              const SizedBox(height: GoklaySpacing.xxs),
              GoklayMoneyText(
                merchant.delivery.money,
                style: GoklayTextStyles.caption,
              ),
            ],
          ),
        ],
      ),
    );
  }
}

/// One item on a shop's menu.
///
/// [PublicItem.orderable] is the whole of the enabled state. The app does not
/// look at stock, at a schedule or at the clock: the server folded all three
/// together, and the cart endpoint will enforce the same answer.
class ItemTile extends StatelessWidget {
  /// Creates an item tile.
  const ItemTile({required this.item, required this.onTap, super.key});

  /// The item.
  final PublicItem item;

  /// Called on tap. Ignored when the item is not orderable.
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Opacity(
      opacity: item.orderable ? 1 : 0.5,
      child: GoklayCard(
        onTap: item.orderable ? onTap : null,
        semanticLabel: item.name,
        child: Row(
          children: <Widget>[
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Text(item.name, style: GoklayTextStyles.emphasis),
                  if (item.description.isNotEmpty) ...<Widget>[
                    const SizedBox(height: GoklaySpacing.xxs),
                    Text(
                      item.description,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: GoklayTextStyles.caption.copyWith(
                        color: GoklayColors.textSecondary,
                      ),
                    ),
                  ],
                ],
              ),
            ),
            const SizedBox(width: GoklaySpacing.md),
            GoklayMoneyText(item.price),
          ],
        ),
      ),
    );
  }
}

/// One line of the cart, with its stepper and whatever is wrong with it.
///
/// [CartLine.issueText] is the server's sentence about a price that moved or
/// stock that ran out between adding and checking out. The app shows it; it
/// does not decide when one applies.
class CartLineTile extends StatelessWidget {
  /// Creates a cart line.
  const CartLineTile({
    required this.line,
    required this.onQuantityChanged,
    required this.onRemove,
    this.enabled = true,
    super.key,
  });

  /// The line.
  final CartLine line;

  /// Called with a new quantity. Zero removes the line, which is the cart
  /// endpoint's own rule.
  final ValueChanged<int> onQuantityChanged;

  /// Called by the remove button.
  final VoidCallback onRemove;

  /// Whether the controls respond — false while a change is in flight.
  final bool enabled;

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    return GoklayCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            children: <Widget>[
              Expanded(
                child: Text(line.name, style: GoklayTextStyles.emphasis),
              ),
              GoklayMoneyText(line.lineTotal),
            ],
          ),
          for (final CartOption option in line.options)
            Text(
              option.name,
              style: GoklayTextStyles.caption.copyWith(
                color: GoklayColors.textSecondary,
              ),
            ),
          if (line.note.isNotEmpty)
            Text(line.note, style: GoklayTextStyles.caption),
          if (line.hasIssue)
            Padding(
              padding: const EdgeInsets.only(top: GoklaySpacing.xxs),
              child: Text(
                line.issueText,
                style: GoklayTextStyles.caption.copyWith(
                  color: GoklayColors.danger,
                ),
              ),
            ),
          const SizedBox(height: GoklaySpacing.sm),
          Row(
            children: <Widget>[
              QuantityStepper(
                quantity: line.quantity,
                minimum: 1,
                enabled: enabled,
                onChanged: onQuantityChanged,
              ),
              const Spacer(),
              GoklayTapTarget(
                onTap: enabled ? onRemove : null,
                semanticLabel: strings.removeLine,
                excludeChildSemantics: true,
                child: Text(
                  strings.removeLine,
                  style: GoklayTextStyles.caption.copyWith(
                    color: GoklayColors.danger,
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

/// One order in the activity list.
class OrderTile extends StatelessWidget {
  /// Creates an order tile.
  const OrderTile({required this.order, required this.onTap, super.key});

  /// The order.
  final Order order;

  /// Called on tap.
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GoklayCard(
      onTap: onTap,
      semanticLabel: order.code,
      child: Row(
        children: <Widget>[
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Text(order.code, style: GoklayTextStyles.emphasis),
                const SizedBox(height: GoklaySpacing.xxs),
                Text(
                  order.statusLabel,
                  style: GoklayTextStyles.caption.copyWith(
                    color: order.live
                        ? GoklayColors.brand
                        : GoklayColors.textSecondary,
                  ),
                ),
              ],
            ),
          ),
          GoklayMoneyText(order.total),
        ],
      ),
    );
  }
}
