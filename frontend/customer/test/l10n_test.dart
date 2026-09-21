
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_customer/src/l10n/customer_strings.dart';

/// Every getter on the table, so a string added to one language and forgotten
/// in the other cannot pass.
List<String> allStrings(CustomerStrings s) => <String>[
  s.languageTag,
  s.skip, s.next, s.getStarted,
  s.onboardingTitle1, s.onboardingBody1,
  s.onboardingTitle2, s.onboardingBody2,
  s.onboardingTitle3, s.onboardingBody3,
  s.signInTitle, s.signInSubtitle, s.continueWithPhone,
  s.phoneLabel, s.phoneHint, s.sendCode,
  s.otpTitle, s.otpSubtitle, s.verifyCode,
  s.resendCountdown, s.resendCode,
  s.verifiedTitle, s.verifiedBody,
  s.verificationFailedTitle, s.verificationFailedBody, s.startOver,
  s.deliverTo, s.searchShops,
  s.typeAll, s.typeRestaurant, s.typeGrocery, s.typePharmacy,
  s.searchWider, s.noShops,
  s.menu, s.combos, s.reviews, s.addToCart,
  s.noteToShop, s.noteHint, s.quantity,
  s.decreaseQuantity, s.increaseQuantity, s.removeLine,
  s.cartTitle, s.cartEmpty, s.reviewCart, s.reviewCartTitle,
  s.placeOrderTitle, s.placeOrder,
  s.paymentMethod, s.payCash, s.payOnline,
  s.deliveryAddress, s.chooseAddress, s.noAddressChosen,
  s.activityTitle, s.ordersLive, s.ordersPast, s.noOrders,
  s.orderTitle, s.orderTimeline, s.receipt,
  s.trackOrder, s.cancelOrder, s.cancelOrderConfirm,
  s.trackingTitle, s.callRider, s.callShop,
  s.awaitingRider, s.trackingEnded,
  s.paymentTitle, s.payNow, s.checkPayment, s.noPaymentPage,
  s.accountTitle, s.personalInfo, s.savedAddresses, s.security,
  s.language, s.notifications, s.support,
  s.editProfile, s.save, s.nameLabel, s.emailLabel,
  s.signOut, s.signOutEverywhere,
  s.addAddress, s.addressLabelField, s.recipientName, s.recipientPhone,
  s.addressLine1, s.addressLine2, s.addressInstructions,
  s.makeDefault, s.defaultAddress, s.deleteAddress,
  s.noAddresses, s.noNotifications, s.notificationFailed,
  s.bengali, s.english,
  s.leaveReview, s.reviewTitle, s.rateShop, s.rateRider,
  s.reviewComment, s.submitReview, s.reviewThanks,
  s.noReviews, s.noRatingYet,
  s.getHelp, s.supportTitle, s.ticketSubject, s.raiseTicket,
  s.ticketOpen, s.ticketResolved, s.noTickets,
  s.notAvailableYet, s.offersTitle, s.promosTitle, s.referralTitle,
  s.safetyTitle, s.permissionsTitle,
  s.paymentMethodsTitle, s.paymentMethodsBody,
  s.permissionLocation, s.permissionNotifications,
  s.back, s.confirm, s.showingSaved, s.nothingHere,
];

/// The Bengali block, `U+0980..U+09FF`.
bool hasBengali(String value) =>
    value.runes.any((int rune) => rune >= 0x0980 && rune <= 0x09FF);

void main() {
  const CustomerStringsBn bn = CustomerStringsBn();
  const CustomerStringsEn en = CustomerStringsEn();

  test('both tables answer every string', () {
    expect(allStrings(bn), hasLength(allStrings(en).length));
    expect(allStrings(bn).where((String s) => s.isEmpty), isEmpty);
    expect(allStrings(en).where((String s) => s.isEmpty), isEmpty);
  });

  test('the Bengali table is actually in Bengali script', () {
    // `bengali`, `english`, the phone placeholder and the language tag are
    // the four that are deliberately not: two are language names shown in
    // their own script, one is a number format, and one is a code.
    final Set<String> exempt = <String>{
      bn.languageTag,
      bn.phoneHint,
      bn.english,
    };
    final Iterable<String> latinOnly = allStrings(
      bn,
    ).where((String s) => !exempt.contains(s) && !hasBengali(s));
    expect(latinOnly, isEmpty, reason: 'not in Bengali: $latinOnly');
  });

  test('the English table is not Bengali, apart from the language name', () {
    final Iterable<String> bengali = allStrings(
      en,
    ).where((String s) => hasBengali(s) && s != en.bengali);
    expect(bengali, isEmpty);
  });

  test('anything that is not English resolves to Bengali', () {
    expect(CustomerLocalizations.forLocale(const Locale('en')).languageTag,
        'en');
    expect(CustomerLocalizations.forLocale(const Locale('bn')).languageTag,
        'bn');
    expect(CustomerLocalizations.forLocale(const Locale('hi')).languageTag,
        'bn');
  });

  test('the delegate supports the two locales and reloads for neither', () {
    const LocalizationsDelegate<CustomerStrings> delegate =
        CustomerLocalizations.delegate;
    expect(delegate.isSupported(const Locale('bn')), isTrue);
    expect(delegate.isSupported(const Locale('en')), isTrue);
    expect(delegate.isSupported(const Locale('hi')), isFalse);
    expect(delegate.shouldReload(delegate), isFalse);
  });

  testWidgets('outside a MaterialApp the table falls back to Bengali', (
    WidgetTester tester,
  ) async {
    late CustomerStrings seen;
    await tester.pumpWidget(
      Builder(
        builder: (BuildContext context) {
          seen = CustomerLocalizations.of(context);
          return const SizedBox.shrink();
        },
      ),
    );
    expect(seen.languageTag, 'bn');
  });

  testWidgets('inside one it is the app\'s locale', (
    WidgetTester tester,
  ) async {
    late CustomerStrings seen;
    await tester.pumpWidget(
      MaterialApp(
        locale: const Locale('en'),
        localizationsDelegates: const <LocalizationsDelegate<Object>>[
          CustomerLocalizations.delegate,
        ],
        supportedLocales: const <Locale>[Locale('bn'), Locale('en')],
        home: Builder(
          builder: (BuildContext context) {
            seen = CustomerLocalizations.of(context);
            return const SizedBox.shrink();
          },
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(seen.languageTag, 'en');
  });
}
