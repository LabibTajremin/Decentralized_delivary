import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../l10n/customer_strings.dart';
import '../widgets/messages.dart';

/// `09__Verification Successfull` (`1:219`) and `10__Verification Failed`
/// (`1:261`) — one screen, told which it is.
class VerificationResultScreen extends StatelessWidget {
  /// The screen the design draws after a code is accepted.
  const VerificationResultScreen.success({required this.onContinue, super.key})
    : succeeded = true,
      error = null;

  /// The screen it draws after one is refused. [error] carries the server's
  /// own explanation, which is shown in place of the generic line whenever
  /// there is one.
  const VerificationResultScreen.failure({
    required this.onContinue,
    required this.error,
    super.key,
  }) : succeeded = false;

  /// Which of the two this is.
  final bool succeeded;

  /// What went wrong, on the failure screen.
  final ApiError? error;

  /// Called by the only button: on to the app, or back to the start.
  final VoidCallback onContinue;

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    final ApiError? failure = error;
    final String body = succeeded
        ? strings.verifiedBody
        : failure == null
        ? strings.verificationFailedBody
        : ErrorView.messageFor(context, failure);

    return Scaffold(
      backgroundColor: GoklayColors.surfaceRaised,
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(GoklaySpacing.xl),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: <Widget>[
              const Spacer(),
              Icon(
                succeeded ? Icons.check_circle : Icons.error_outline,
                size: 64,
                color: succeeded ? GoklayColors.brand : GoklayColors.danger,
              ),
              const SizedBox(height: GoklaySpacing.lg),
              Text(
                succeeded
                    ? strings.verifiedTitle
                    : strings.verificationFailedTitle,
                textAlign: TextAlign.center,
                style: GoklayTextStyles.title,
              ),
              const SizedBox(height: GoklaySpacing.sm),
              Text(
                body,
                textAlign: TextAlign.center,
                style: GoklayTextStyles.body,
              ),
              const Spacer(),
              GoklayButton(
                label: succeeded ? strings.getStarted : strings.startOver,
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
