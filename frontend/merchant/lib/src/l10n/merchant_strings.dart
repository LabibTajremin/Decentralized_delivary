import 'package:flutter/widgets.dart';
import 'package:goklay_core/goklay_core.dart';

/// The words the merchant app says on its own account.
///
/// Short, for the reason `goklay_core`'s table is short: every sentence about
/// an order, a price, a shop's opening state or an approval decision is
/// composed by the server and arrives ready to paint (2.9). `open_status`,
/// `status_label`, `review_note` and the rejection reason a shop types are all
/// examples already in use.
///
/// Nothing here interpolates a number, an amount or a status.
abstract class MerchantStrings {
  /// The language this table is.
  String get languageTag;

  // ---- the shop -----------------------------------------------------------

  /// The heading of the shop screen.
  String get shopTitle;

  /// The heading of the registration form.
  String get registerTitle;

  /// The line under it.
  String get registerSubtitle;

  /// The label on the shop-name field.
  String get shopName;

  /// The heading above the three shop types.
  String get shopType;

  /// A restaurant.
  String get typeRestaurant;

  /// A grocery.
  String get typeGrocery;

  /// A pharmacy.
  String get typePharmacy;

  /// The label on the shop-phone field.
  String get shopPhone;

  /// The label on the shop-email field.
  String get shopEmail;

  /// The label on the first address line.
  String get shopLine1;

  /// The label on the second address line.
  String get shopLine2;

  /// Saves the form.
  String get save;

  /// Registers the shop.
  String get register;

  /// Sends the shop for review.
  String get submitForReview;

  /// The heading above the document list.
  String get documents;

  /// Adds a document.
  String get addDocument;

  /// The label on the document-number field.
  String get documentNumber;

  /// The label on the document-file field.
  String get documentFile;

  /// The heading above what is still wanted.
  String get stillNeeded;

  /// Shown when a shop is waiting on an admin.
  String get awaitingReview;

  // ---- hours and holiday --------------------------------------------------

  /// The heading of the hours screen.
  String get hoursTitle;

  /// Explains the format the windows are typed in.
  String get hoursHelp;

  /// The heading of the holiday screen.
  String get holidayTitle;

  /// The label on the holiday-reason field.
  String get holidayReason;

  /// Closes the shop.
  String get closeShop;

  /// Reopens it.
  String get reopenShop;

  /// Shown when the shop is on holiday.
  String get onHoliday;

  // ---- the catalogue ------------------------------------------------------

  /// The heading of the catalogue screen.
  String get catalogueTitle;

  /// The sections tab.
  String get sections;

  /// The items tab.
  String get items;

  /// The bundles tab.
  String get combos;

  /// Adds a section.
  String get addSection;

  /// Adds an item.
  String get addItem;

  /// The label on the item-name field.
  String get itemName;

  /// The label on the item-description field.
  String get itemDescription;

  /// The label on the price field, which takes minor units.
  String get itemPriceMinor;

  /// The label on the shelf-count field.
  String get stockQuantity;

  /// The label on the unit-of-sale picker.
  String get unitOfSale;

  /// The label on the pack-size field.
  String get packSize;

  /// The label on the brand field.
  String get brand;

  /// The prescription-only switch.
  String get requiresPrescription;

  /// Marks something the shop has switched off.
  String get hidden;

  /// Marks something a customer cannot currently order.
  String get notOrderable;

  /// Turns something on.
  String get show;

  /// Turns something off.
  String get hide;

  /// Shown over an empty catalogue.
  String get catalogueEmpty;

  // ---- the order board ----------------------------------------------------

  /// The heading of the board.
  String get boardTitle;

  /// The tab of orders still in progress.
  String get boardLive;

  /// The tab of finished orders.
  String get boardPast;

  /// Shown over an empty board.
  String get boardEmpty;

  /// Takes an order.
  String get accept;

  /// Refuses one.
  String get reject;

  /// The label on the rejection-reason field.
  String get rejectReason;

  /// Says the kitchen has started.
  String get startPreparing;

  /// Says it is ready for a rider.
  String get markReady;

  /// The heading above an order's lines.
  String get orderLines;

  /// The heading above an order's receipt.
  String get receipt;

  /// The heading above where it goes.
  String get destination;

  /// Marks an order to be paid in cash at the door.
  String get payOnDelivery;

  /// Marks one already paid online.
  String get paidOnline;

  // ---- account ------------------------------------------------------------

  /// The heading of the account screen.
  String get accountTitle;

  /// The shop row.
  String get shopRow;

  /// The hours row.
  String get hoursRow;

  /// The holiday row.
  String get holidayRow;

  /// Signs out.
  String get signOut;

  /// Shown when the signed-in account is not a merchant.
  String get wrongRole;
}

/// Bengali. The default (1.4).
class MerchantStringsBn implements MerchantStrings {
  /// Creates the Bengali table.
  const MerchantStringsBn();

