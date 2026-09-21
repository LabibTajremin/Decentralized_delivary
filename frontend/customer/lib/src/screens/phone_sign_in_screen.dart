import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/auth.dart';
import '../app_scope.dart';
import '../l10n/customer_strings.dart';
import '../state/store.dart';
import '../widgets/messages.dart';
import '../widgets/scaffold.dart';

/// `06__Sign In With Mobile Number` (`1:142`).
///
/// The number is sent as typed. Whether it is a valid Bangladeshi mobile
/// number is decided by P04, which normalises it and answers `400` with a
/// sentence when it is not — so the app does not carry a second, slightly
/// different, regular expression that would reject numbers the server accepts.
/// The only check here is that the field is not empty, which is about not
/// making a pointless request rather than about validity.
class PhoneSignInScreen extends StatefulWidget {
  /// Creates the screen.
  const PhoneSignInScreen({required this.onCodeSent, super.key});

  /// Called with the number and the server's challenge once a code is on its
  /// way.
  final void Function(String phone, OtpChallenge challenge) onCodeSent;

  @override
  State<PhoneSignInScreen> createState() => _PhoneSignInScreenState();
}

class _PhoneSignInScreenState extends State<PhoneSignInScreen> {
  final TextEditingController _phone = TextEditingController();
  final ActionRunner _runner = ActionRunner();

  @override
  void initState() {
    super.initState();
    _phone.addListener(_onTyped);
  }

  @override
  void dispose() {
    _phone
      ..removeListener(_onTyped)
      ..dispose();
    _runner.dispose();
    super.dispose();
  }

  void _onTyped() => setState(() {});

  Future<void> _send() async {
    final String phone = _phone.text.trim();
    OtpChallenge? challenge;
    final bool sent = await _runner.run(() async {
      challenge = await AppScope.of(context).auth.requestOtp(phone);
    });
    if (sent && challenge != null && mounted) {
      widget.onCodeSent(phone, challenge!);
    }
  }

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    return CustomerScaffold(
      title: strings.signInTitle,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) {
          final ApiError? error = _runner.error;
          return Padding(
            padding: const EdgeInsets.all(GoklaySpacing.lg),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: <Widget>[
                LabelledField(
                  label: strings.phoneLabel,
                  hint: strings.phoneHint,
                  controller: _phone,
                  keyboardType: TextInputType.phone,
                ),
                if (error != null)
                  Padding(
                    padding: const EdgeInsets.only(top: GoklaySpacing.sm),
                    child: Text(
                      ErrorView.messageFor(context, error),
                      style: GoklayTextStyles.caption.copyWith(
                        color: GoklayColors.danger,
                      ),
                    ),
                  ),
                const SizedBox(height: GoklaySpacing.lg),
                GoklayButton(
                  label: strings.sendCode,
                  expand: true,
                  onPressed: _phone.text.trim().isEmpty || _runner.isBusy
                      ? null
                      : _send,
                ),
              ],
            ),
          );
        },
      ),
    );
  }
}
