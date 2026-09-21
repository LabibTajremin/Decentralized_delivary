import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/api_page.dart';
import '../api/models/account.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/customer_strings.dart';
import '../state/store.dart';
import '../widgets/async_view.dart';
import '../widgets/cards.dart';
import '../widgets/messages.dart';
import '../widgets/scaffold.dart';

/// `12__Saved Address` (`1:3665`).
///
/// Each row shows [Address.singleLine] — the server's own one-line rendering,
/// which the receipt and the rider's screen also show. The app never joins
/// line1, line2 and the area itself, because a second implementation would
/// eventually join them differently.
class AddressesScreen extends StatefulWidget {
  /// Creates the address book.
  const AddressesScreen({required this.onAdd, super.key});

  /// Opens the add-address form. Returns true when one was saved.
  final Future<bool> Function() onAdd;

  @override
  State<AddressesScreen> createState() => _AddressesScreenState();
}

class _AddressesScreenState extends State<AddressesScreen> {
  final Store<List<Address>> _addresses = Store<List<Address>>();
  final ActionRunner _runner = ActionRunner();

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
  }

  Future<void> _add() async {
    final bool added = await widget.onAdd();
    if (added && mounted) {
      await _load();
    }
  }

  Future<void> _change(Future<void> Function() call) async {
    await _runner.run(call);
    await _load();
  }

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    final Dependencies dependencies = AppScope.of(context);
    return CustomerScaffold(
      title: strings.savedAddresses,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => AsyncView<List<Address>>(
          store: _addresses,
          onRetry: _load,
          builder: (BuildContext context, List<Address> addresses) =>
              addresses.isEmpty
              ? EmptyView(message: strings.noAddresses)
              : ListView(
                  children: <Widget>[
                    for (final Address address in addresses)
                      GoklayCard(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: <Widget>[
                            Row(
                              children: <Widget>[
                                Expanded(
                                  child: Text(
                                    address.label,
                                    style: GoklayTextStyles.emphasis,
                                  ),
                                ),
                                if (address.isDefault)
                                  Text(
                                    strings.defaultAddress,
                                    style: GoklayTextStyles.caption.copyWith(
                                      color: GoklayColors.brand,
                                    ),
                                  ),
                              ],
                            ),
                            Text(
                              address.singleLine,
                              style: GoklayTextStyles.caption.copyWith(
                                color: GoklayColors.textSecondary,
                              ),
                            ),
                            const SizedBox(height: GoklaySpacing.sm),
                            Row(
                              children: <Widget>[
                                if (!address.isDefault)
                                  GoklayTapTarget(
                                    onTap: _runner.isBusy
                                        ? null
                                        : () => _change(
                                            () => dependencies.account
                                                .makeDefault(address.id),
                                          ),
                                    semanticLabel: strings.makeDefault,
                                    excludeChildSemantics: true,
                                    child: Text(
                                      strings.makeDefault,
                                      style: GoklayTextStyles.caption
                                          .copyWith(
                                            color: GoklayColors.brand,
                                          ),
                                    ),
                                  ),
                                const Spacer(),
                                GoklayTapTarget(
                                  onTap: _runner.isBusy
                                      ? null
                                      : () => _change(
                                          () => dependencies.account
                                              .deleteAddress(address.id),
                                        ),
                                  semanticLabel: strings.deleteAddress,
                                  excludeChildSemantics: true,
                                  child: Text(
                                    strings.deleteAddress,
                                    style: GoklayTextStyles.caption.copyWith(
                                      color: GoklayColors.danger,
                                    ),
                                  ),
                                ),
                              ],
                            ),
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
                  ],
                ),
        ),
      ),
      bottom: GoklayButton(
        label: strings.addAddress,
        expand: true,
        onPressed: _add,
      ),
    );
  }
}
