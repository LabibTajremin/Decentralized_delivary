import 'package:flutter/widgets.dart';
import 'package:goklay_core/goklay_core.dart';

/// The words the customer app says on its own account.
///
/// `goklay_core` has seven strings, and the reason is stated there: every
/// sentence about an order, a price, a fee or a rule is composed by the server
/// and arrives ready to paint (2.9). What is left is chrome — the heading over
/// a list, the label on a field, the name of a tab — and the handful of
/// sentences that exist *before* a server has answered.
///
/// Nothing here interpolates a number, an amount or a status. If a string
/// would need one, it is the wrong string: the server sends the whole
/// sentence. `notice`, `blocker_text`, `issue_text`, `status_label`,
/// `open_status` and `cancel.text` are all examples of that already in use.
///
/// Hand-written rather than generated for the same reason `goklay_core`'s
/// table is: a generated file would have to be carved out of the 100% coverage
/// gate, and `frontend/coverage-exclusions.txt` is empty on purpose.
abstract class CustomerStrings {
  /// The language this table is, for tests and for the profile call.
  String get languageTag;

  // ---- onboarding and sign-in ------------------------------------------

  /// Skips the onboarding pages.
  String get skip;

  /// Moves to the next onboarding page.
  String get next;


  /// Heading of the first onboarding page.
  String get onboardingTitle1;

  /// Body of the first onboarding page.
  String get onboardingBody1;

  /// Heading of the second onboarding page.
  String get onboardingTitle2;

  /// Body of the second onboarding page.
  String get onboardingBody2;

  /// Heading of the third onboarding page.
  String get onboardingTitle3;

  /// Body of the third onboarding page.
  String get onboardingBody3;


  /// Sub-heading of the sign-in options screen.
  String get signInSubtitle;

  /// The one sign-in method this product has.
  String get continueWithPhone;














  // ---- home and discovery ----------------------------------------------

  /// Label above the delivery address on the home screen.
  String get deliverTo;

  /// Placeholder in the shop search field.
  String get searchShops;

  /// The chip that clears the type filter.
  String get typeAll;

  /// Restaurants.
  String get typeRestaurant;

  /// Grocery shops.
  String get typeGrocery;

  /// Pharmacies.
  String get typePharmacy;

  /// Widens the search to the next rung of the radius ladder.
  String get searchWider;

  /// Shown when a search found nothing at all.
  String get noShops;

  // ---- shop, item and cart ---------------------------------------------

  /// Tab over a shop's items.
  String get menu;

  /// Tab over a shop's bundles.
  String get combos;

  /// Tab over a shop's reviews.
  String get reviews;

  /// Adds the configured item to the cart.
  String get addToCart;

  /// Label above the note field on an item.
  String get noteToShop;

  /// Placeholder in the note field.
  String get noteHint;

  /// Label above the quantity stepper.
  String get quantity;



  /// Removes a line from the cart.
  String get removeLine;

  /// Heading of the cart.
  String get cartTitle;

  /// Shown over an empty cart.
  String get cartEmpty;

  /// Goes from the cart to the review screen.
  String get reviewCart;

  /// Heading of the review screen.
  String get reviewCartTitle;

  /// Heading of the place-order screen.
  String get placeOrderTitle;

  /// Places the order.
  String get placeOrder;

  /// Heading above the two payment methods.
  String get paymentMethod;

  /// Cash on delivery.
  String get payCash;

  /// Pay online.
  String get payOnline;

  /// Heading above the chosen address.
  String get deliveryAddress;

  /// Opens the address book from checkout.
  String get chooseAddress;

  /// Shown at checkout when no address is chosen yet.
  String get noAddressChosen;

  // ---- orders, tracking and payment ------------------------------------

  /// Heading of the orders screen.
  String get activityTitle;

  /// The tab of orders still in progress.
  String get ordersLive;

  /// The tab of finished orders.
  String get ordersPast;

  /// Shown over an empty order list.
  String get noOrders;

  /// Heading of one order.
  String get orderTitle;

  /// Heading above an order's timeline.
  String get orderTimeline;

  /// Heading above an order's receipt.
  String get receipt;

  /// Opens live tracking.
  String get trackOrder;

  /// Cancels an order.
  String get cancelOrder;

