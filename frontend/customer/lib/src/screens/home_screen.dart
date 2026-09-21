import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/api_page.dart';
import '../api/models/account.dart';
import '../api/models/discovery.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/customer_strings.dart';
import '../state/async_value.dart';
import '../state/store.dart';
import '../widgets/async_view.dart';
import '../widgets/cards.dart';
import '../widgets/messages.dart';
import '../widgets/scaffold.dart';

/// `01__Home Screen` (`1:280`, `1:859`, `1:1401` — the same screen with each
/// type chip selected).
///
/// The screen searches around the customer's default address. There is no
/// device-location plugin behind it, and that is deliberate: every figure on
/// a shop card — the distance, the delivery fee, whether the shop is in range
/// at all — is computed by discovery from a *delivery point*, and the delivery
/// point is an address the customer saved, not wherever the handset happens to
/// be standing.
///
/// Widening the radius is the one search parameter the customer controls, and
/// even that is not a number the app picks: it sends
/// [DiscoveryExpansion.nextLevel], only when [DiscoveryExpansion.canExpand] is
/// true, and it stops offering when [DiscoveryExpansion.atCeiling] says the
/// division boundary has been reached (D3).
class HomeScreen extends StatefulWidget {
  /// Creates the home screen.
  const HomeScreen({
    required this.onShopSelected,
    required this.onAddAddress,
    super.key,
  });

  /// Called with a shop the customer tapped.
  final void Function(DiscoveryMerchant merchant) onShopSelected;

  /// Called when there is no address to search from.
  final VoidCallback onAddAddress;

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  final Store<List<Address>> _addresses = Store<List<Address>>();
  final Store<DiscoverySearch> _search = Store<DiscoverySearch>();
  final TextEditingController _query = TextEditingController();

