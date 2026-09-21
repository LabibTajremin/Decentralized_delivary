import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_merchant/src/l10n/merchant_strings.dart';

/// Every getter on the table, so a string added to one language and forgotten
/// in the other cannot pass.
List<String> allStrings(MerchantStrings s) => <String>[
  s.languageTag,
  s.shopTitle, s.registerTitle, s.registerSubtitle,
  s.shopName, s.shopType,
  s.typeRestaurant, s.typeGrocery, s.typePharmacy,
  s.shopPhone, s.shopEmail, s.shopLine1, s.shopLine2,
  s.save, s.register, s.submitForReview,
  s.documents, s.addDocument, s.documentNumber, s.documentFile,
  s.stillNeeded, s.awaitingReview,
  s.hoursTitle, s.hoursHelp,
  s.holidayTitle, s.holidayReason, s.closeShop, s.reopenShop, s.onHoliday,
  s.catalogueTitle, s.sections, s.items, s.combos,
  s.addSection, s.addItem,
  s.itemName, s.itemDescription, s.itemPriceMinor,
  s.stockQuantity, s.unitOfSale, s.packSize, s.brand,
  s.requiresPrescription,
  s.hidden, s.notOrderable, s.show, s.hide, s.catalogueEmpty,
  s.boardTitle, s.boardLive, s.boardPast, s.boardEmpty,
  s.accept, s.reject, s.rejectReason, s.startPreparing, s.markReady,
  s.orderLines, s.receipt, s.destination,
  s.payOnDelivery, s.paidOnline,
  s.accountTitle, s.shopRow, s.hoursRow, s.holidayRow, s.signOut, s.wrongRole,
];

/// The Bengali block, `U+0980..U+09FF`.
bool hasBengali(String value) =>
    value.runes.any((int rune) => rune >= 0x0980 && rune <= 0x09FF);

void main() {
  const MerchantStringsBn bn = MerchantStringsBn();
  const MerchantStringsEn en = MerchantStringsEn();

  test('both tables answer every string', () {
    expect(allStrings(bn), hasLength(allStrings(en).length));
    expect(allStrings(bn).where((String s) => s.isEmpty), isEmpty);
    expect(allStrings(en).where((String s) => s.isEmpty), isEmpty);
  });

  test('the Bengali table is actually in Bengali script', () {
    // The language tag is a code, and the hours help names a 24-hour clock
    // format that is the same in both languages — but it also has Bengali
    // around it, so only the tag is exempt.
    final Iterable<String> latinOnly = allStrings(
      bn,
    ).where((String s) => s != bn.languageTag && !hasBengali(s));
    expect(latinOnly, isEmpty, reason: 'not in Bengali: $latinOnly');
  });

  test('the English table is not Bengali', () {
    expect(allStrings(en).where(hasBengali), isEmpty);
  });

  test('anything that is not English resolves to Bengali', () {
    expect(
      MerchantLocalizations.forLocale(const Locale('en')).languageTag,
      'en',
    );
    expect(
      MerchantLocalizations.forLocale(const Locale('bn')).languageTag,
      'bn',
    );
    expect(
      MerchantLocalizations.forLocale(const Locale('ar')).languageTag,
      'bn',
    );
  });

  test('the delegate supports the two locales and reloads for neither', () {
    const LocalizationsDelegate<MerchantStrings> delegate =
        MerchantLocalizations.delegate;
    expect(delegate.isSupported(const Locale('bn')), isTrue);
    expect(delegate.isSupported(const Locale('en')), isTrue);
    expect(delegate.isSupported(const Locale('ar')), isFalse);
    expect(delegate.shouldReload(delegate), isFalse);
  });

  testWidgets('outside a MaterialApp the table falls back to Bengali', (
    WidgetTester tester,
  ) async {
    late MerchantStrings seen;
    await tester.pumpWidget(
      Builder(
        builder: (BuildContext context) {
          seen = MerchantLocalizations.of(context);
          return const SizedBox.shrink();
        },
      ),
    );
    expect(seen.languageTag, 'bn');
  });

  testWidgets('inside one it is the app\'s locale', (
    WidgetTester tester,
  ) async {
    late MerchantStrings seen;
    await tester.pumpWidget(
      MaterialApp(
        locale: const Locale('en'),
        localizationsDelegates: const <LocalizationsDelegate<Object>>[
          MerchantLocalizations.delegate,
        ],
        supportedLocales: const <Locale>[Locale('bn'), Locale('en')],
        home: Builder(
          builder: (BuildContext context) {
            seen = MerchantLocalizations.of(context);
            return const SizedBox.shrink();
          },
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(seen.languageTag, 'en');
  });
}