  /// Asks the customer to confirm a cancellation.
  String get cancelOrderConfirm;

  /// Heading of the tracking screen.
  String get trackingTitle;

  /// Calls the rider.
  String get callRider;

  /// Calls the shop.
  String get callShop;

  /// Shown while no rider has picked the order up yet.
  String get awaitingRider;

  /// Shown once the tracking stream has sent its last frame.
  String get trackingEnded;

  /// Heading of the payment screen.
  String get paymentTitle;

  /// Opens the payment page.
  String get payNow;

  /// Re-reads the payment's state.
  String get checkPayment;

  /// Shown when a checkout came back without anywhere to send the customer.
  String get noPaymentPage;

  /// The button that completes a payment on a demo deployment, where there is
  /// no bank to confirm one. Shown only when the server says so.
  String get completeDemoPayment;

  // ---- account ----------------------------------------------------------

  /// Heading of the account screen.
  String get accountTitle;

  /// The profile row.
  String get personalInfo;

  /// The address-book row.
  String get savedAddresses;

  /// The sessions row.
  String get security;

  /// The language row.
  String get language;

  /// The notifications row.
  String get notifications;

  /// The support row.
  String get support;

  /// Edits the profile.
  String get editProfile;

  /// Saves an edited form.
  String get save;

  /// Label on the name field.
  String get nameLabel;

  /// Label on the email field.
  String get emailLabel;

  /// Signs out on this device.
  String get signOut;

  /// Signs out everywhere.
  String get signOutEverywhere;

  /// Adds an address.
  String get addAddress;

  /// Label on the address nickname field.
  String get addressLabelField;

  /// Label on the recipient-name field.
  String get recipientName;

  /// Label on the recipient-phone field.
  String get recipientPhone;

  /// Label on the first address line.
  String get addressLine1;

  /// Label on the second address line.
  String get addressLine2;

  /// Label on the rider-instructions field.
  String get addressInstructions;

  /// Makes an address the default.
  String get makeDefault;

  /// Marks the current default.
  String get defaultAddress;

  /// Deletes an address.
  String get deleteAddress;

  /// Shown over an empty address book.
  String get noAddresses;

  /// Shown over an empty notification list.
  String get noNotifications;

  /// Marks a notification the server could not deliver.
  String get notificationFailed;

  /// Bengali, named in Bengali.
  String get bengali;

  /// English, named in English.
  String get english;

  // ---- reviews and support ----------------------------------------------

  /// Leaves a review on a delivered order.
  String get leaveReview;

  /// Heading of the review form.
  String get reviewTitle;

  /// Heading above the shop's stars.
  String get rateShop;

  /// Heading above the rider's stars.
  String get rateRider;

  /// Label on the comment field.
  String get reviewComment;

  /// Submits a review.
  String get submitReview;

  /// Shown after a review is accepted.
  String get reviewThanks;

  /// Shown over a subject with no reviews.
  String get noReviews;

  /// Shown over a subject with no rating yet.
  String get noRatingYet;

  /// Opens the support form from an order.
  String get getHelp;

  /// Heading of the support screen.
  String get supportTitle;

  /// Label on the ticket-subject field.
  String get ticketSubject;

  /// Raises a ticket.
  String get raiseTicket;

  /// Marks an open ticket.
  String get ticketOpen;

  /// Marks a resolved ticket.
  String get ticketResolved;

  /// Shown over an empty ticket list.
  String get noTickets;

  // ---- screens the backend does not have --------------------------------


  /// Heading of the offers placeholder.
  String get offersTitle;

  /// Heading of the promos placeholder.
  String get promosTitle;

  /// Heading of the referral placeholder.
  String get referralTitle;

  /// Heading of the safety placeholder.
  String get safetyTitle;

  /// Heading of the permissions screen.
  String get permissionsTitle;

  /// Heading of the payment-methods screen.
  String get paymentMethodsTitle;

  /// Explains that the product has exactly two payment methods.
  String get paymentMethodsBody;

  /// Explains why the app asks for location.
  String get permissionLocation;

  /// Explains why the app asks to send notifications.
  String get permissionNotifications;

  // ---- common -----------------------------------------------------------

  /// Goes back.
  String get back;

  /// Confirms.
  String get confirm;


}

