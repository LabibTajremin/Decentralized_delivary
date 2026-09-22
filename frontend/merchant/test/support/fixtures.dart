/// The response bodies the fake backend answers with, in the shape
/// `api/openapi.yaml` specifies.
library;

/// A `Money` object.
Map<String, Object?> money(int minor, String display) => <String, Object?>{
  'minor': minor,
  'currency': 'BDT',
  'display': display,
};

/// A `RegistrationRequirements` response.
Map<String, Object?> requirementsJson() => <String, Object?>{
  'types': <Map<String, Object?>>[
    <String, Object?>{
      'type': 'restaurant',
      'required_documents': <String>['trade_licence', 'food_licence'],
    },
    <String, Object?>{
      'type': 'grocery',
      'required_documents': <String>['trade_licence'],
    },
    <String, Object?>{
      'type': 'pharmacy',
      'required_documents': <String>['trade_licence', 'drug_licence'],
    },
  ],
};

/// A `Merchant` response.
Map<String, Object?> merchantJson({
  String status = 'approved',
  bool canSubmit = false,
  List<String> missing = const <String>[],
  bool open = true,
  bool holiday = false,
  String reviewNote = '',
}) => <String, Object?>{
  'id': 'mch-1',
  'owner_user_id': 'usr-1',
  'name': 'নূরজাহান হোটেল',
  'type': 'restaurant',
  'status': status,
  'phone': '+8801712345678',
  'email': 'shop@example.com',
  'logo_url': '',
  'line1': '১২/এ, মিরপুর রোড',
  'line2': '',
  'single_line': '১২/এ, মিরপুর রোড, ধানমন্ডি, ঢাকা',
  'lat': 23.7509,
  'lng': 90.3925,
  'area_code': 'DHK-DHM',
  'area_name': 'ধানমন্ডি',
  'district_code': 'DHK',
  'division_code': 'DHA',
  'is_listed': status == 'approved' && !holiday,
  'is_open_now': open && status == 'approved' && !holiday,
  'open_status': open ? 'খোলা আছে — বন্ধ হবে 22:00' : 'বন্ধ',
  'hours': <String, Object?>{
    '0': <String>['09:00-22:00'],
    '1': <String>['09:00-22:00'],
  },
  if (holiday)
    'holiday': <String, Object?>{
      'until': '2026-09-25T00:00:00Z',
      'reason': 'ঈদের ছুটি',
    },
  'documents': <Map<String, Object?>>[
    <String, Object?>{
      'kind': 'trade_licence',
      'number': 'TL-99',
      'file_url': '/static/tl.pdf',
      'uploaded_at': '2026-09-20T10:00:00Z',
    },
  ],
  'required_documents': <String>['trade_licence', 'food_licence'],
  'missing_documents': missing,
  'can_submit': canSubmit,
  'next_statuses': <String>['pending_review'],
  if (reviewNote.isNotEmpty) 'review_note': reviewNote,
  'created_at': '2026-09-20T09:00:00Z',
};

/// A `CatalogueCapabilities` response.
Map<String, Object?> capabilitiesJson({String type = 'restaurant'}) =>
    switch (type) {
      'grocery' => <String, Object?>{
        'merchant_type': 'grocery',
        'variants': true,
        'addons': false,
        'combos': false,
        'tracks_stock': true,
        'requires_unit': true,
        'prescriptions': false,
        'units': <String>['piece', 'kg', 'litre'],
      },
      'pharmacy' => <String, Object?>{
        'merchant_type': 'pharmacy',
        'variants': true,
        'addons': false,
        'combos': false,
        'tracks_stock': true,
        'requires_unit': true,
        'prescriptions': true,
        'units': <String>['strip', 'bottle'],
      },
      _ => <String, Object?>{
        'merchant_type': 'restaurant',
        'variants': true,
        'addons': true,
        'combos': true,
        'tracks_stock': false,
        'requires_unit': false,
        'prescriptions': false,
        'units': <String>[],
      },
    };

/// A `Category` object.
Map<String, Object?> categoryJson({
  String id = 'cat-1',
  String name = 'বিরিয়ানি',
  bool active = true,
}) => <String, Object?>{
  'id': id,
  'name': name,
  'sort_order': 1,
  'active': active,
};

