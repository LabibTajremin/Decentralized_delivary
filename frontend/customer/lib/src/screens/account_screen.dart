import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/api_page.dart';
import '../api/models/account.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/customer_strings.dart';
import '../state/store.dart';
import '../widgets/async_view.dart';
import '../widgets/scaffold.dart';

/// `04__Account` (`1:2917`).
///
/// The greeting is [Profile.displayName], which the server guarantees is never
/// empty and decides the fallback for. A customer can order without giving a
/// name, and two clients inventing two different greetings for the same
/// nameless customer is exactly the kind of drift 2.9 exists to stop.
class AccountScreen extends StatefulWidget {
  /// Creates the account screen.
  const AccountScreen({
    required this.onProfile,
    required this.onAddresses,
    required this.onSecurity,
    required this.onLanguage,
    required this.onNotifications,
    required this.onSupport,
    required this.onPlaceholder,
    required this.onSignedOut,
    this.showBack = true,
    super.key,
  });

  /// Opens the profile.
  final VoidCallback onProfile;

  /// Opens the address book.
  final VoidCallback onAddresses;

  /// Opens the device list.
  final VoidCallback onSecurity;

  /// Opens the language picker.
  final VoidCallback onLanguage;

  /// Opens the notification history.
  final VoidCallback onNotifications;

  /// Opens the support tickets.
  final VoidCallback onSupport;

  /// Opens one of the screens the backend has nothing behind, by its title.
  final void Function(PlaceholderScreen screen) onPlaceholder;

  /// Called once the tokens are cleared.
  final VoidCallback onSignedOut;

  /// Whether to draw a back button.
  final bool showBack;

  @override
  State<AccountScreen> createState() => _AccountScreenState();
}

/// The screens the design draws that have no endpoint behind them.
///
/// Listed in `docs/design-gaps.md`, each with why. They are named here rather
/// than described, so that adding a backend for one is a change in one place.
enum PlaceholderScreen {
  /// `02__Offers` — no promotions module exists.
  offers,

  /// `13__Promos` and `14__Add Promos` — no promo-code endpoint.
  promos,

  /// `15__Reffer & Get Discounts` — no referral module.
  referral,

  /// `10__Payment Methods` — P13 has two methods, chosen at checkout.
  paymentMethods,

  /// `18__GoKlay Safety` — static content with no endpoint.
  safety,

  /// `17__Permissions` — device permissions, not an API surface.
  permissions,
}

class _AccountScreenState extends State<AccountScreen> {
  final Store<Profile> _profile = Store<Profile>();
  final ActionRunner _runner = ActionRunner();

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _profile.dispose();
    _runner.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final Dependencies dependencies = AppScope.of(context);
    await _profile.load(() async {
      final ApiPage<Profile> page = await dependencies.account.profile();
      return page.toAsyncData();
    });
  }

  /// Signs out locally even when the server call fails.
  ///
  /// A customer who taps sign-out on a handset with no signal must still end
  /// up signed out on it. The server-side revocation is attempted first and
  /// its failure swallowed on purpose; the tokens and the cached screens go
  /// either way.
  Future<void> _signOut() async {
    final Dependencies dependencies = AppScope.of(context);
    await _runner.run(() async {
      try {
        await dependencies.auth.logout();
      } on ApiError {
        // Deliberately ignored — see above.
      }
      await dependencies.signOut();
    });
    if (mounted) {
      widget.onSignedOut();
    }
  }

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    return CustomerScaffold(
      title: strings.accountTitle,
      showBack: widget.showBack,
      body: AsyncView<Profile>(
        store: _profile,
        onRetry: _load,
        builder: (BuildContext context, Profile profile) => ListView(
          children: <Widget>[
            Padding(
              padding: const EdgeInsets.all(GoklaySpacing.lg),
              child: Text(
                profile.displayName,
                style: GoklayTextStyles.title,
              ),
            ),
            SettingRow(
              label: strings.personalInfo,
              onTap: widget.onProfile,
            ),
            SettingRow(
              label: strings.savedAddresses,
              onTap: widget.onAddresses,
            ),
            SettingRow(label: strings.security, onTap: widget.onSecurity),
            SettingRow(
              label: strings.language,
              value: profile.language,
              onTap: widget.onLanguage,
            ),
            SettingRow(
              label: strings.notifications,
              onTap: widget.onNotifications,
            ),
            SettingRow(label: strings.support, onTap: widget.onSupport),
            SettingRow(
              label: strings.offersTitle,
              onTap: () => widget.onPlaceholder(PlaceholderScreen.offers),
            ),
            SettingRow(
              label: strings.promosTitle,
              onTap: () => widget.onPlaceholder(PlaceholderScreen.promos),
            ),
            SettingRow(
              label: strings.referralTitle,
              onTap: () => widget.onPlaceholder(PlaceholderScreen.referral),
            ),
            SettingRow(
              label: strings.paymentMethodsTitle,
              onTap: () =>
                  widget.onPlaceholder(PlaceholderScreen.paymentMethods),
            ),
            SettingRow(
              label: strings.safetyTitle,
              onTap: () => widget.onPlaceholder(PlaceholderScreen.safety),
            ),
            SettingRow(
              label: strings.permissionsTitle,
              onTap: () => widget.onPlaceholder(PlaceholderScreen.permissions),
            ),
            Padding(
              padding: const EdgeInsets.all(GoklaySpacing.lg),
              child: ListenableBuilder(
                listenable: _runner,
                builder: (BuildContext context, _) => GoklayButton(
                  label: strings.signOut,
                  variant: GoklayButtonVariant.outlined,
                  expand: true,
                  onPressed: _runner.isBusy ? null : _signOut,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
