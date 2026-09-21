import 'package:flutter/widgets.dart';
import 'package:goklay_core/goklay_core.dart';

/// The words the rider's app says on its own account.
///
/// Short, for the reason `goklay_core`'s table is short: the sentence about
/// why a feed is empty, what a job's status is, which distance band it falls
/// in and how much cash is owed are all composed by the server (2.9). What is
/// left is chrome and the two or three things that must be sayable before a
/// server answers.
abstract class PartnerStrings {
  /// The language this table is.
  String get languageTag;

  // ---- signing up and the shift ------------------------------------------

  /// The heading of the sign-up form.
  String get registerTitle;

  /// The line under it.
  String get registerSubtitle;

  /// The label on the name field.
  String get riderName;

  /// The label on the phone field.
  String get riderPhone;

  /// The label on the vehicle field.
  String get vehicle;

  /// Signs up.
  String get register;

  /// Goes on shift.
  String get goOnShift;

  /// Goes off shift.
  String get goOffShift;

  /// The heading above the distance choice.
  String get preference;

  /// Short deliveries only.
  String get preferenceShort;

  /// Long deliveries only.
  String get preferenceLong;

  /// Either.
  String get preferenceAny;

  /// The label above the reported position.
  String get yourLocation;

  /// Sends a new position.
  String get updateLocation;

  /// The label on the acceptance figure.
  String get acceptance;

  // ---- the feed and the job ----------------------------------------------

  /// The heading of the feed.
  String get feedTitle;

  /// Takes an offered delivery.
  String get acceptJob;

  /// Passes on one.
  String get declineJob;

  /// The heading of the jobs list.
  String get jobsTitle;

  /// The tab of deliveries being carried.
  String get jobsLive;

  /// The tab of finished ones.
  String get jobsPast;

  /// Shown over an empty jobs list.
  String get noJobs;

  /// The heading above where to collect.
  String get pickup;

  /// The heading above where to deliver.
  String get dropOff;

  /// Says the goods are in the bag.
  String get collect;

  /// Says they have been handed over.
  String get deliver;

  /// Says the delivery could not be completed.
  String get failJob;

  /// The label on the failure-reason field.
  String get failReason;

  /// The label on the pickup distance.
  String get toPickup;

  /// The label on the whole distance.
  String get wholeDistance;

  // ---- cash ---------------------------------------------------------------

  /// The heading of the ledger.
  String get cashTitle;

  /// The total still owed.
  String get outstanding;

  /// The total already handed over.
  String get remitted;

  /// The heading above the held collections.
  String get heldCollections;

  /// Shown when there is nothing to hand over.
  String get settled;

  // ---- offline ------------------------------------------------------------

  /// Shown when taps are waiting to be sent.
  String get waitingToSend;

  /// Sends them.
  String get sendNow;

  /// Shown when the server refused something that had been queued.
  String get queuedActionRefused;

  // ---- account ------------------------------------------------------------

  /// The heading of the account screen.
  String get accountTitle;

  /// Signs out.
  String get signOut;
}

/// Bengali. The default (1.4).
class PartnerStringsBn implements PartnerStrings {
  /// Creates the Bengali table.
  const PartnerStringsBn();

  @override
  String get languageTag => 'bn';
  @override
  String get registerTitle => 'রাইডার হিসেবে যোগ দিন';
  @override
  String get registerSubtitle =>
      'নাম আর মোবাইল নম্বর দিন। আপনি বাংলাদেশের যেকোনো জায়গায় কাজ করতে পারবেন।';
  @override
  String get riderName => 'আপনার নাম';
  @override
  String get riderPhone => 'মোবাইল নম্বর';
  @override
  String get vehicle => 'বাহন';
  @override
  String get register => 'যোগ দিন';
  @override
  String get goOnShift => 'কাজ শুরু করুন';
  @override
  String get goOffShift => 'কাজ বন্ধ করুন';
  @override
  String get preference => 'কেমন দূরত্ব নেবেন';
  @override
  String get preferenceShort => 'কাছের';
  @override
  String get preferenceLong => 'দূরের';
  @override
  String get preferenceAny => 'যেকোনো';
  @override
  String get yourLocation => 'আপনার অবস্থান';
  @override
  String get updateLocation => 'অবস্থান পাঠান';
  @override
  String get acceptance => 'গ্রহণের হার';
  @override
  String get feedTitle => 'নতুন ডেলিভারি';
  @override
  String get acceptJob => 'নিন';
  @override
  String get declineJob => 'বাদ দিন';
  @override
  String get jobsTitle => 'আমার ডেলিভারি';
  @override
  String get jobsLive => 'চলমান';
  @override
  String get jobsPast => 'আগের';
  @override
  String get noJobs => 'এখন কোনো ডেলিভারি নেই।';
  @override
  String get pickup => 'যেখান থেকে নেবেন';
  @override
  String get dropOff => 'যেখানে দেবেন';
  @override
  String get collect => 'মাল নিয়েছি';
  @override
  String get deliver => 'পৌঁছে দিয়েছি';
  @override
  String get failJob => 'দেওয়া যায়নি';
  @override
  String get failReason => 'কেন দেওয়া যায়নি?';
  @override
  String get toPickup => 'দোকান পর্যন্ত';
  @override
  String get wholeDistance => 'মোট দূরত্ব';
  @override
  String get cashTitle => 'নগদ হিসাব';
  @override
  String get outstanding => 'জমা দিতে হবে';
  @override
  String get remitted => 'জমা দেওয়া হয়েছে';
  @override
  String get heldCollections => 'আপনার কাছে যা আছে';
  @override
  String get settled => 'জমা দেওয়ার মতো কিছু নেই।';
  @override
  String get waitingToSend => 'পাঠানোর অপেক্ষায় আছে';
  @override
  String get sendNow => 'এখন পাঠান';
  @override
  String get queuedActionRefused => 'সার্ভার একটি কাজ গ্রহণ করেনি।';
  @override
  String get accountTitle => 'অ্যাকাউন্ট';
  @override
  String get signOut => 'সাইন আউট';
}

