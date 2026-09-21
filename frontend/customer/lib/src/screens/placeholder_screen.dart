import 'package:flutter/material.dart';

import '../l10n/customer_strings.dart';
import '../widgets/messages.dart';
import '../widgets/scaffold.dart';
import 'account_screen.dart';

/// The screens the Figma file draws that this backend has nothing behind.
///
/// Each says so, in the customer's language, and says nothing else. The
/// alternative — a plausible-looking wallet balance, a made-up referral code,
/// an offer nobody will honour — is worse than a blank, because somebody
/// eventually believes it and complains when it turns out not to be real.
///
/// `docs/design-gaps.md` lists all of them with the reason. Two of them have
/// something true to add, so they do: the payment screen can state the two
/// methods P13 really has, and the permissions screen can say what the app
/// will ask for and why.
class PlaceholderScreenView extends StatelessWidget {
  /// Creates the screen for [screen].
  const PlaceholderScreenView({required this.screen, super.key});

  /// Which one.
  final PlaceholderScreen screen;

  /// The heading for [screen], in the caller's language.
  static String titleOf(CustomerStrings strings, PlaceholderScreen screen) =>
      switch (screen) {
        PlaceholderScreen.offers => strings.offersTitle,
        PlaceholderScreen.promos => strings.promosTitle,
        PlaceholderScreen.referral => strings.referralTitle,
        PlaceholderScreen.paymentMethods => strings.paymentMethodsTitle,
        PlaceholderScreen.safety => strings.safetyTitle,
        PlaceholderScreen.permissions => strings.permissionsTitle,
      };

  /// The extra sentence for [screen], where there is a true one.
  static String? detailOf(CustomerStrings strings, PlaceholderScreen screen) =>
      switch (screen) {
        PlaceholderScreen.paymentMethods => strings.paymentMethodsBody,
        PlaceholderScreen.permissions =>
          '${strings.permissionLocation} ${strings.permissionNotifications}',
        _ => null,
      };

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    return CustomerScaffold(
      title: titleOf(strings, screen),
      body: NotAvailableView(detail: detailOf(strings, screen)),
    );
  }
}
