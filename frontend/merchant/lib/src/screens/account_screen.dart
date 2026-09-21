import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/merchant_strings.dart';

/// The owner's own account, as distinct from their shop.
class MerchantAccountScreen extends StatefulWidget {
  /// Creates the screen.
  const MerchantAccountScreen({
    required this.onLanguage,
    required this.onSignedOut,
    this.showBack = true,
    super.key,
  });

  /// Opens the language picker.
  final VoidCallback onLanguage;

  /// Called once the tokens are cleared.
  final VoidCallback onSignedOut;

  /// Whether to draw a back button.
  final bool showBack;

  @override
  State<MerchantAccountScreen> createState() => _MerchantAccountScreenState();
}

class _MerchantAccountScreenState extends State<MerchantAccountScreen> {
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
    final Dependencies dependencies = MerchantScope.of(context);
    await _profile.load(() async {
      final ApiPage<Profile> page = await dependencies.account.profile();
      return page.toAsyncData();
    });
  }

  /// Signs out locally even when the server call fails.
  ///
  /// A shopkeeper who taps sign-out on a handset with no signal must still end
  /// up signed out on it. The revocation is attempted and its failure
  /// swallowed on purpose; the tokens and the cached board go either way.
  Future<void> _signOut() async {
    final Dependencies dependencies = MerchantScope.of(context);
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
    final MerchantStrings strings = MerchantLocalizations.of(context);
    return GoklayScaffold(
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
              label: 'ভাষা / Language',
              value: profile.language,
              onTap: widget.onLanguage,
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

/// Switching the app's language, which switches the requests too.
class MerchantLanguageScreen extends StatefulWidget {
  /// Creates the picker.
  const MerchantLanguageScreen({required this.onChanged, super.key});

  /// Called with the chosen locale.
  final void Function(Locale locale) onChanged;

  @override
  State<MerchantLanguageScreen> createState() =>
      _MerchantLanguageScreenState();
}

class _MerchantLanguageScreenState extends State<MerchantLanguageScreen> {
  final ActionRunner _runner = ActionRunner();

  @override
  void dispose() {
    _runner.dispose();
    super.dispose();
  }

  Future<void> _choose(String languageCode) async {
    final Dependencies dependencies = MerchantScope.of(context);
    final Locale locale = Locale(languageCode);
    dependencies.locale = locale;
    widget.onChanged(locale);
    await _runner.run(
      () => dependencies.account.updateProfile(language: languageCode),
    );
  }

  @override
  Widget build(BuildContext context) {
    final MerchantStrings strings = MerchantLocalizations.of(context);
    return GoklayScaffold(
      title: 'ভাষা / Language',
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => ListView(
          children: <Widget>[
            SettingRow(
              label: 'বাংলা',
              value: strings.languageTag == 'bn' ? '✓' : null,
              onTap: _runner.isBusy ? null : () => _choose('bn'),
            ),
            SettingRow(
              label: 'English',
              value: strings.languageTag == 'en' ? '✓' : null,
              onTap: _runner.isBusy ? null : () => _choose('en'),
            ),
          ],
        ),
      ),
    );
  }
}
