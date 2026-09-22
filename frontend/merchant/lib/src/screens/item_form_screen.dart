import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/catalogue.dart';
import '../app_scope.dart';
import '../l10n/merchant_strings.dart';

/// Adding an item.
///
/// **Which fields appear is the capabilities response, not the shop's type.**
/// A unit picker only where `requires_unit`; a prescription switch only where
/// `prescriptions`. A field this kind of shop does not have is *refused* by
/// the server rather than dropped, so an owner never silently loses something
/// they typed — which is why the form shows only what will be accepted.
///
/// The price is typed in minor units. That is deliberate: money crosses this
/// wire as an integer of poisha and nothing else, and a decimal field would
/// mean parsing one here — an arithmetic step, on money, in the client (2.9).
/// Everything the owner reads back is the server's formatted string.
class ItemFormScreen extends StatefulWidget {
  /// Creates the form.
  const ItemFormScreen({
    required this.merchantId,
    required this.capabilities,
    required this.categories,
    required this.onAdded,
    super.key,
  });

  /// Whose catalogue.
  final String merchantId;

  /// What this shop type may contain.
  final CatalogueCapabilities capabilities;

  /// The sections to file it under.
  final List<OwnerCategory> categories;

  /// Called with the item the server created.
  final void Function(OwnerItem item) onAdded;

  @override
  State<ItemFormScreen> createState() => _ItemFormScreenState();
}

class _ItemFormScreenState extends State<ItemFormScreen> {
  final ActionRunner _runner = ActionRunner();
  final TextEditingController _name = TextEditingController();
  final TextEditingController _description = TextEditingController();
  final TextEditingController _price = TextEditingController();
  final TextEditingController _packSize = TextEditingController();
  final TextEditingController _brand = TextEditingController();
  late String _categoryId = widget.categories.isEmpty
      ? ''
      : widget.categories.first.id;
  late String _unit = widget.capabilities.units.isEmpty
      ? ''
      : widget.capabilities.units.first;
  bool _requiresPrescription = false;

  @override
  void initState() {
    super.initState();
    _name.addListener(_onTyped);
    _price.addListener(_onTyped);
  }

  @override
  void dispose() {
    _name.removeListener(_onTyped);
    _price.removeListener(_onTyped);
    for (final TextEditingController controller in <TextEditingController>[
      _name,
      _description,
      _price,
      _packSize,
      _brand,
    ]) {
      controller.dispose();
    }
    _runner.dispose();
    super.dispose();
  }

  void _onTyped() => setState(() {});

  int? get _priceMinor => int.tryParse(_price.text.trim());

  bool get _canSave =>
      _name.text.trim().isNotEmpty &&
      _priceMinor != null &&
      _categoryId.isNotEmpty;

  Future<void> _add() async {
    OwnerItem? saved;
    final bool ok = await _runner.run(() async {
      saved = await MerchantScope.of(context).catalogue.addItem(
        merchantId: widget.merchantId,
        categoryId: _categoryId,
        name: _name.text.trim(),
        priceMinor: _priceMinor!,
        description: _description.text.trim(),
        unit: widget.capabilities.requiresUnit ? _unit : '',
        packSize: _packSize.text.trim(),
        brand: _brand.text.trim(),
        requiresPrescription:
            widget.capabilities.prescriptions && _requiresPrescription,
      );
    });
    if (ok && saved != null && mounted) {
      widget.onAdded(saved!);
    }
  }

  @override
  Widget build(BuildContext context) {
    final MerchantStrings strings = MerchantLocalizations.of(context);
    return GoklayScaffold(
      title: strings.addItem,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => ListView(
          padding: const EdgeInsets.all(GoklaySpacing.lg),
          children: <Widget>[
            Text(
              strings.sections,
              style: GoklayTextStyles.caption.copyWith(
                color: GoklayColors.textSecondary,
              ),
            ),
            for (final OwnerCategory category in widget.categories)
              _Choice(
                label: category.name,
                selected: _categoryId == category.id,
                onTap: _runner.isBusy
                    ? null
                    : () => setState(() => _categoryId = category.id),
              ),
            LabelledField(label: strings.itemName, controller: _name),
            LabelledField(
              label: strings.itemDescription,
              controller: _description,
              maxLines: 2,
            ),
            LabelledField(
              label: strings.itemPriceMinor,
              controller: _price,
              keyboardType: TextInputType.number,
            ),
            if (widget.capabilities.requiresUnit) ...<Widget>[
              Text(
                strings.unitOfSale,
                style: GoklayTextStyles.caption.copyWith(
                  color: GoklayColors.textSecondary,
                ),
              ),
              for (final String unit in widget.capabilities.units)
                _Choice(
                  label: unit,
                  selected: _unit == unit,
                  onTap: _runner.isBusy
                      ? null
                      : () => setState(() => _unit = unit),
                ),
              LabelledField(
                label: strings.packSize,
                controller: _packSize,
              ),
              LabelledField(label: strings.brand, controller: _brand),
            ],
            if (widget.capabilities.prescriptions)
              _Choice(
                label: strings.requiresPrescription,
                selected: _requiresPrescription,
                onTap: _runner.isBusy
                    ? null
                    : () => setState(
                        () => _requiresPrescription = !_requiresPrescription,
                      ),
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
              label: strings.addItem,
              expand: true,
              onPressed: _canSave && !_runner.isBusy ? _add : null,
            ),
          ],
        ),
      ),
    );
  }
}

class _Choice extends StatelessWidget {
  const _Choice({
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
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: GoklaySpacing.xxs),
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
            Expanded(child: Text(label, style: GoklayTextStyles.body)),
          ],
        ),
      ),
    );
  }
}