  @override
  String get languageTag => 'bn';
  @override
  String get shopTitle => 'দোকান';
  @override
  String get registerTitle => 'দোকান নিবন্ধন';
  @override
  String get registerSubtitle =>
      'দোকানের তথ্য দিন, কাগজপত্র যোগ করুন, তারপর অনুমোদনের জন্য পাঠান।';
  @override
  String get shopName => 'দোকানের নাম';
  @override
  String get shopType => 'দোকানের ধরন';
  @override
  String get typeRestaurant => 'রেস্তোরাঁ';
  @override
  String get typeGrocery => 'মুদি দোকান';
  @override
  String get typePharmacy => 'ফার্মেসি';
  @override
  String get shopPhone => 'দোকানের মোবাইল নম্বর';
  @override
  String get shopEmail => 'ইমেইল';
  @override
  String get shopLine1 => 'ঠিকানা';
  @override
  String get shopLine2 => 'ঠিকানা (দ্বিতীয় লাইন)';
  @override
  String get save => 'সংরক্ষণ করুন';
  @override
  String get register => 'নিবন্ধন করুন';
  @override
  String get submitForReview => 'অনুমোদনের জন্য পাঠান';
  @override
  String get documents => 'কাগজপত্র';
  @override
  String get addDocument => 'কাগজ যোগ করুন';
  @override
  String get documentNumber => 'নম্বর';
  @override
  String get documentFile => 'ফাইলের ঠিকানা';
  @override
  String get stillNeeded => 'এখনো যা প্রয়োজন';
  @override
  String get awaitingReview => 'অনুমোদনের অপেক্ষায় আছে।';
  @override
  String get hoursTitle => 'খোলার সময়';
  @override
  String get hoursHelp => 'প্রতিটি সময় এভাবে লিখুন: 09:00-22:00। খালি রাখলে সেদিন বন্ধ।';
  @override
  String get holidayTitle => 'ছুটি';
  @override
  String get holidayReason => 'কারণ';
  @override
  String get closeShop => 'দোকান বন্ধ রাখুন';
  @override
  String get reopenShop => 'আবার খুলুন';
  @override
  String get onHoliday => 'দোকান এখন ছুটিতে।';
  @override
  String get catalogueTitle => 'মেনু';
  @override
  String get sections => 'বিভাগ';
  @override
  String get items => 'পণ্য';
  @override
  String get combos => 'প্যাকেজ';
  @override
  String get addSection => 'বিভাগ যোগ করুন';
  @override
  String get addItem => 'পণ্য যোগ করুন';
  @override
  String get itemName => 'পণ্যের নাম';
  @override
  String get itemDescription => 'বিবরণ';
  @override
  String get itemPriceMinor => 'দাম (পয়সায়)';
  @override
  String get stockQuantity => 'স্টক';
  @override
  String get unitOfSale => 'একক';
  @override
  String get packSize => 'প্যাকের পরিমাণ';
  @override
  String get brand => 'ব্র্যান্ড';
  @override
  String get requiresPrescription => 'প্রেসক্রিপশন লাগবে';
  @override
  String get hidden => 'লুকানো';
  @override
  String get notOrderable => 'অর্ডার নেওয়া যাচ্ছে না';
  @override
  String get show => 'দেখান';
  @override
  String get hide => 'লুকান';
  @override
  String get catalogueEmpty => 'মেনুতে এখনো কিছু নেই।';
  @override
  String get boardTitle => 'অর্ডার বোর্ড';
  @override
  String get boardLive => 'চলমান';
  @override
  String get boardPast => 'আগের';
  @override
  String get boardEmpty => 'এখন কোনো অর্ডার নেই।';
  @override
  String get accept => 'গ্রহণ করুন';
  @override
  String get reject => 'ফিরিয়ে দিন';
  @override
  String get rejectReason => 'কেন ফেরত দিচ্ছেন?';
  @override
  String get startPreparing => 'রান্না শুরু';
  @override
  String get markReady => 'প্রস্তুত';
  @override
  String get orderLines => 'যা বানাতে হবে';
  @override
  String get receipt => 'হিসাব';
  @override
  String get destination => 'যেখানে যাবে';
  @override
  String get payOnDelivery => 'ডেলিভারিতে নগদ';
  @override
  String get paidOnline => 'অনলাইনে পরিশোধিত';
  @override
  String get accountTitle => 'অ্যাকাউন্ট';
  @override
  String get shopRow => 'দোকানের তথ্য';
  @override
  String get hoursRow => 'খোলার সময়';
  @override
  String get holidayRow => 'ছুটি';
  @override
  String get signOut => 'সাইন আউট';
  @override
  String get wrongRole => 'এই অ্যাকাউন্টটি দোকানের নয়।';
}

/// English, for `?lang=en`.
class MerchantStringsEn implements MerchantStrings {
  /// Creates the English table.
  const MerchantStringsEn();

