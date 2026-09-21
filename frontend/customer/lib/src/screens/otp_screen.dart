import 'dart:async';

import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/auth.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/customer_strings.dart';
import '../state/store.dart';
import '../widgets/messages.dart';
import '../widgets/scaffold.dart';

/// `07__OTP Verification` (`1:181`) and `08__OTP Timeout | Resend` (`1:200`).
///
/// They are one screen with two states, and the state is the server's: the
/// countdown starts at [OtpChallenge.resendAfter] and "resend" appears when it
/// reaches zero. The app does not hold its own idea of how long a code lasts —
/// P04 states it per request, and an admin can change it without an app
/// release (2.9).
class OtpScreen extends StatefulWidget {
  /// Creates the screen.
  const OtpScreen({
    required this.phone,
    required this.challenge,
    required this.onVerified,
    required this.onFailed,
    super.key,
  });

  /// The number the code went to.
  final String phone;

  /// The server's challenge, which is where the countdown comes from.
  final OtpChallenge challenge;

  /// Called with the signed-in result once the tokens are stored.
  final void Function(AuthResult result) onVerified;

  /// Called when verification was refused, for the failure screen.
  final void Function(ApiError error) onFailed;

  @override
  State<OtpScreen> createState() => _OtpScreenState();
}

class _OtpScreenState extends State<OtpScreen> {
  final TextEditingController _code = TextEditingController();
  final ActionRunner _runner = ActionRunner();
  Timer? _ticker;
  late int _secondsLeft;

  /// The length P04 fixes the code at, and what the field accepts.
  static const int codeLength = 6;

  @override
  void initState() {
    super.initState();
    _code.addListener(_onTyped);
    _startCountdown(widget.challenge.resendAfter);
  }

  @override
  void dispose() {
    _ticker?.cancel();
    _code
      ..removeListener(_onTyped)
      ..dispose();
    _runner.dispose();
    super.dispose();
  }

  void _onTyped() => setState(() {});

  void _startCountdown(Duration from) {
    _ticker?.cancel();
    _secondsLeft = from.inSeconds;
    _ticker = Timer.periodic(const Duration(seconds: 1), (Timer timer) {
      if (_secondsLeft <= 0) {
        timer.cancel();
        return;
      }
      setState(() => _secondsLeft -= 1);
    });
  }

  Future<void> _verify() async {
    final Dependencies dependencies = AppScope.of(context);
    AuthResult? result;
    final bool ok = await _runner.run(() async {
      result = await dependencies.auth.verifyOtp(
        phone: widget.phone,
        code: _code.text.trim(),
      );
      await dependencies.session.adopt(result!.tokens);
    });
    if (!mounted) {
      return;
    }
    if (ok && result != null) {
      widget.onVerified(result!);
      return;
    }
    final ApiError? error = _runner.error;
    if (error != null) {
      widget.onFailed(error);
    }
  }

  Future<void> _resend() async {
    final bool ok = await _runner.run(() async {
      final OtpChallenge challenge = await AppScope.of(
        context,
      ).auth.requestOtp(widget.phone);
      if (mounted) {
        setState(() => _startCountdown(challenge.resendAfter));
      }
    });
    if (!ok && mounted) {
      setState(() {});
    }
  }

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    return CustomerScaffold(
      title: strings.otpTitle,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) {
          final ApiError? error = _runner.error;
          final bool canResend = _secondsLeft <= 0 && !_runner.isBusy;
          return Padding(
            padding: const EdgeInsets.all(GoklaySpacing.lg),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: <Widget>[
                Text(strings.otpSubtitle, style: GoklayTextStyles.body),
                const SizedBox(height: GoklaySpacing.lg),
                LabelledField(
                  label: strings.otpTitle,
                  controller: _code,
                  keyboardType: TextInputType.number,
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
                  label: strings.verifyCode,
                  expand: true,
                  onPressed:
                      _code.text.trim().length == codeLength && !_runner.isBusy
                      ? _verify
                      : null,
                ),
                const SizedBox(height: GoklaySpacing.md),
                if (canResend)
                  GoklayButton(
                    label: strings.resendCode,
                    variant: GoklayButtonVariant.outlined,
                    expand: true,
                    onPressed: _resend,
                  )
                else
                  Text(
                    '${strings.resendCountdown} $_secondsLeft',
                    textAlign: TextAlign.center,
                    style: GoklayTextStyles.caption.copyWith(
                      color: GoklayColors.textSecondary,
                    ),
                  ),
              ],
            ),
          );
        },
      ),
    );
  }
}
