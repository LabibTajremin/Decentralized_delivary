import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/endpoints/cart_api.dart';
import '../api/models/catalogue.dart';
import '../app_scope.dart';
import '../l10n/customer_strings.dart';

/// `03__More Details` / `04__Add to cart` (`1:11963`, `1:8519`).
///
/// The picker is shaped by the shop's own rules, which arrive on each
/// [OptionGroup]: required or not, and how many choices it takes. Shaping is
/// all the app does with them — `POST /v1/cart/items` checks the same rules
/// again and refuses a selection that breaks them, so an app that got the
/// shaping wrong produces a rejected request rather than a bad cart.
///
/// Nothing here adds a price up. The button says "add to cart"; what the line
/// costs is in the cart the server returns.
class ItemScreen extends StatefulWidget {
  /// Creates the screen for [item].
  const ItemScreen({required this.item, required this.onAdded, super.key});

  /// The item.
  final PublicItem item;

  /// Called once the server has accepted the line.
  final VoidCallback onAdded;

  @override
  State<ItemScreen> createState() => _ItemScreenState();
}

class _ItemScreenState extends State<ItemScreen> {
  final Map<String, Set<String>> _chosen = <String, Set<String>>{};
  final TextEditingController _note = TextEditingController();
  final ActionRunner _runner = ActionRunner();
  int _quantity = 1;

  @override
  void dispose() {
    _note.dispose();
    _runner.dispose();
    super.dispose();
  }

  /// Whether every required group has as many choices as it asks for.
  bool get _selectionIsComplete => widget.item.allGroups.every(
    (OptionGroup group) =>
        !group.isRequired ||
        (_chosen[group.id]?.length ?? 0) >= group.minChoices,
  );

  void _toggle(OptionGroup group, ItemOption option) {
    setState(() {
      final Set<String> selected = _chosen.putIfAbsent(
        group.id,
        () => <String>{},
      );
      if (selected.contains(option.id)) {
        selected.remove(option.id);
        return;
      }
      if (!group.isMultiSelect) {
        selected.clear();
      } else if (selected.length >= group.maxChoices) {
        return;
      }
      selected.add(option.id);
    });
  }

  Future<void> _add() async {
    final List<CartChoice> choices = <CartChoice>[
      for (final MapEntry<String, Set<String>> entry in _chosen.entries)
        for (final String optionId in entry.value)
          CartChoice(groupId: entry.key, optionId: optionId),
    ];
    final bool added = await _runner.run(() async {
      await AppScope.of(context).cart.add(
        merchantId: widget.item.merchantId,
        kind: 'item',
        targetId: widget.item.id,
        quantity: _quantity,
        choices: choices,
        note: _note.text.trim(),
      );
    });
    if (added && mounted) {
      widget.onAdded();
    }
  }

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    final PublicItem item = widget.item;
    return GoklayScaffold(
      title: item.name,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => ListView(
          padding: const EdgeInsets.all(GoklaySpacing.lg),
          children: <Widget>[
            Row(
              children: <Widget>[
                Expanded(
                  child: Text(item.name, style: GoklayTextStyles.title),
                ),
                GoklayMoneyText(item.price),
              ],
            ),
            if (item.description.isNotEmpty) ...<Widget>[
              const SizedBox(height: GoklaySpacing.sm),
              Text(item.description, style: GoklayTextStyles.body),
            ],
            for (final String detail in _details(item))
              Padding(
                padding: const EdgeInsets.only(top: GoklaySpacing.xxs),
                child: Text(
                  detail,
                  style: GoklayTextStyles.caption.copyWith(
                    color: GoklayColors.textSecondary,
                  ),
                ),
              ),
            for (final OptionGroup group in item.allGroups)
              _group(group),
            const SizedBox(height: GoklaySpacing.lg),
            LabelledField(
              label: strings.noteToShop,
              hint: strings.noteHint,
              controller: _note,
              maxLines: 2,
            ),
            const SizedBox(height: GoklaySpacing.lg),
            Row(
              children: <Widget>[
                Text(strings.quantity, style: GoklayTextStyles.emphasis),
                const Spacer(),
                QuantityStepper(
                  quantity: _quantity,
                  enabled: !_runner.isBusy,
                  onChanged: (int value) => setState(() => _quantity = value),
                ),
              ],
            ),
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
            const SizedBox(height: GoklaySpacing.lg),
            GoklayButton(
              label: strings.addToCart,
              expand: true,
              onPressed:
                  item.orderable && _selectionIsComplete && !_runner.isBusy
                  ? _add
                  : null,
            ),
          ],
        ),
      ),
    );
  }

  Widget _group(OptionGroup group) {
    final Set<String> selected = _chosen[group.id] ?? const <String>{};
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        SectionHeading(group.name),
        for (final ItemOption option in group.options)
          Opacity(
            opacity: option.available ? 1 : 0.4,
            child: GoklayTapTarget(
              onTap: option.available ? () => _toggle(group, option) : null,
              semanticLabel: option.name,
              child: Padding(
                padding: const EdgeInsets.symmetric(
                  vertical: GoklaySpacing.xxs,
                ),
                child: Row(
                  children: <Widget>[
                    Icon(
                      selected.contains(option.id)
                          ? Icons.check_circle
                          : Icons.circle_outlined,
                      size: 20,
                      color: selected.contains(option.id)
                          ? GoklayColors.brand
                          : GoklayColors.textSecondary,
                    ),
                    const SizedBox(width: GoklaySpacing.sm),
                    Expanded(
                      child: Text(option.name, style: GoklayTextStyles.body),
                    ),
                    GoklayMoneyText(
                      option.price,
                      style: GoklayTextStyles.caption,
                    ),
                  ],
                ),
              ),
            ),
          ),
      ],
    );
  }

  /// The pharmacy and grocery fields, shown only when the shop sent them.
  ///
  /// They are printed as they arrived. Nothing is converted, abbreviated or
  /// recomputed — a pack size is what the pack says.
  static List<String> _details(PublicItem item) => <String>[
    if (item.brand.isNotEmpty) item.brand,
    if (item.genericName.isNotEmpty) item.genericName,
    if (item.strength.isNotEmpty) item.strength,
    if (item.packSize.isNotEmpty) item.packSize,
    if (item.unit.isNotEmpty) item.unit,
  ];
}