/// English, for `?lang=en`.
class PartnerStringsEn implements PartnerStrings {
  /// Creates the English table.
  const PartnerStringsEn();

  @override
  String get languageTag => 'en';
  @override
  String get registerTitle => 'Join as a rider';
  @override
  String get registerSubtitle =>
      'Your name and mobile number. You can work anywhere in Bangladesh.';
  @override
  String get riderName => 'Your name';
  @override
  String get riderPhone => 'Mobile number';
  @override
  String get vehicle => 'Vehicle';
  @override
  String get register => 'Join';
  @override
  String get goOnShift => 'Go on shift';
  @override
  String get goOffShift => 'Go off shift';
  @override
  String get preference => 'Distances you will take';
  @override
  String get preferenceShort => 'Short';
  @override
  String get preferenceLong => 'Long';
  @override
  String get preferenceAny => 'Either';
  @override
  String get yourLocation => 'Your location';
  @override
  String get updateLocation => 'Send location';
  @override
  String get acceptance => 'Acceptance rate';
  @override
  String get feedTitle => 'New deliveries';
  @override
  String get acceptJob => 'Take it';
  @override
  String get declineJob => 'Pass';
  @override
  String get jobsTitle => 'My deliveries';
  @override
  String get jobsLive => 'Current';
  @override
  String get jobsPast => 'Past';
  @override
  String get noJobs => 'No deliveries right now.';
  @override
  String get pickup => 'Collect from';
  @override
  String get dropOff => 'Deliver to';
  @override
  String get collect => 'I have it';
  @override
  String get deliver => 'Delivered';
  @override
  String get failJob => 'Could not deliver';
  @override
  String get failReason => 'Why could you not deliver it?';
  @override
  String get toPickup => 'To the shop';
  @override
  String get wholeDistance => 'Whole distance';
  @override
  String get cashTitle => 'Cash';
  @override
  String get outstanding => 'To hand over';
  @override
  String get remitted => 'Handed over';
  @override
  String get heldCollections => 'What you are carrying';
  @override
  String get settled => 'Nothing to hand over.';
  @override
  String get waitingToSend => 'Waiting to be sent';
  @override
  String get sendNow => 'Send now';
  @override
  String get queuedActionRefused => 'The server refused one of your taps.';
  @override
  String get accountTitle => 'Account';
  @override
  String get signOut => 'Sign out';
}

/// Looks the table up, Bengali-first.
abstract final class PartnerLocalizations {
  /// The delegate the app installs alongside `goklay_core`'s.
  static const LocalizationsDelegate<PartnerStrings> delegate =
      _PartnerStringsDelegate();

  /// The table for [locale]. Anything that is not English is Bengali.
  static PartnerStrings forLocale(Locale locale) => locale.languageCode == 'en'
      ? const PartnerStringsEn()
      : const PartnerStringsBn();

  /// The table in scope, falling back to Bengali outside a [MaterialApp].
  static PartnerStrings of(BuildContext context) =>
      Localizations.of<PartnerStrings>(context, PartnerStrings) ??
      const PartnerStringsBn();
}

class _PartnerStringsDelegate extends LocalizationsDelegate<PartnerStrings> {
  const _PartnerStringsDelegate();

  @override
  bool isSupported(Locale locale) => GoklayLocalizations.supportedLocales.any(
    (Locale supported) => supported.languageCode == locale.languageCode,
  );

  @override
  Future<PartnerStrings> load(Locale locale) async =>
      PartnerLocalizations.forLocale(locale);

  @override
  bool shouldReload(_PartnerStringsDelegate old) => false;
}