  @override
  String get languageTag => 'en';
  @override
  String get shopTitle => 'Shop';
  @override
  String get registerTitle => 'Register your shop';
  @override
  String get registerSubtitle =>
      'Fill in the details, add your documents, then send it for approval.';
  @override
  String get shopName => 'Shop name';
  @override
  String get shopType => 'Type of shop';
  @override
  String get typeRestaurant => 'Restaurant';
  @override
  String get typeGrocery => 'Grocery';
  @override
  String get typePharmacy => 'Pharmacy';
  @override
  String get shopPhone => 'Shop phone number';
  @override
  String get shopEmail => 'Email';
  @override
  String get shopLine1 => 'Address';
  @override
  String get shopLine2 => 'Address line 2';
  @override
  String get save => 'Save';
  @override
  String get register => 'Register';
  @override
  String get submitForReview => 'Send for approval';
  @override
  String get documents => 'Documents';
  @override
  String get addDocument => 'Add a document';
  @override
  String get documentNumber => 'Number';
  @override
  String get documentFile => 'File location';
  @override
  String get stillNeeded => 'Still needed';
  @override
  String get awaitingReview => 'Waiting for approval.';
  @override
  String get hoursTitle => 'Opening hours';
  @override
  String get hoursHelp =>
      'Write each window like this: 09:00-22:00. Leave a day empty to close it.';
  @override
  String get holidayTitle => 'Holiday';
  @override
  String get holidayReason => 'Reason';
  @override
  String get closeShop => 'Close the shop';
  @override
  String get reopenShop => 'Reopen';
  @override
  String get onHoliday => 'The shop is on holiday.';
  @override
  String get catalogueTitle => 'Menu';
  @override
  String get sections => 'Sections';
  @override
  String get items => 'Items';
  @override
  String get combos => 'Bundles';
  @override
  String get addSection => 'Add a section';
  @override
  String get addItem => 'Add an item';
  @override
  String get itemName => 'Item name';
  @override
  String get itemDescription => 'Description';
  @override
  String get itemPriceMinor => 'Price (in poisha)';
  @override
  String get stockQuantity => 'Stock';
  @override
  String get unitOfSale => 'Unit';
  @override
  String get packSize => 'Pack size';
  @override
  String get brand => 'Brand';
  @override
  String get requiresPrescription => 'Prescription required';
  @override
  String get hidden => 'Hidden';
  @override
  String get notOrderable => 'Cannot be ordered';
  @override
  String get show => 'Show';
  @override
  String get hide => 'Hide';
  @override
  String get catalogueEmpty => 'Nothing on the menu yet.';
  @override
  String get boardTitle => 'Order board';
  @override
  String get boardLive => 'Current';
  @override
  String get boardPast => 'Past';
  @override
  String get boardEmpty => 'No orders right now.';
  @override
  String get accept => 'Accept';
  @override
  String get reject => 'Reject';
  @override
  String get rejectReason => 'Why are you rejecting it?';
  @override
  String get startPreparing => 'Start preparing';
  @override
  String get markReady => 'Ready';
  @override
  String get orderLines => 'To make';
  @override
  String get receipt => 'Receipt';
  @override
  String get destination => 'Going to';
  @override
  String get payOnDelivery => 'Cash on delivery';
  @override
  String get paidOnline => 'Paid online';
  @override
  String get accountTitle => 'Account';
  @override
  String get shopRow => 'Shop details';
  @override
  String get hoursRow => 'Opening hours';
  @override
  String get holidayRow => 'Holiday';
  @override
  String get signOut => 'Sign out';
  @override
  String get wrongRole => 'This account is not a shop.';
}

/// Looks the table up, Bengali-first.
abstract final class MerchantLocalizations {
  /// The delegate the app installs alongside `goklay_core`'s.
  static const LocalizationsDelegate<MerchantStrings> delegate =
      _MerchantStringsDelegate();

  /// The table for [locale]. Anything that is not English is Bengali.
  static MerchantStrings forLocale(Locale locale) =>
      locale.languageCode == 'en'
      ? const MerchantStringsEn()
      : const MerchantStringsBn();

  /// The table in scope, falling back to Bengali outside a [MaterialApp].
  static MerchantStrings of(BuildContext context) =>
      Localizations.of<MerchantStrings>(context, MerchantStrings) ??
      const MerchantStringsBn();
}

class _MerchantStringsDelegate extends LocalizationsDelegate<MerchantStrings> {
  const _MerchantStringsDelegate();

  @override
  bool isSupported(Locale locale) => GoklayLocalizations.supportedLocales.any(
    (Locale supported) => supported.languageCode == locale.languageCode,
  );

  @override
  Future<MerchantStrings> load(Locale locale) async =>
      MerchantLocalizations.forLocale(locale);

  @override
  bool shouldReload(_MerchantStringsDelegate old) => false;
}
