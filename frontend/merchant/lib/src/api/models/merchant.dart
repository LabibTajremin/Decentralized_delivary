import 'package:flutter/foundation.dart';
import 'package:goklay_core/goklay_core.dart';

/// A document the shop had to upload before it could be reviewed.
@immutable
class MerchantDocument {
  /// Creates a document.
  const MerchantDocument({
    required this.kind,
    required this.number,
    required this.fileUrl,
  });

  /// Reads the `MerchantDocument` object.
  factory MerchantDocument.fromJson(Map<String, Object?> json) =>
      MerchantDocument(
        kind: readString(json, 'kind'),
        number: readString(json, 'number'),
        fileUrl: readString(json, 'file_url'),
      );

  /// `trade_licence`, `national_id`, `food_licence` or `drug_licence`.
  final String kind;

  /// The number written on it.
  final String number;

  /// Where the scan lives.
  final String fileUrl;
}

/// The holiday a shop has closed itself for, if any.
@immutable
class MerchantHoliday {
  /// Creates a holiday.
  const MerchantHoliday({required this.until, required this.reason});

  /// Reads the `holiday` object.
  factory MerchantHoliday.fromJson(Map<String, Object?> json) =>
      MerchantHoliday(
        until: DateTime.tryParse(readString(json, 'until')),
        reason: readString(json, 'reason'),
      );

  /// When the shop reopens, or null for indefinitely.
  final DateTime? until;

  /// What the owner said.
  final String reason;
}

/// A shop, as its owner sees it.
///
/// Four fields on here are the whole of the app's decision-making, and none of
/// them is computed locally: [canSubmit] says whether the "submit for review"
/// button is live, [missingDocuments] says what is still wanted, [nextStatuses]
/// says which moves the workflow will accept, and [openStatus] is the sentence
/// about opening hours — which the app never derives from [hours], because the
/// server owns holidays, the clock and the timezone.
@immutable
class Merchant {
  /// Creates a shop.
  const Merchant({
    required this.id,
    required this.name,
    required this.type,
    required this.status,
    required this.phone,
    required this.email,
    required this.line1,
    required this.line2,
    required this.singleLine,
    required this.lat,
    required this.lng,
    required this.areaName,
    required this.divisionCode,
    required this.isListed,
    required this.isOpenNow,
    required this.openStatus,
    required this.hours,
    required this.holiday,
    required this.documents,
    required this.requiredDocuments,
    required this.missingDocuments,
    required this.canSubmit,
    required this.nextStatuses,
    required this.reviewNote,
  });

  /// Reads the `Merchant` response.
  factory Merchant.fromJson(Map<String, Object?> json) {
    final Map<String, Object?> holiday = readObject(json, 'holiday');
    return Merchant(
      id: readString(json, 'id'),
      name: readString(json, 'name'),
      type: readString(json, 'type'),
      status: readString(json, 'status'),
      phone: readString(json, 'phone'),
      email: readString(json, 'email'),
      line1: readString(json, 'line1'),
      line2: readString(json, 'line2'),
      singleLine: readString(json, 'single_line'),
      lat: readDouble(json, 'lat'),
      lng: readDouble(json, 'lng'),
      areaName: readString(json, 'area_name'),
      divisionCode: readString(json, 'division_code'),
      isListed: readBool(json, 'is_listed'),
      isOpenNow: readBool(json, 'is_open_now'),
      openStatus: readString(json, 'open_status'),
      hours: _hours(readObject(json, 'hours')),
      holiday: holiday.isEmpty ? null : MerchantHoliday.fromJson(holiday),
      documents: readList(json, 'documents', MerchantDocument.fromJson),
      requiredDocuments: _strings(json['required_documents']),
      missingDocuments: _strings(json['missing_documents']),
      canSubmit: readBool(json, 'can_submit'),
      nextStatuses: _strings(json['next_statuses']),
      reviewNote: readString(json, 'review_note'),
    );
  }

  /// The shop's id, which every catalogue and order path needs.
  final String id;

  /// Its name.
  final String name;

  /// `restaurant`, `grocery` or `pharmacy`. Decides what its catalogue may
  /// contain — but the app asks the capabilities endpoint rather than
  /// branching on this itself.
  final String type;

  /// `draft`, `pending_review`, `approved`, `rejected` or `suspended`.
  final String status;

  /// The shop's phone.
  final String phone;

  /// Its email, or empty.
  final String email;

  /// The street line.
  final String line1;

  /// The second line, or empty.
  final String line2;

  /// The whole address on one line, composed by the server.
  final String singleLine;

  /// Where the rider collects from.
  final double lat;

  /// Where the rider collects from.
  final double lng;

  /// The area it resolved to.
  final String areaName;

  /// **D3.** A customer outside this division can never see the shop, whatever
  /// the radius settings say.
  final String divisionCode;

  /// Approved and not on holiday.
  final bool isListed;

  /// Listed and inside an opening window.
  final bool isOpenNow;

  /// The sentence to show about that, composed by the server.
  final String openStatus;

  /// Opening windows by weekday number, Sunday as `"0"`.
  final Map<String, List<String>> hours;

  /// The holiday the shop has closed itself for, or null.
  final MerchantHoliday? holiday;

  /// What has been uploaded.
  final List<MerchantDocument> documents;

  /// What this shop type needs.
  final List<String> requiredDocuments;

  /// What is still wanted. The server works this out; the app does not
  /// subtract one list from the other.
  final List<String> missingDocuments;

  /// **Whether "submit for review" is live.**
  final bool canSubmit;

  /// The statuses the workflow will accept from here.
  final List<String> nextStatuses;

  /// Why the shop was rejected or suspended. Empty otherwise.
  final String reviewNote;

  /// Whether the shop is still being filled in.
  bool get isDraft => status == 'draft';

  /// Whether an admin is looking at it.
  bool get isAwaitingReview => status == 'pending_review';

  /// Whether it may trade.
  bool get isApproved => status == 'approved';

  /// Whether the owner has been told something went wrong.
  bool get hasReviewNote => reviewNote.isNotEmpty;

  static List<String> _strings(Object? raw) => <String>[
    if (raw is List<Object?>)
      for (final Object? entry in raw)
        if (entry is String) entry,
  ];

  static Map<String, List<String>> _hours(Map<String, Object?> json) =>
      <String, List<String>>{
        for (final MapEntry<String, Object?> entry in json.entries)
          entry.key: _strings(entry.value),
      };
}

/// What the registration form has to ask for, per shop type.
///
/// Served rather than hardcoded: which documents a pharmacy needs is a rule,
/// and a rule in the app is a rule that goes stale when the regulator changes
/// their mind (2.9).
@immutable
class RegistrationRequirements {
  /// Creates the requirements.
  const RegistrationRequirements({required this.byType});

  /// Reads the `RegistrationRequirements` response.
  factory RegistrationRequirements.fromJson(Map<String, Object?> json) {
    final Object? raw = json['types'];
    return RegistrationRequirements(
      byType: <String, List<String>>{
        if (raw is List<Object?>)
          for (final Object? entry in raw)
            if (entry is Map<String, Object?>)
              readString(entry, 'type'): Merchant._strings(
                entry['required_documents'],
              ),
      },
    );
  }

  /// The documents each shop type must upload.
  final Map<String, List<String>> byType;

  /// The types a shop may register as, in the order the server listed them.
  List<String> get types => byType.keys.toList();
}