/// Bengali. The default, because the product is (1.4).
class CustomerStringsBn implements CustomerStrings {
  /// Creates the Bengali table.
  const CustomerStringsBn();

  @override
  String get languageTag => 'bn';

  @override
  String get skip => 'এড়িয়ে যান';
  @override
  String get next => 'পরবর্তী';
  @override
  String get onboardingTitle1 => 'কাছের দোকান থেকে';
  @override
  String get onboardingBody1 =>
      'আপনার এলাকার রেস্তোরাঁ, মুদি দোকান আর ফার্মেসি এক জায়গায়।';
  @override
  String get onboardingTitle2 => 'দাম আগেই জানুন';
  @override
  String get onboardingBody2 =>
      'ডেলিভারি খরচসহ পুরো হিসাব অর্ডার করার আগেই দেখে নিন।';
  @override
  String get onboardingTitle3 => 'পথেই দেখুন';
  @override
  String get onboardingBody3 =>
      'রাইডার কোথায় আছেন, অর্ডার দেওয়ার পর থেকেই দেখতে পাবেন।';
  @override
  String get signInSubtitle => 'চালিয়ে যেতে মোবাইল নম্বর দিন।';
  @override
  String get continueWithPhone => 'মোবাইল নম্বর দিয়ে চালিয়ে যান';

  @override
  String get deliverTo => 'ডেলিভারি হবে';
  @override
  String get searchShops => 'দোকান খুঁজুন';
  @override
  String get typeAll => 'সব';
  @override
  String get typeRestaurant => 'খাবার';
  @override
  String get typeGrocery => 'মুদি';
  @override
  String get typePharmacy => 'ওষুধ';
  @override
  String get searchWider => 'আরও দূর পর্যন্ত খুঁজুন';
  @override
  String get noShops => 'এখানে এখনো কোনো দোকান নেই।';

  @override
  String get menu => 'মেনু';
  @override
  String get combos => 'প্যাকেজ';
  @override
  String get reviews => 'রিভিউ';
  @override
  String get addToCart => 'কার্টে যোগ করুন';
  @override
  String get noteToShop => 'দোকানকে নোট';
  @override
  String get noteHint => 'যেমন: ঝাল কম দেবেন';
  @override
  String get quantity => 'পরিমাণ';
  @override
  String get removeLine => 'সরান';
  @override
  String get cartTitle => 'কার্ট';
  @override
  String get cartEmpty => 'আপনার কার্ট খালি।';
  @override
  String get reviewCart => 'কার্ট দেখে নিন';
  @override
  String get reviewCartTitle => 'কার্ট যাচাই';
  @override
  String get placeOrderTitle => 'অর্ডার নিশ্চিত করুন';
  @override
  String get placeOrder => 'অর্ডার করুন';
  @override
  String get paymentMethod => 'পেমেন্ট পদ্ধতি';
  @override
  String get payCash => 'ডেলিভারিতে নগদ';
  @override
  String get payOnline => 'অনলাইনে পেমেন্ট';
  @override
  String get deliveryAddress => 'ডেলিভারি ঠিকানা';
  @override
  String get chooseAddress => 'ঠিকানা বাছুন';
  @override
  String get noAddressChosen => 'কোনো ঠিকানা বাছা হয়নি।';

  @override
  String get activityTitle => 'অর্ডারসমূহ';
  @override
  String get ordersLive => 'চলমান';
  @override
  String get ordersPast => 'আগের';
  @override
  String get noOrders => 'আপনি এখনো কোনো অর্ডার করেননি।';
  @override
  String get orderTitle => 'অর্ডার';
  @override
  String get orderTimeline => 'অর্ডারের ধাপ';
  @override
  String get receipt => 'হিসাব';
  @override
  String get trackOrder => 'অর্ডার ট্র্যাক করুন';
  @override
  String get cancelOrder => 'অর্ডার বাতিল করুন';
  @override
  String get cancelOrderConfirm => 'অর্ডারটি বাতিল করবেন?';
  @override
  String get trackingTitle => 'ট্র্যাকিং';
  @override
  String get callRider => 'রাইডারকে কল করুন';
  @override
  String get callShop => 'দোকানে কল করুন';
  @override
  String get awaitingRider => 'রাইডার এখনো নেননি।';
  @override
  String get trackingEnded => 'ট্র্যাকিং শেষ।';
  @override
  String get paymentTitle => 'পেমেন্ট';
  @override
  String get payNow => 'এখন পেমেন্ট করুন';
  @override
  String get checkPayment => 'পেমেন্টের অবস্থা দেখুন';
  @override
  String get noPaymentPage => 'পেমেন্ট পেজ পাওয়া যায়নি।';

