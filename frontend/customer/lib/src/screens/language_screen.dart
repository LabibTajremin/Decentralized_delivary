import 'package:flutter/material.dart';

import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/customer_strings.dart';
import '../state/store.dart';
import '../widgets/scaffold.dart';

/// `16__Language` (`1:3960`).
///
/// Changing the language does two things, and both matter. It switches the
/// widgets, and it switches the `lang` parameter on every subsequent request —
/// because most of what the customer reads is composed by the server (2.9), a
/// client that changed only its own strings would show a Bengali order status
/// under an English heading.
///
/// It is also saved to the profile, so the next device the customer signs in
/// on starts in the language they chose rather than the one the handset is
/// set to.
class LanguageScreen extends StatefulWidget {
  /// Creates the picker.
  const LanguageScreen({required this.onChanged, super.key});

  /// Called with the chosen locale, so the app can rebuild in it.
  final void Function(Locale locale) onChanged;

  @override
  State<LanguageScreen> createState() => _LanguageScreenState();
}

class _LanguageScreenState extends State<LanguageScreen> {
  final ActionRunner _runner = ActionRunner();

  @override
  void dispose() {
    _runner.dispose();
    super.dispose();
  }

  Future<void> _choose(String languageCode) async {
    final Dependencies dependencies = AppScope.of(context);
    final Locale locale = Locale(languageCode);
    dependencies.locale = locale;
    widget.onChanged(locale);
    await _runner.run(
      () => dependencies.account.updateProfile(language: languageCode),
    );
  }

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    final String current = strings.languageTag;
    return CustomerScaffold(
      title: strings.language,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => ListView(
          children: <Widget>[
            SettingRow(
              label: strings.bengali,
              value: current == 'bn' ? '✓' : null,
              onTap: _runner.isBusy ? null : () => _choose('bn'),
            ),
            SettingRow(
              label: strings.english,
              value: current == 'en' ? '✓' : null,
              onTap: _runner.isBusy ? null : () => _choose('en'),
            ),
          ],
        ),
      ),
    );
  }
}
