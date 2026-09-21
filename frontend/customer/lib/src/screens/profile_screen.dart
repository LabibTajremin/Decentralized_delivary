import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/customer_strings.dart';

/// `05__Profile | Personal info` and `06__Profile Edit` (`1:3289`, `1:3394`),
/// which are one screen: the fields are editable and a save button commits
/// them.
///
/// Only the fields that changed are sent. `PATCH /v1/me` leaves an omitted
/// field alone, so editing a name cannot blank an email the customer set from
/// another device.
class ProfileScreen extends StatefulWidget {
  /// Creates the screen.
  const ProfileScreen({super.key});

  @override
  State<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends State<ProfileScreen> {
  final Store<Profile> _profile = Store<Profile>();
  final ActionRunner _runner = ActionRunner();
  final TextEditingController _name = TextEditingController();
  final TextEditingController _email = TextEditingController();
  bool _saved = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _profile.dispose();
    _runner.dispose();
    _name.dispose();
    _email.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final Dependencies dependencies = AppScope.of(context);
    await _profile.load(() async {
      final ApiPage<Profile> page = await dependencies.account.profile();
      return page.toAsyncData();
    });
    final Profile? loaded = _profile.state.valueOrNull;
    if (loaded != null) {
      _name.text = loaded.name;
      _email.text = loaded.email;
    }
  }

  Future<void> _save() async {
    final Dependencies dependencies = AppScope.of(context);
    setState(() => _saved = false);
    await _runner.run(() async {
      final Profile updated = await dependencies.account.updateProfile(
        name: _name.text.trim(),
        email: _email.text.trim(),
      );
      _profile.emit(AsyncData<Profile>(updated));
      if (mounted) {
        setState(() => _saved = true);
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    return GoklayScaffold(
      title: strings.personalInfo,
      body: AsyncView<Profile>(
        store: _profile,
        onRetry: _load,
        builder: (BuildContext context, Profile profile) => ListenableBuilder(
          listenable: _runner,
          builder: (BuildContext context, _) => ListView(
            padding: const EdgeInsets.all(GoklaySpacing.lg),
            children: <Widget>[
              Text(profile.displayName, style: GoklayTextStyles.title),
              const SizedBox(height: GoklaySpacing.lg),
              LabelledField(label: strings.nameLabel, controller: _name),
              LabelledField(
                label: strings.emailLabel,
                controller: _email,
                keyboardType: TextInputType.emailAddress,
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
              if (_saved)
                Padding(
                  padding: const EdgeInsets.only(top: GoklaySpacing.sm),
                  child: Text(
                    profile.displayName,
                    style: GoklayTextStyles.caption.copyWith(
                      color: GoklayColors.brand,
                    ),
                  ),
                ),
              const SizedBox(height: GoklaySpacing.lg),
              GoklayButton(
                label: strings.save,
                expand: true,
                onPressed: _runner.isBusy ? null : _save,
              ),
            ],
          ),
        ),
      ),
    );
  }
}