  @override
  String get completeDemoPayment => 'ডেমো পেমেন্ট সম্পন্ন করুন';

  @override
  String get accountTitle => 'অ্যাকাউন্ট';
  @override
  String get personalInfo => 'ব্যক্তিগত তথ্য';
  @override
  String get savedAddresses => 'সংরক্ষিত ঠিকানা';
  @override
  String get security => 'নিরাপত্তা';
  @override
  String get language => 'ভাষা';
  @override
  String get notifications => 'নোটিফিকেশন';
  @override
  String get support => 'সহায়তা';
  @override
  String get editProfile => 'তথ্য সম্পাদনা';
  @override
  String get save => 'সংরক্ষণ করুন';
  @override
  String get nameLabel => 'নাম';
  @override
  String get emailLabel => 'ইমেইল';
  @override
  String get signOut => 'সাইন আউট';
  @override
  String get signOutEverywhere => 'সব ডিভাইস থেকে সাইন আউট';
  @override
  String get addAddress => 'ঠিকানা যোগ করুন';
  @override
  String get addressLabelField => 'ঠিকানার নাম';
  @override
  String get recipientName => 'যিনি নেবেন';
  @override
  String get recipientPhone => 'তাঁর মোবাইল নম্বর';
  @override
  String get addressLine1 => 'ঠিকানা';
  @override
  String get addressLine2 => 'ঠিকানা (দ্বিতীয় লাইন)';
  @override
  String get addressInstructions => 'রাইডারের জন্য নির্দেশনা';
  @override
  String get makeDefault => 'ডিফল্ট করুন';
  @override
  String get defaultAddress => 'ডিফল্ট';
  @override
  String get deleteAddress => 'ঠিকানা মুছুন';
  @override
  String get noAddresses => 'কোনো ঠিকানা সংরক্ষিত নেই।';
  @override
  String get noNotifications => 'কোনো নোটিফিকেশন নেই।';
  @override
  String get notificationFailed => 'পৌঁছায়নি';
  @override
  String get bengali => 'বাংলা';
  @override
  String get english => 'English';

  @override
  String get leaveReview => 'রিভিউ দিন';
  @override
  String get reviewTitle => 'রিভিউ';
  @override
  String get rateShop => 'দোকানকে রেটিং দিন';
  @override
  String get rateRider => 'রাইডারকে রেটিং দিন';
  @override
  String get reviewComment => 'আপনার মন্তব্য';
  @override
  String get submitReview => 'রিভিউ পাঠান';
  @override
  String get reviewThanks => 'ধন্যবাদ, আপনার রিভিউ পাঠানো হয়েছে।';
  @override
  String get noReviews => 'এখনো কোনো রিভিউ নেই।';
  @override
  String get noRatingYet => 'এখনো রেটিং নেই';
  @override
  String get getHelp => 'সহায়তা নিন';
  @override
  String get supportTitle => 'সহায়তা';
  @override
  String get ticketSubject => 'সমস্যাটি কী?';
  @override
  String get raiseTicket => 'পাঠান';
  @override
  String get ticketOpen => 'চলমান';
  @override
  String get ticketResolved => 'সমাধান হয়েছে';
  @override
  String get noTickets => 'কোনো অনুরোধ নেই।';

  @override
  String get offersTitle => 'অফার';
  @override
  String get promosTitle => 'প্রোমো কোড';
  @override
  String get referralTitle => 'বন্ধুকে আনুন';
  @override
  String get safetyTitle => 'সুরক্ষা';
  @override
  String get permissionsTitle => 'অনুমতি';
  @override
  String get paymentMethodsTitle => 'পেমেন্ট পদ্ধতি';
  @override
  String get paymentMethodsBody =>
      'অর্ডার করার সময় নগদ অথবা অনলাইন বেছে নিতে পারবেন। কার্ড বা ওয়ালেট সংরক্ষণ করা হয় না।';
  @override
  String get permissionLocation =>
      'কাছের দোকান দেখাতে ও ডেলিভারি খরচ হিসাব করতে আপনার অবস্থান লাগে।';
  @override
  String get permissionNotifications =>
      'অর্ডারের অবস্থা জানাতে নোটিফিকেশন পাঠানোর অনুমতি লাগে।';

