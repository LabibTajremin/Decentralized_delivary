import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/payment.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/customer_strings.dart';

/// Paying for an order placed `online`.
///
/// `POST /v1/payments/checkout` is safe to call again: it resumes the attempt
/// the order already has rather than starting a second one, which is what
/// makes a customer who closed the payment page and came back an ordinary case
/// rather than a double charge.
///
/// The one gateway this product ships is the manual one, which records the
/// attempt and returns **no** `redirect_url` — a human confirms it through the
/// webhook. So the screen shows the payment's state and a way to re-read it,
/// and shows a link only when a gateway actually supplies one. It does not
/// launch anything: there is no browser plugin in this build, and inventing a
/// redirect for a gateway that did not give one would send the customer to a
/// blank page.
class PaymentScreen extends StatefulWidget {
  /// Creates the screen for [orderId].
  const PaymentScreen({required this.orderId, super.key});

  /// Which order is being paid for.
  final String orderId;

  @override
  State<PaymentScreen> createState() => _PaymentScreenState();
}

class _PaymentScreenState extends State<PaymentScreen> {
  final ActionRunner _runner = ActionRunner();
  Checkout? _checkout;
  Payment? _payment;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _startCheckout());
  }

  @override
  void dispose() {
    _runner.dispose();
    super.dispose();
  }

  Future<void> _startCheckout() async {
    final Dependencies dependencies = AppScope.of(context);
    await _runner.run(() async {
      final Checkout checkout = await dependencies.payments.checkout(
        widget.orderId,
      );
      if (mounted) {
        setState(() => _checkout = checkout);
      }
    });
  }

  /// Stands in for the bank, on a demo deployment that has none.
  ///
  /// Offered only when the checkout said [Checkout.demoCompletion] — the
  /// server's answer, not the app's guess — and followed immediately by a
  /// re-read, so what the screen shows afterwards is the payment's real
  /// state rather than an assumption that completing worked.
  Future<void> _completeDemo(String paymentId) async {
    final Dependencies dependencies = AppScope.of(context);
    await _runner.run(() async {
      await dependencies.payments.completeDemoPayment(paymentId);
      final ApiPage<Payment> page = await dependencies.payments.payment(
        widget.orderId,
      );
      if (mounted) {
        setState(() => _payment = page.value);
      }
    });
  }

  Future<void> _refresh() async {
    final Dependencies dependencies = AppScope.of(context);
    await _runner.run(() async {
      final ApiPage<Payment> page = await dependencies.payments.payment(
        widget.orderId,
      );
      if (mounted) {
        setState(() => _payment = page.value);
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    return GoklayScaffold(
      title: strings.paymentTitle,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) {
          final Checkout? checkout = _checkout;
          final Payment? payment = _payment;
          final ApiError? error = _runner.error;
          if (checkout == null && error != null) {
            return ErrorView(error: error, onRetry: _startCheckout);
          }
          if (checkout == null) {
            return const LoadingView();
          }
          return Padding(
            padding: const EdgeInsets.all(GoklaySpacing.lg),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: <Widget>[
                Row(
                  children: <Widget>[
                    Expanded(
                      child: Text(
                        payment?.statusLabel ?? checkout.statusLabel,
                        style: GoklayTextStyles.title,
                      ),
                    ),
                    GoklayMoneyText(payment?.amount ?? checkout.amount),
                  ],
                ),
                if (payment != null && payment.reason.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(top: GoklaySpacing.sm),
                    child: Text(
                      payment.reason,
                      style: GoklayTextStyles.body.copyWith(
                        color: GoklayColors.danger,
                      ),
                    ),
                  ),
                const SizedBox(height: GoklaySpacing.lg),
                if (checkout.hasRedirect)
                  SelectableText(
                    checkout.redirectUrl,
                    style: GoklayTextStyles.caption.copyWith(
                      color: GoklayColors.brand,
                    ),
                  )
                else
                  Text(
                    strings.noPaymentPage,
                    style: GoklayTextStyles.caption.copyWith(
                      color: GoklayColors.textSecondary,
                    ),
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
                if (checkout.demoCompletion && !(payment?.isCaptured ?? false))
                  Padding(
                    padding: const EdgeInsets.only(bottom: GoklaySpacing.md),
                    child: GoklayButton(
                      label: strings.completeDemoPayment,
                      expand: true,
                      onPressed: _runner.isBusy
                          ? null
                          : () => _completeDemo(checkout.paymentId),
                    ),
                  ),
                GoklayButton(
                  label: strings.checkPayment,
                  expand: true,
                  variant: checkout.demoCompletion
                      ? GoklayButtonVariant.outlined
                      : GoklayButtonVariant.filled,
                  onPressed: _runner.isBusy ? null : _refresh,
                ),
              ],
            ),
          );
        },
      ),
    );
  }
}
