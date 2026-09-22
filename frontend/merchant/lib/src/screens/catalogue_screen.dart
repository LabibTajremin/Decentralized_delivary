import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/catalogue.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/merchant_strings.dart';
import '../widgets/cards.dart';

/// Everything the shop sells: its sections, its items, its bundles.
///
/// The screen's shape comes from `GET .../catalogue/capabilities`. A grocery
/// gets a shelf count; a restaurant does not. A pharmacy gets a prescription
/// switch; nothing else does. The bundles tab appears only where the server
/// says bundles exist. The app never branches on the shop's type itself —
/// which is what keeps the per-type schema rules in one place (2.9).
class CatalogueScreen extends StatefulWidget {
  /// Creates the screen for [merchantId].
  const CatalogueScreen({
    required this.merchantId,
    required this.onAddItem,
    this.showBack = true,
    super.key,
  });

  /// Whose catalogue.
  final String merchantId;

  /// Opens the item form. Answers true when one was added.
  final Future<bool> Function(CatalogueCapabilities capabilities) onAddItem;

  /// Whether to draw a back button.
  final bool showBack;

  @override
  State<CatalogueScreen> createState() => _CatalogueScreenState();
}

/// Which of the three lists is showing.
enum CatalogueTab {
  /// The shop's sections.
  sections,

  /// Its items.
  items,

  /// Its bundles.
  combos,
}

class _CatalogueScreenState extends State<CatalogueScreen> {
  final Store<CatalogueCapabilities> _capabilities =
      Store<CatalogueCapabilities>();
  final Store<List<OwnerCategory>> _categories = Store<List<OwnerCategory>>();
  final Store<List<OwnerItem>> _items = Store<List<OwnerItem>>();
  final Store<List<OwnerCombo>> _combos = Store<List<OwnerCombo>>();
  final ActionRunner _runner = ActionRunner();
  final TextEditingController _section = TextEditingController();
  CatalogueTab _tab = CatalogueTab.items;

  @override
  void initState() {
    super.initState();
    _section.addListener(_onTyped);
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _capabilities.dispose();
    _categories.dispose();
    _items.dispose();
    _combos.dispose();
    _section
      ..removeListener(_onTyped)
      ..dispose();
    _runner.dispose();
    super.dispose();
  }

  void _onTyped() => setState(() {});

  Future<void> _load() async {
    final Dependencies dependencies = MerchantScope.of(context);
    await _capabilities.load(() async {
      final ApiPage<CatalogueCapabilities> page = await dependencies.catalogue
          .capabilities(widget.merchantId);
      return page.toAsyncData();
    });
    await _reload();
  }

  Future<void> _reload() async {
    final Dependencies dependencies = MerchantScope.of(context);
    await _categories.load(() async {
      final ApiPage<List<OwnerCategory>> page = await dependencies.catalogue
          .categories(widget.merchantId);
      return page.toAsyncData();
    });
    await _items.load(() async {
      final ApiPage<List<OwnerItem>> page = await dependencies.catalogue.items(
        widget.merchantId,
      );
      return page.toAsyncData();
    });
    final CatalogueCapabilities? capabilities = _capabilities.state.valueOrNull;
    if (capabilities != null && capabilities.combos) {
      await _combos.load(() async {
        final ApiPage<List<OwnerCombo>> page = await dependencies.catalogue
            .combos(widget.merchantId);
        return page.toAsyncData();
      });
    }
  }

  Future<void> _change(Future<void> Function() call) async {
    await _runner.run(call);
    await _reload();
  }

  Future<void> _addSection() async {
    final Dependencies dependencies = MerchantScope.of(context);
    final String name = _section.text.trim();
    await _change(
      () => dependencies.catalogue.addCategory(
        merchantId: widget.merchantId,
        name: name,
      ),
    );
    if (mounted && _runner.error == null) {
      _section.clear();
    }
  }

  Future<void> _addItem(CatalogueCapabilities capabilities) async {
    final bool added = await widget.onAddItem(capabilities);
    if (added && mounted) {
      await _reload();
    }
  }