  @override
  String get back => 'ফিরে যান';
  @override
  String get confirm => 'নিশ্চিত করুন';
}

/// English, for `?lang=en`.
class CustomerStringsEn implements CustomerStrings {
  /// Creates the English table.
  const CustomerStringsEn();

  @override
  String get languageTag => 'en';

  @override
  String get skip => 'Skip';
  @override
  String get next => 'Next';
  @override
  String get onboardingTitle1 => 'From shops near you';
  @override
  String get onboardingBody1 =>
      'Restaurants, grocers and pharmacies in your area, in one place.';
  @override
  String get onboardingTitle2 => 'Know the price first';
  @override
  String get onboardingBody2 =>
      'See the whole bill, delivery included, before you order.';
  @override
  String get onboardingTitle3 => 'Watch it come';
  @override
  String get onboardingBody3 =>
      'Follow your rider from the moment the order is placed.';
  @override
  String get signInSubtitle => 'Enter your mobile number to continue.';
  @override
  String get continueWithPhone => 'Continue with mobile number';

  @override
  String get deliverTo => 'Deliver to';
  @override
  String get searchShops => 'Search shops';
  @override
  String get typeAll => 'All';
  @override
  String get typeRestaurant => 'Food';
  @override
  String get typeGrocery => 'Grocery';
  @override
  String get typePharmacy => 'Pharmacy';
  @override
  String get searchWider => 'Search further out';
  @override
  String get noShops => 'No shops here yet.';

  @override
  String get menu => 'Menu';
  @override
  String get combos => 'Bundles';
  @override
  String get reviews => 'Reviews';
  @override
  String get addToCart => 'Add to cart';
  @override
  String get noteToShop => 'Note to the shop';
  @override
  String get noteHint => 'For example: not too spicy';
  @override
  String get quantity => 'Quantity';
  @override
  String get removeLine => 'Remove';
  @override
  String get cartTitle => 'Cart';
  @override
  String get cartEmpty => 'Your cart is empty.';
  @override
  String get reviewCart => 'Review cart';
  @override
  String get reviewCartTitle => 'Review cart';
  @override
  String get placeOrderTitle => 'Confirm order';
  @override
  String get placeOrder => 'Place order';
  @override
  String get paymentMethod => 'Payment method';
  @override
  String get payCash => 'Cash on delivery';
  @override
  String get payOnline => 'Pay online';
  @override
  String get deliveryAddress => 'Delivery address';
  @override
  String get chooseAddress => 'Choose an address';
  @override
  String get noAddressChosen => 'No address chosen yet.';

  @override
  String get activityTitle => 'Orders';
  @override
  String get ordersLive => 'Current';
  @override
  String get ordersPast => 'Past';
  @override
  String get noOrders => 'You have not ordered yet.';
  @override
  String get orderTitle => 'Order';
  @override
  String get orderTimeline => 'Progress';
  @override
  String get receipt => 'Receipt';
  @override
  String get trackOrder => 'Track order';
  @override
  String get cancelOrder => 'Cancel order';
  @override
  String get cancelOrderConfirm => 'Cancel this order?';
  @override
  String get trackingTitle => 'Tracking';
  @override
  String get callRider => 'Call the rider';
  @override
  String get callShop => 'Call the shop';
  @override
  String get awaitingRider => 'No rider has collected it yet.';
  @override
  String get trackingEnded => 'Tracking finished.';
  @override
  String get paymentTitle => 'Payment';
  @override
  String get payNow => 'Pay now';
  @override
  String get checkPayment => 'Check payment status';
  @override
  String get noPaymentPage => 'No payment page was returned.';

  @override
  String get completeDemoPayment => 'Complete the demo payment';

