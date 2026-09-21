import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../l10n/customer_strings.dart';

/// `05__Sign In Option` (`1:111`), with the one option this product has.
///
/// The design draws social sign-in buttons beside the phone one. P04 built a
/// single method — a phone number and a one-time code — and there is no OAuth
/// anywhere in the backend, so this offers what exists. Recorded in
/// `docs/design-gaps.md`.
class SignInOptionsScreen extends StatelessWidget {
  /// Creates the screen.
  const SignInOptionsScreen({required this.onContinue, super.key});

  /// Called to start phone sign-in.
  final VoidCallback onContinue;

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    return Scaffold(
      backgroundColor: GoklayColors.surfaceRaised,
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(GoklaySpacing.xl),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: <Widget>[
              const Spacer(),
              Text(strings.signInTitle, style: GoklayTextStyles.display),
              const SizedBox(height: GoklaySpacing.sm),
              Text(strings.signInSubtitle, style: GoklayTextStyles.body),
              const Spacer(),
              GoklayButton(
                label: strings.continueWithPhone,
                expand: true,
                onPressed: onContinue,
              ),
            ],
          ),
        ),
      ),
    );
  }
}