  @override
  Widget build(BuildContext context) {
    final MerchantStrings strings = MerchantLocalizations.of(context);
    return GoklayScaffold(
      title: strings.catalogueTitle,
      showBack: widget.showBack,
      body: AsyncView<CatalogueCapabilities>(
        store: _capabilities,
        onRetry: _load,
        builder:
            (BuildContext context, CatalogueCapabilities capabilities) =>
                ListenableBuilder(
                  listenable: _runner,
                  builder: (BuildContext context, _) =>
                      _body(strings, capabilities),
                ),
      ),
    );
  }

  Widget _body(MerchantStrings strings, CatalogueCapabilities capabilities) {
    return Column(
      children: <Widget>[
        Row(
          children: <Widget>[
            Expanded(
              child: _Tab(
                label: strings.sections,
                selected: _tab == CatalogueTab.sections,
                onTap: () => setState(() => _tab = CatalogueTab.sections),
              ),
            ),
            Expanded(
              child: _Tab(
                label: strings.items,
                selected: _tab == CatalogueTab.items,
                onTap: () => setState(() => _tab = CatalogueTab.items),
              ),
            ),
            if (capabilities.combos)
              Expanded(
                child: _Tab(
                  label: strings.combos,
                  selected: _tab == CatalogueTab.combos,
                  onTap: () => setState(() => _tab = CatalogueTab.combos),
                ),
              ),
          ],
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
        Expanded(
          child: switch (_tab) {
            CatalogueTab.sections => _sections(strings),
            CatalogueTab.items => _itemList(strings, capabilities),
            CatalogueTab.combos => _comboList(strings),
          },
        ),
      ],
    );
  }

  Widget _sections(MerchantStrings strings) {
    final Dependencies dependencies = MerchantScope.of(context);
    return AsyncView<List<OwnerCategory>>(
      store: _categories,
      onRetry: _reload,
      builder: (BuildContext context, List<OwnerCategory> categories) =>
          ListView(
            children: <Widget>[
              for (final OwnerCategory category in categories)
                GoklayCard(
                  child: Row(
                    children: <Widget>[
                      Expanded(
                        child: Text(
                          category.name,
                          style: GoklayTextStyles.emphasis,
                        ),
                      ),
                      GoklayTapTarget(
                        onTap: _runner.isBusy
                            ? null
                            : () => _change(
                                () => dependencies.catalogue
                                    .setCategoryActive(
                                      merchantId: widget.merchantId,
                                      categoryId: category.id,
                                      active: !category.active,
                                    ),
                              ),
                        semanticLabel: category.active
                            ? strings.hide
                            : strings.show,
                        excludeChildSemantics: true,
                        child: Text(
                          category.active ? strings.hide : strings.show,
                          style: GoklayTextStyles.caption.copyWith(
                            color: GoklayColors.brand,
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              Padding(
                padding: const EdgeInsets.all(GoklaySpacing.lg),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: <Widget>[
                    LabelledField(
                      label: strings.addSection,
                      controller: _section,
                    ),
                    const SizedBox(height: GoklaySpacing.sm),
                    GoklayButton(
                      label: strings.addSection,
                      expand: true,
                      onPressed:
                          _section.text.trim().isEmpty || _runner.isBusy
                          ? null
                          : _addSection,
                    ),
                  ],
                ),
              ),
            ],
          ),
    );
  }

  Widget _itemList(
    MerchantStrings strings,
    CatalogueCapabilities capabilities,
  ) {
    final Dependencies dependencies = MerchantScope.of(context);
    return AsyncView<List<OwnerItem>>(
      store: _items,
      onRetry: _reload,
      builder: (BuildContext context, List<OwnerItem> items) => ListView(
        children: <Widget>[
          if (items.isEmpty) EmptyView(message: strings.catalogueEmpty),
          for (final OwnerItem item in items)
            GoklayCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Row(
                    children: <Widget>[
                      Expanded(
                        child: Text(
                          item.name,
                          style: GoklayTextStyles.emphasis,
                        ),
                      ),
                      GoklayMoneyText(item.price),
                    ],
                  ),
                  if (!item.active)
                    Text(
                      strings.hidden,
                      style: GoklayTextStyles.caption.copyWith(
                        color: GoklayColors.textSecondary,
                      ),
                    )
                  else if (!item.orderable)
                    Text(
                      strings.notOrderable,
                      style: GoklayTextStyles.caption.copyWith(
                        color: GoklayColors.danger,
                      ),
                    ),
                  if (capabilities.tracksStock && item.stockTracked)
                    Row(
                      children: <Widget>[
                        Text(
                          strings.stockQuantity,
                          style: GoklayTextStyles.caption,
                        ),
                        const Spacer(),
                        QuantityStepper(
                          quantity: item.stockQuantity ?? 0,
                          minimum: 0,
                          maximum: 999,
                          enabled: !_runner.isBusy,
                          onChanged: (int quantity) => _change(
                            () => dependencies.catalogue.setStock(
                              merchantId: widget.merchantId,
                              itemId: item.id,
                              quantity: quantity,
                            ),
                          ),
                        ),
                      ],
                    ),
                  Align(
                    alignment: AlignmentDirectional.centerEnd,
                    child: GoklayTapTarget(
                      onTap: _runner.isBusy
                          ? null
                          : () => _change(
                              () => dependencies.catalogue.setItemActive(
                                merchantId: widget.merchantId,
                                itemId: item.id,
                                active: !item.active,
                              ),
                            ),
                      semanticLabel: item.active
                          ? strings.hide
                          : strings.show,
                      excludeChildSemantics: true,
                      child: Text(
                        item.active ? strings.hide : strings.show,
                        style: GoklayTextStyles.caption.copyWith(
                          color: GoklayColors.brand,
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),
          Padding(
            padding: const EdgeInsets.all(GoklaySpacing.lg),
            child: GoklayButton(
              label: strings.addItem,
              expand: true,
              onPressed: _runner.isBusy ? null : () => _addItem(capabilities),
            ),
          ),
        ],
      ),
    );
  }

  Widget _comboList(MerchantStrings strings) {
    final Dependencies dependencies = MerchantScope.of(context);
    return AsyncView<List<OwnerCombo>>(
      store: _combos,
      onRetry: _reload,
      builder: (BuildContext context, List<OwnerCombo> combos) => ListView(
        children: <Widget>[
          if (combos.isEmpty) EmptyView(message: strings.catalogueEmpty),
          for (final OwnerCombo combo in combos)
            GoklayCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Row(
                    children: <Widget>[
                      Expanded(
                        child: Text(
                          combo.name,
                          style: GoklayTextStyles.emphasis,
                        ),
                      ),
                      GoklayMoneyText(combo.price),
                    ],
                  ),
                  Text(
                    combo.lineNames.join(', '),
                    style: GoklayTextStyles.caption.copyWith(
                      color: GoklayColors.textSecondary,
                    ),
                  ),
                  if (!combo.orderable)
                    Text(
                      strings.notOrderable,
                      style: GoklayTextStyles.caption.copyWith(
                        color: GoklayColors.danger,
                      ),
                    ),
                  Align(
                    alignment: AlignmentDirectional.centerEnd,
                    child: GoklayTapTarget(
                      onTap: _runner.isBusy
                          ? null
                          : () => _change(
                              () => dependencies.catalogue.setComboActive(
                                merchantId: widget.merchantId,
                                comboId: combo.id,
                                active: !combo.active,
                              ),
                            ),
                      semanticLabel: combo.active
                          ? strings.hide
                          : strings.show,
                      excludeChildSemantics: true,
                      child: Text(
                        combo.active ? strings.hide : strings.show,
                        style: GoklayTextStyles.caption.copyWith(
                          color: GoklayColors.brand,
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),
        ],
      ),
    );
  }
}

class _Tab extends StatelessWidget {
  const _Tab({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GoklayTapTarget(
      onTap: onTap,
      semanticLabel: label,
      excludeChildSemantics: true,
      child: Container(
        alignment: Alignment.center,
        padding: const EdgeInsets.symmetric(vertical: GoklaySpacing.md),
        decoration: BoxDecoration(
          color: GoklayColors.surfaceRaised,
          border: Border(
            bottom: BorderSide(
              width: 2,
              color: selected
                  ? GoklayColors.brand
                  : GoklayColors.surfaceRaised,
            ),
          ),
        ),
        child: Text(
          label,
          style: GoklayTextStyles.body.copyWith(
            color: selected
                ? GoklayColors.brand
                : GoklayColors.textSecondary,
          ),
        ),
      ),
    );
  }
}
