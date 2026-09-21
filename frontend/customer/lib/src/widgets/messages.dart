import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../l10n/customer_strings.dart';

/// A failure, said in the server's own words where there are any.
///
/// [ApiError.message] is already written for a person, in the caller's
/// language, so it is shown directly. A client-side map from [ApiError.code]
/// to a sentence would go stale the first time the backend added a code, and
/// the user would get "unknown error" for something the server explained
/// perfectly well.
class ErrorView extends StatelessWidget {
  /// Creates an error view.
  const ErrorView({required this.error, this.onRetry, super.key});

  /// What went wrong.
  final ApiError error;

  /// Called by the retry button. Null hides it.
  final Future<void> Function()? onRetry;

  /// The sentence to show: the server's, the offline banner, or the shared
  /// fallback — in that order.
  static String messageFor(BuildContext context, ApiError error) {
    final GoklayStrings strings = GoklayLocalizations.of(context);
    if (error.message.isNotEmpty) {
      return error.message;
    }
    return error.isOffline ? strings.offlineBanner : strings.unexpectedError;
  }

  @override
  Widget build(BuildContext context) {
    final GoklayStrings strings = GoklayLocalizations.of(context);
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(GoklaySpacing.xl),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            Text(
              messageFor(context, error),
              textAlign: TextAlign.center,
              style: GoklayTextStyles.body,
            ),
            if (onRetry != null) ...<Widget>[
              const SizedBox(height: GoklaySpacing.lg),
              GoklayButton(label: strings.retry, onPressed: onRetry),
            ],
          ],
        ),
      ),
    );
  }
}

/// A list with nothing in it, said plainly.
class EmptyView extends StatelessWidget {
  /// Creates an empty view. [message] defaults to the generic one.
  const EmptyView({this.message, super.key});

  /// What to say. Null uses `nothingHere`.
  final String? message;

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(GoklaySpacing.xl),
        child: Text(
          message ?? strings.nothingHere,
          textAlign: TextAlign.center,
          style: GoklayTextStyles.body.copyWith(
            color: GoklayColors.textSecondary,
          ),
        ),
      ),
    );
  }
}

/// A screen that the design draws but the backend has nothing behind.
///
/// Each one is listed in `docs/design-gaps.md` with the reason. It says so
/// rather than showing an invented balance, offer or referral code: a
/// plausible-looking fake is worse than an honest blank, because somebody
/// eventually believes it.
class NotAvailableView extends StatelessWidget {
  /// Creates the placeholder.
  const NotAvailableView({this.detail, super.key});

  /// An extra sentence, where there is something true to add.
  final String? detail;

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(GoklaySpacing.xl),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            Text(
              strings.notAvailableYet,
              textAlign: TextAlign.center,
              style: GoklayTextStyles.body,
            ),
            if (detail != null) ...<Widget>[
              const SizedBox(height: GoklaySpacing.sm),
              Text(
                detail!,
                textAlign: TextAlign.center,
                style: GoklayTextStyles.caption.copyWith(
                  color: GoklayColors.textSecondary,
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