/// An owner's `Item` object.
Map<String, Object?> itemJson({
  String id = 'itm-1',
  bool active = true,
  bool orderable = true,
  bool stockTracked = false,
  int? stock,
}) => <String, Object?>{
  'id': id,
  'merchant_id': 'mch-1',
  'category_id': 'cat-1',
  'name': 'কাচ্চি',
  'description': 'বাসমতি চালের কাচ্চি',
  'image_url': '',
  'price': money(32000, '৳ ৩২০'),
  'orderable': orderable,
  if (!orderable) 'unavailable_reason': 'out_of_stock',
  'unit': stockTracked ? 'piece' : '',
  'pack_size': '',
  'brand': '',
  'generic_name': '',
  'strength': '',
  'requires_prescription': false,
  'is_vegetarian': false,
  'preparation_minutes': 25,
  'variant_groups': const <Object?>[],
  'addon_groups': const <Object?>[],
  'active': active,
  'stock_tracked': stockTracked,
  'stock_quantity': stock,
  'sort_order': 1,
};

/// A `Combo` object.
Map<String, Object?> comboJson({bool active = true, bool orderable = true}) =>
    <String, Object?>{
      'id': 'cmb-1',
      'merchant_id': 'mch-1',
      'name': 'পরিবার প্যাক',
      'description': '',
      'image_url': '',
      'price': money(60000, '৳ ৬০০'),
      'lines': <Map<String, Object?>>[
        <String, Object?>{
          'item_id': 'itm-1',
          'name': 'কাচ্চি',
          'quantity': 2,
        },
      ],
      'orderable': orderable,
      'active': active,
      'sort_order': 1,
    };

/// An `Order` as the shop sees it.
Map<String, Object?> orderJson({
  String id = 'ord-1',
  String status = 'placed',
  bool live = true,
  String payment = 'cash',
  List<String> nextActions = const <String>['accepted', 'rejected'],
}) => <String, Object?>{
  'id': id,
  'code': 'GK-7F3K',
  'merchant_id': 'mch-1',
  'status': status,
  'status_label': 'নতুন অর্ডার',
  'live': live,
  'payment_method': payment,
  'lines': <Map<String, Object?>>[
    <String, Object?>{
      'id': 'lin-1',
      'kind': 'item',
      'target_id': 'itm-1',
      'name': 'কাচ্চি',
      'options': <Map<String, Object?>>[
        <String, Object?>{'name': 'বড়', 'price': money(5000, '৳ ৫০')},
      ],
      'quantity': 2,
      'note': 'ঝাল কম',
      'unit_price': money(32000, '৳ ৩২০'),
      'line_total': money(64000, '৳ ৬৪০'),
    },
  ],
  'count': 2,
  'subtotal': money(64000, '৳ ৬৪০'),
  'delivery': money(6000, '৳ ৬০'),
  'total': money(70000, '৳ ৭০০'),
  'receipt': <Map<String, Object?>>[
    <String, Object?>{
      'key': 'subtotal',
      'label': 'সাবটোটাল',
      'amount': money(64000, '৳ ৬৪০'),
    },
  ],
  'expanded': false,
  'distance_m': 1200.0,
  'pickup': <String, Object?>{
    'name': 'নূরজাহান হোটেল',
    'phone': '01799999999',
    'single_line': '১২/এ, মিরপুর রোড',
    'lat': 23.75,
    'lng': 90.39,
  },
  'destination': <String, Object?>{
    'name': 'রিয়া',
    'phone': '01712345678',
    'single_line': 'রোড ৫, ধানমন্ডি, ঢাকা',
    'lat': 23.7461,
    'lng': 90.3742,
  },
  'events': <Map<String, Object?>>[
    <String, Object?>{
      'status': 'placed',
      'label': 'অর্ডার হয়েছে',
      'actor': 'customer',
      'at': '2026-09-21T10:00:00Z',
    },
  ],
  'next_actions': nextActions,
  'cancel': <String, Object?>{'allowed': false, 'seconds_left': 0},
  'placed_at': '2026-09-21T10:00:00Z',
  'updated_at': '2026-09-21T10:00:00Z',
};

/// An `OrderList` response.
Map<String, Object?> orderListJson({
  List<Map<String, Object?>>? orders,
}) => <String, Object?>{
  'orders': orders ?? <Map<String, Object?>>[orderJson()],
  'total': orders?.length ?? 1,
};

/// A `Profile` response.
Map<String, Object?> profileJson({String name = 'করিম'}) => <String, Object?>{
  'user_id': 'usr-1',
  'name': name,
  'display_name': name,
  'email': '',
  'language': 'bn',
};

/// A `TokenPair` response.
Map<String, Object?> tokenPairJson({String role = 'merchant'}) =>
    <String, Object?>{
      'access_token': 'access-1',
      'refresh_token': 'refresh-1',
      'token_type': 'Bearer',
      'expires_in': 900,
      'role': role,
      'user_id': 'usr-1',
      'session_id': 'ses-1',
    };
