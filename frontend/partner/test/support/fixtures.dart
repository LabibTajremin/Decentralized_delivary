/// The response bodies the fake backend answers with.
library;

/// A `Money` object.
Map<String, Object?> money(int minor, String display) => <String, Object?>{
  'minor': minor,
  'currency': 'BDT',
  'display': display,
};

/// A `DeliveryPartner` response.
Map<String, Object?> partnerJson({
  String availability = 'available',
  String preference = 'any',
  int carrying = 0,
}) => <String, Object?>{
  'id': 'ptn-1',
  'name': 'করিম',
  'phone': '01811111111',
  'vehicle': 'bike',
  'availability': availability,
  'availability_label': switch (availability) {
    'offline' => 'আপনি এখন অফলাইন',
    'busy' => 'আপনি এখন ব্যস্ত',
    _ => 'কাজের জন্য প্রস্তুত',
  },
  'preference': preference,
  'preference_label': switch (preference) {
    'short' => 'কাছের ডেলিভারি',
    'long' => 'দূরের ডেলিভারি',
    _ => 'যেকোনো দূরত্ব',
  },
  'lat': 23.75,
  'lng': 90.38,
  'carrying': carrying,
  'acceptance_percent': 82,
};

/// A `DeliveryJob` object.
Map<String, Object?> jobJson({
  String id = 'job-1',
  String status = 'offered',
  bool live = true,
  String band = 'short',
  String reason = '',
}) => <String, Object?>{
  'id': id,
  'order_id': 'ord-1',
  'code': 'GK-7F3K',
  'status': status,
  'status_label': switch (status) {
    'offered' => 'আপনাকে দেওয়া হয়েছে',
    'assigned' => 'আপনি নিয়েছেন',
    'collected' => 'আপনার কাছে আছে',
    'delivered' => 'পৌঁছে দেওয়া হয়েছে',
    _ => 'শেষ',
  },
  'live': live,
  'band': band,
  'band_label': band == 'short' ? 'কাছের' : 'দূরের',
  'distance_m': 3200.0,
  'distance': '৩.২ কিমি',
  'to_pickup_m': 400.0,
  'to_pickup': '৪০০ মিটার',
  'pickup': <String, Object?>{
    'name': 'নূরজাহান হোটেল',
    'phone': '01799999999',
    'single_line': '১২/এ, মিরপুর রোড',
    'lat': 23.7509,
    'lng': 90.3925,
  },
  'destination': <String, Object?>{
    'name': 'রিয়া',
    'phone': '01712345678',
    'single_line': 'রোড ৫, ধানমন্ডি, ঢাকা',
    'lat': 23.7461,
    'lng': 90.3742,
  },
  if (reason.isNotEmpty) 'reason': reason,
  'seconds_left': 45,
};

/// A `PartnerFeed` response.
Map<String, Object?> feedJson({
  List<Map<String, Object?>>? jobs,
  String availability = 'available',
  String? reason,
  String notice = '',
}) => <String, Object?>{
  'partner': partnerJson(availability: availability),
  'jobs': jobs ?? <Map<String, Object?>>[jobJson()],
  'radius_m': 5000.0,
  'reason': ?reason,
  'notice': notice,
};

/// A `DeliveryJobList` response.
Map<String, Object?> jobListJson({List<Map<String, Object?>>? jobs}) =>
    <String, Object?>{
      'jobs': jobs ?? <Map<String, Object?>>[jobJson(status: 'collected')],
      'total': jobs?.length ?? 1,
    };

/// A `Ledger` response.
Map<String, Object?> ledgerJson({bool settled = false}) => <String, Object?>{
  'partner_id': 'ptn-1',
  'outstanding': money(settled ? 0 : 70000, settled ? '৳ ০' : '৳ ৭০০'),
  'remitted': money(120000, '৳ ১,২০০'),
  'held': settled
      ? const <Object?>[]
      : <Map<String, Object?>>[
          <String, Object?>{
            'id': 'col-1',
            'order_id': 'ord-1',
            'amount': money(70000, '৳ ৭০০'),
            'status': 'held',
            'status_label': 'আপনার কাছে আছে',
          },
        ],
};

/// A `Profile` response.
Map<String, Object?> profileJson() => <String, Object?>{
  'user_id': 'usr-1',
  'name': 'করিম',
  'display_name': 'করিম',
  'email': '',
  'language': 'bn',
};

/// A `TokenPair` response.
Map<String, Object?> tokenPairJson({String role = 'partner'}) =>
    <String, Object?>{
      'access_token': 'access-1',
      'refresh_token': 'refresh-1',
      'token_type': 'Bearer',
      'expires_in': 900,
      'role': role,
      'user_id': 'usr-1',
      'session_id': 'ses-1',
    };