  @override
  String get accountTitle => 'Account';
  @override
  String get personalInfo => 'Personal info';
  @override
  String get savedAddresses => 'Saved addresses';
  @override
  String get security => 'Security';
  @override
  String get language => 'Language';
  @override
  String get notifications => 'Notifications';
  @override
  String get support => 'Support';
  @override
  String get editProfile => 'Edit profile';
  @override
  String get save => 'Save';
  @override
  String get nameLabel => 'Name';
  @override
  String get emailLabel => 'Email';
  @override
  String get signOut => 'Sign out';
  @override
  String get signOutEverywhere => 'Sign out everywhere';
  @override
  String get addAddress => 'Add an address';
  @override
  String get addressLabelField => 'Name for this address';
  @override
  String get recipientName => 'Who will receive it';
  @override
  String get recipientPhone => 'Their mobile number';
  @override
  String get addressLine1 => 'Address';
  @override
  String get addressLine2 => 'Address line 2';
  @override
  String get addressInstructions => 'Instructions for the rider';
  @override
  String get makeDefault => 'Make default';
  @override
  String get defaultAddress => 'Default';
  @override
  String get deleteAddress => 'Delete address';
  @override
  String get noAddresses => 'No addresses saved.';
  @override
  String get noNotifications => 'No notifications.';
  @override
  String get notificationFailed => 'Not delivered';
  @override
  String get bengali => 'বাংলা';
  @override
  String get english => 'English';

  @override
  String get leaveReview => 'Leave a review';
  @override
  String get reviewTitle => 'Review';
  @override
  String get rateShop => 'Rate the shop';
  @override
  String get rateRider => 'Rate the rider';
  @override
  String get reviewComment => 'Your comment';
  @override
  String get submitReview => 'Send review';
  @override
  String get reviewThanks => 'Thank you — your review was sent.';
  @override
  String get noReviews => 'No reviews yet.';
  @override
  String get noRatingYet => 'No rating yet';
  @override
  String get getHelp => 'Get help';
  @override
  String get supportTitle => 'Support';
  @override
  String get ticketSubject => 'What went wrong?';
  @override
  String get raiseTicket => 'Send';
  @override
  String get ticketOpen => 'Open';
  @override
  String get ticketResolved => 'Resolved';
  @override
  String get noTickets => 'No requests.';

  @override
  String get offersTitle => 'Offers';
  @override
  String get promosTitle => 'Promo codes';
  @override
  String get referralTitle => 'Refer a friend';
  @override
  String get safetyTitle => 'Safety';
  @override
  String get permissionsTitle => 'Permissions';
  @override
  String get paymentMethodsTitle => 'Payment methods';
  @override
  String get paymentMethodsBody =>
      'You choose cash or online when you place an order. No card or wallet is stored.';
  @override
  String get permissionLocation =>
      'Your location is needed to show nearby shops and to work out delivery.';
  @override
  String get permissionNotifications =>
      'Notification permission is needed to tell you how your order is going.';

  @override
  String get back => 'Back';
  @override
  String get confirm => 'Confirm';
}

/// Looks the table up, Bengali-first.
abstract final class CustomerLocalizations {
  /// The delegate the app installs alongside `goklay_core`'s.
  static const LocalizationsDelegate<CustomerStrings> delegate =
      _CustomerStringsDelegate();

  /// The table for [locale]. Anything that is not English is Bengali, which
  /// is the same rule `goklay_core` applies and the same one the backend does.
  static CustomerStrings forLocale(Locale locale) =>
      locale.languageCode == 'en'
      ? const CustomerStringsEn()
      : const CustomerStringsBn();

  /// The table in scope, falling back to Bengali outside a [MaterialApp] —
  /// which is what a widget test that pumps a bare screen gets.
  static CustomerStrings of(BuildContext context) =>
      Localizations.of<CustomerStrings>(context, CustomerStrings) ??
      const CustomerStringsBn();
}

class _CustomerStringsDelegate extends LocalizationsDelegate<CustomerStrings> {
  const _CustomerStringsDelegate();

  @override
  bool isSupported(Locale locale) => GoklayLocalizations.supportedLocales.any(
    (Locale supported) => supported.languageCode == locale.languageCode,
  );

  @override
  Future<CustomerStrings> load(Locale locale) async =>
      CustomerLocalizations.forLocale(locale);

  @override
  bool shouldReload(_CustomerStringsDelegate old) => false;
}