  Address? _address;
  String? _type;
  int _level = 0;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _loadAddresses());
  }

  @override
  void dispose() {
    _addresses.dispose();
    _search.dispose();
    _query.dispose();
    super.dispose();
  }

  Future<void> _loadAddresses() async {
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
    setState(() => _address = loaded.firstWhere(
      (Address address) => address.isDefault,
      orElse: () => loaded.first,
    ));
    await _runSearch();
  }

  Future<void> _runSearch() async {
    final Address? address = _address;
    if (address == null) {
      return;
    }
    final Dependencies dependencies = AppScope.of(context);
    await _search.load(() async {
      final ApiPage<DiscoverySearch> page = await dependencies.discovery.search(
        lat: address.lat,
        lng: address.lng,
        level: _level,
        type: _type,
        query: _query.text.trim(),
      );
      return page.toAsyncData();
    });
  }

  Future<void> _selectType(String? type) async {
    setState(() {
      _type = type;
      _level = 0;
    });
    await _runSearch();
  }

  Future<void> _expand(DiscoveryExpansion expansion) async {
    setState(() => _level = expansion.nextLevel);
    await _runSearch();
  }

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    return CustomerScaffold(
      title: 'GoKlay',
      showBack: false,
      body: ListenableBuilder(
        listenable: _addresses,
        builder: (BuildContext context, _) {
          final AsyncValue<List<Address>> state = _addresses.state;
          return switch (state) {
            AsyncLoading<List<Address>>() => const LoadingView(),
            AsyncFailure<List<Address>>(:final ApiError error) => ErrorView(
              error: error,
              onRetry: _loadAddresses,
            ),
            AsyncData<List<Address>>(:final List<Address> value) =>
              value.isEmpty ? _noAddress(strings) : _searchBody(strings),
          };
        },
      ),
    );
  }

  Widget _noAddress(CustomerStrings strings) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(GoklaySpacing.xl),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            Text(
              strings.noAddresses,
              textAlign: TextAlign.center,
              style: GoklayTextStyles.body,
            ),
            const SizedBox(height: GoklaySpacing.lg),
            GoklayButton(
              label: strings.addAddress,
              onPressed: widget.onAddAddress,
            ),
          ],
        ),
      ),
    );
  }

  Widget _searchBody(CustomerStrings strings) {
    final Address? address = _address;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: <Widget>[
        Padding(
          padding: const EdgeInsets.all(GoklaySpacing.lg),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Text(
                strings.deliverTo,
                style: GoklayTextStyles.caption.copyWith(
                  color: GoklayColors.textSecondary,
                ),
              ),
              if (address != null)
                Text(address.singleLine, style: GoklayTextStyles.emphasis),
              const SizedBox(height: GoklaySpacing.md),
              TextField(
                controller: _query,
                style: GoklayTextStyles.body,
                onSubmitted: (_) => _runSearch(),
                decoration: InputDecoration(hintText: strings.searchShops),
              ),
            ],
          ),
        ),
        SizedBox(
          height: GoklayA11y.minTapTarget,
          child: ListView(
            scrollDirection: Axis.horizontal,
            padding: const EdgeInsets.symmetric(
              horizontal: GoklaySpacing.lg,
            ),
            children: <Widget>[
              _TypeChip(
                label: strings.typeAll,
                selected: _type == null,
                onTap: () => _selectType(null),
              ),
              _TypeChip(
                label: strings.typeRestaurant,
                selected: _type == 'restaurant',
                onTap: () => _selectType('restaurant'),
              ),
              _TypeChip(
                label: strings.typeGrocery,
                selected: _type == 'grocery',
                onTap: () => _selectType('grocery'),
              ),
              _TypeChip(
                label: strings.typePharmacy,
                selected: _type == 'pharmacy',
                onTap: () => _selectType('pharmacy'),
              ),
            ],
          ),
        ),
        Expanded(
          child: AsyncView<DiscoverySearch>(
            store: _search,
            onRetry: _runSearch,
            builder: (BuildContext context, DiscoverySearch search) =>
                _results(strings, search),
          ),
        ),
      ],
    );
  }

  Widget _results(CustomerStrings strings, DiscoverySearch search) {
    final DiscoveryExpansion expansion = search.expansion;
    return ListView(
      children: <Widget>[
        if (search.notice.isNotEmpty)
          Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: GoklaySpacing.lg,
              vertical: GoklaySpacing.sm,
            ),
            child: Text(search.notice, style: GoklayTextStyles.caption),
          ),
        if (search.merchants.isEmpty) EmptyView(message: strings.noShops),
        for (final DiscoveryMerchant merchant in search.merchants)
          ShopCard(
            merchant: merchant,
            onTap: () => widget.onShopSelected(merchant),
          ),
        if (expansion.canExpand && !expansion.atCeiling)
          Padding(
            padding: const EdgeInsets.all(GoklaySpacing.lg),
            child: GoklayButton(
              label: strings.searchWider,
              variant: GoklayButtonVariant.outlined,
              expand: true,
              onPressed: () => _expand(expansion),
            ),
          ),
      ],
    );
  }
}

class _TypeChip extends StatelessWidget {
  const _TypeChip({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(right: GoklaySpacing.sm),
      child: GoklayTapTarget(
        onTap: onTap,
        semanticLabel: label,
        excludeChildSemantics: true,
        child: Container(
          alignment: Alignment.center,
          padding: const EdgeInsets.symmetric(
            horizontal: GoklaySpacing.lg,
            vertical: GoklaySpacing.sm,
          ),
          decoration: BoxDecoration(
            color: selected
                ? GoklayColors.brand
                : GoklayColors.surfaceRaised,
            borderRadius: const BorderRadius.all(GoklayRadii.pill),
          ),
          child: Text(
            label,
            style: GoklayTextStyles.caption.copyWith(
              color: selected
                  ? GoklayColors.onBrand
                  : GoklayColors.textPrimary,
            ),
          ),
        ),
      ),
    );
  }
}
