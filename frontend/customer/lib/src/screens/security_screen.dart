import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/customer_strings.dart';
import '../widgets/cards.dart';

/// `07__Profile | Security` (`1:3446`), which the design draws empty.
///
/// What P04 has for it is the device list and "sign out everywhere" — the
/// thing somebody reaches for when they think their account is compromised,
/// and which deliberately does not require knowing which device was taken.
///
/// The list carries no token and no hash, only a label and a time. That is
/// enough to recognise a device, and a list of credentials would be a new
/// place to leak them from.
class SecurityScreen extends StatefulWidget {
  /// Creates the screen.
  const SecurityScreen({required this.onSignedOut, super.key});

  /// Called once every session has been revoked and the tokens cleared.
  final VoidCallback onSignedOut;

  @override
  State<SecurityScreen> createState() => _SecurityScreenState();
}

class _SecurityScreenState extends State<SecurityScreen> {
  final Store<List<DeviceSession>> _sessions = Store<List<DeviceSession>>();
  final ActionRunner _runner = ActionRunner();

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _sessions.dispose();
    _runner.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final Dependencies dependencies = AppScope.of(context);
    await _sessions.load(() async {
      final ApiPage<List<DeviceSession>> page = await dependencies.auth
          .sessions();
      return page.toAsyncData();
    });
  }

  Future<void> _signOutEverywhere() async {
    final Dependencies dependencies = AppScope.of(context);
    final bool ok = await _runner.run(() async {
      await dependencies.auth.logoutAll();
      await dependencies.signOut();
    });
    if (ok && mounted) {
      widget.onSignedOut();
    }
  }

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    return GoklayScaffold(
      title: strings.security,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => AsyncView<List<DeviceSession>>(
          store: _sessions,
          onRetry: _load,
          builder: (BuildContext context, List<DeviceSession> sessions) =>
              ListView(
                children: <Widget>[
                  for (final DeviceSession session in sessions)
                    GoklayCard(
                      child: Row(
                        children: <Widget>[
                          Expanded(
                            child: Text(
                              session.device,
                              style: GoklayTextStyles.body,
                            ),
                          ),
                          if (session.isCurrent)
                            Text(
                              strings.defaultAddress,
                              style: GoklayTextStyles.caption.copyWith(
                                color: GoklayColors.brand,
                              ),
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
      bottom: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => GoklayButton(
          label: strings.signOutEverywhere,
          variant: GoklayButtonVariant.outlined,
          expand: true,
          onPressed: _runner.isBusy ? null : _signOutEverywhere,
        ),
      ),
    );
  }
}
