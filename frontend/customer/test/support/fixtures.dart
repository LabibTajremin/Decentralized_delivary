/// The response bodies the fake backend answers with.
///
/// They are written out in full, in the shape `api/openapi.yaml` specifies,
/// rather than being built by a helper — a fixture that a builder trimmed is
/// how a parser ends up tested against a body no server would ever send.
library;

/// A `Money` object.
Map<String, Object?> money(int minor, String display) => <String, Object?>{
  'minor': minor,
  'currency': 'BDT',
  'display': display,
};

/// A `DiscoveryMerchant`.
Map<String, Object?> merchantJson({
  String id = 'mer-1',
  String name = 'Kacchi Bhai',
  String type = 'restaurant',
  bool open = true,
}) => <String, Object?>{
  'id': id,
  'name': name,
  'type': type,
  'logo_url': 'http://cdn.test/$id.png',
  'area_name': 'ধানমন্ডি',
  'distance_m': 1200.0,
  'distance': '১.২ কিমি',
  'is_open_now': open,
  'open_status': open ? 'খোলা আছে' : 'বন্ধ',
  'delivery': <String, Object?>{
    'minor': 6000,
    'currency': 'BDT',
    'display': '৳ ৬০',
    'expanded': false,
  },
};

/// A `DiscoverySearch`.
Map<String, Object?> searchJson({
  List<Map<String, Object?>>? merchants,
  bool canExpand = true,
  bool atCeiling = false,
  String notice = 'কাছের দোকান',
}) => <String, Object?>{
  'area_name': 'ধানমন্ডি',
  'notice': notice,
  'total': merchants?.length ?? 1,
  'expansion': <String, Object?>{
    'level': 0,
    'radius': '৩ কিমি',
    'stage': 'base',
    'can_expand': canExpand,
    'offered': false,
    'at_ceiling': atCeiling,
    'next_level': 1,
    'next_radius': '৫ কিমি',
  },
  'merchants': merchants ?? <Map<String, Object?>>[merchantJson()],
};

/// An `Address`.
Map<String, Object?> addressJson({
  String id = 'adr-1',
  String label = 'বাসা',
  bool isDefault = true,
}) => <String, Object?>{
  'id': id,
  'label': label,
  'recipient_name': 'রিয়া',
  'recipient_phone': '01712345678',
  'line1': 'রোড ৫, ধানমন্ডি',
  'line2': '',
  'instructions': 'নীল গেট',
  'single_line': 'রোড ৫, ধানমন্ডি, ঢাকা',
  'lat': 23.7461,
  'lng': 90.3742,
  'area_code': 'DHK-DHM',
  'area_name': 'ধানমন্ডি',
  'district_code': 'DHK',
  'division_code': 'DHA',
  'is_default': isDefault,
};

/// A `Profile`.
Map<String, Object?> profileJson({String name = 'রিয়া'}) => <String, Object?>{
  'user_id': 'usr-1',
  'name': name,
  'display_name': name.isEmpty ? 'অতিথি' : name,
  'email': 'riya@example.com',
  'language': 'bn',
};

/// An `OptionGroup`.
Map<String, Object?> groupJson({
  String id = 'grp-1',
  bool required = true,
  int max = 1,
  bool available = true,
}) => <String, Object?>{
  'id': id,
  'name': 'সাইজ',
  'required': required,
  'min_choices': required ? 1 : 0,
  'max_choices': max,
  'options': <Map<String, Object?>>[
    <String, Object?>{
      'id': '$id-a',
      'name': 'ছোট',
      'price': money(0, '৳ ০'),
      'available': available,
    },
    <String, Object?>{
      'id': '$id-b',
      'name': 'বড়',
      'price': money(5000, '৳ ৫০'),
      'available': true,
    },
  ],
};

/// A `PublicItem`.
Map<String, Object?> itemJson({
  String id = 'itm-1',
  bool orderable = true,
  List<Map<String, Object?>> variantGroups = const <Map<String, Object?>>[],
}) => <String, Object?>{
  'id': id,
  'merchant_id': 'mer-1',
  'category_id': 'cat-1',
  'name': 'কাচ্চি',
  'description': 'বাসমতি চালের কাচ্চি',
  'image_url': '',
  'price': money(32000, '৳ ৩২০'),
  'orderable': orderable,
  if (!orderable) 'unavailable_reason': 'out_of_stock',
  'unit': '',
  'pack_size': '',
  'brand': '',
  'generic_name': '',
  'strength': '',
  'requires_prescription': false,
  'is_vegetarian': false,
  'preparation_minutes': 25,
  'variant_groups': variantGroups,
  'addon_groups': const <Map<String, Object?>>[],
};

/// A `Menu`.
Map<String, Object?> menuJson({
  List<Map<String, Object?>>? items,
  List<Map<String, Object?>>? combos,
}) => <String, Object?>{
  'merchant_id': 'mer-1',
  'categories': <Map<String, Object?>>[
    <String, Object?>{
      'id': 'cat-1',
      'name': 'বিরিয়ানি',
      'sort_order': 1,
      'active': true,
    },
  ],
  'items': items ?? <Map<String, Object?>>[itemJson()],
  'combos': combos ?? const <Map<String, Object?>>[],
};

/// A `Combo`.
Map<String, Object?> comboJson({bool withSavings = true}) => <String, Object?>{
  'id': 'cmb-1',
  'merchant_id': 'mer-1',
  'name': 'পরিবার প্যাক',
  'description': '',
  'image_url': '',
  'price': money(60000, '৳ ৬০০'),
  if (withSavings) 'savings': money(4000, '৳ ৪০'),
  'lines': <Map<String, Object?>>[
    <String, Object?>{'item_id': 'itm-1', 'name': 'কাচ্চি', 'quantity': 2},
  ],
  'orderable': true,
  'active': true,
  'sort_order': 1,
};

/// A `ReceiptRow` list.
List<Map<String, Object?>> receiptJson() => <Map<String, Object?>>[
  <String, Object?>{
    'key': 'subtotal',
    'label': 'সাবটোটাল',
    'amount': money(32000, '৳ ৩২০'),
  },
  <String, Object?>{
    'key': 'delivery',
    'label': 'ডেলিভারি',
    'amount': money(6000, '৳ ৬০'),
  },
];

/// A `Cart`.
Map<String, Object?> cartJson({
  bool orderable = true,
  bool empty = false,
  bool lineHasIssue = false,
  String? addressId,
  int quantity = 1,
}) => <String, Object?>{
  'id': 'crt-1',
  'merchant_id': 'mer-1',
  'merchant_name': 'Kacchi Bhai',
  'merchant_logo_url': '',
  'address_id': ?addressId,
  'lines': empty
      ? const <Map<String, Object?>>[]
      : <Map<String, Object?>>[
          <String, Object?>{
            'id': 'lin-1',
            'name': 'কাচ্চি',
            'options': <Map<String, Object?>>[
              <String, Object?>{'name': 'বড়', 'price': money(5000, '৳ ৫০')},
            ],
            'quantity': quantity,
            'note': 'ঝাল কম',
            'line_total': money(37000, '৳ ৩৭০'),
            if (lineHasIssue) 'issue': 'out_of_stock',
            'issue_text': lineHasIssue ? 'স্টক শেষ' : '',
            'orderable': !lineHasIssue,
          },
        ],
  'count': empty ? 0 : quantity,
  'orderable': orderable,
  if (!orderable) 'blocker': 'shop_closed',
  'blocker_text': orderable ? '' : 'দোকান বন্ধ',
  'pricing': <String, Object?>{
    'total': money(43000, '৳ ৪৩০'),
    'free_delivery': false,
    'away_from_free_delivery': money(7000, '৳ ৭০'),
    'expanded': false,
    'rows': receiptJson(),
    'notice': 'আর ৳ ৭০ যোগ করলে ডেলিভারি ফ্রি',
  },
};

/// An `Order`.
Map<String, Object?> orderJson({
  String id = 'ord-1',
  String status = 'preparing',
  bool live = true,
  String payment = 'cash',
  String? partnerId,
}) => <String, Object?>{
  'id': id,
  'code': 'GK-7F3K',
  'merchant_id': 'mer-1',
  'partner_id': ?partnerId,
  'status': status,
  'status_label': 'রান্না হচ্ছে',
  'live': live,
  'payment_method': payment,
  'lines': <Map<String, Object?>>[
    <String, Object?>{
      'id': 'lin-1',
      'kind': 'item',
      'target_id': 'itm-1',
      'name': 'কাচ্চি',
      'quantity': 1,
      'note': '',
      'unit_price': money(32000, '৳ ৩২০'),
      'line_total': money(32000, '৳ ৩২০'),
    },
  ],
  'count': 1,
  'subtotal': money(32000, '৳ ৩২০'),
  'delivery': money(6000, '৳ ৬০'),
  'total': money(38000, '৳ ৩৮০'),
  'receipt': receiptJson(),
  'expanded': false,
  'distance_m': 1200.0,
  'pickup': <String, Object?>{
    'name': 'Kacchi Bhai',
    'phone': '01799999999',
    'single_line': 'রোড ২, ধানমন্ডি',
    'lat': 23.74,
    'lng': 90.37,
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
    <String, Object?>{
      'status': 'preparing',
      'label': 'রান্না হচ্ছে',
      'actor': 'merchant',
      'reason': '',
      'at': '2026-09-21T10:05:00Z',
    },
  ],
  'next_actions': <String>['cancelled'],
  'cancel': cancellationJson(),
  'placed_at': '2026-09-21T10:00:00Z',
  'updated_at': '2026-09-21T10:05:00Z',
};

/// An `OrderCancellation`.
Map<String, Object?> cancellationJson({bool allowed = true}) =>
    <String, Object?>{
      'allowed': allowed,
      if (!allowed) 'reason': 'window_closed',
      'text': allowed ? 'আর ৪ মিনিট বাতিল করা যাবে' : 'আর বাতিল করা যাবে না',
      'seconds_left': allowed ? 240 : 0,
    };

/// A `Checkout`.
Map<String, Object?> checkoutJson({String redirect = ''}) => <String, Object?>{
  'payment_id': 'pay-1',
  'order_id': 'ord-1',
  'status': 'pending',
  'status_label': 'অপেক্ষমাণ',
  'amount': money(38000, '৳ ৩৮০'),
  if (redirect.isNotEmpty) 'redirect_url': redirect,
};

/// A `Payment`.
Map<String, Object?> paymentJson({
  String status = 'captured',
  String reason = '',
}) => <String, Object?>{
  'id': 'pay-1',
  'order_id': 'ord-1',
  'status': status,
  'status_label': status == 'captured' ? 'পেমেন্ট হয়েছে' : 'ব্যর্থ',
  'amount': money(38000, '৳ ৩৮০'),
  if (reason.isNotEmpty) 'reason': reason,
};

/// A `Review`.
Map<String, Object?> reviewJson({int rating = 5, String comment = 'ভালো'}) =>
    <String, Object?>{
      'id': 'rev-1',
      'order_id': 'ord-1',
      'subject': 'merchant',
      'subject_id': 'mer-1',
      'rating': rating,
      'comment': comment,
      'created_at': '2026-09-21T11:00:00Z',
    };

/// A `Rating`.
Map<String, Object?> ratingJson({int count = 4}) => <String, Object?>{
  'subject': 'merchant',
  'subject_id': 'mer-1',
  'average': 4.5,
  'count': count,
};

/// A `SupportTicket`.
Map<String, Object?> ticketJson({String status = 'open'}) => <String, Object?>{
  'id': 'tkt-1',
  'order_id': 'ord-1',
  'subject': 'খাবার ঠান্ডা ছিল',
  'status': status,
  if (status == 'resolved') 'resolution': 'refunded',
  'note': status == 'resolved' ? 'ফেরত দেওয়া হয়েছে' : '',
  'created_at': '2026-09-21T11:00:00Z',
  if (status == 'resolved') 'resolved_at': '2026-09-21T12:00:00Z',
};

/// A `Notification`.
Map<String, Object?> notificationJson({String status = 'sent'}) =>
    <String, Object?>{
      'id': 'ntf-1',
      'title': 'অর্ডার গ্রহণ হয়েছে',
      'body': 'দোকান আপনার অর্ডার নিয়েছে',
      'channel': 'push',
      'status': status,
      'created_at': '2026-09-21T10:01:00Z',
    };

/// A `TokenPair`.
Map<String, Object?> tokenPairJson({String role = 'customer'}) =>
    <String, Object?>{
      'access_token': 'access-1',
      'refresh_token': 'refresh-1',
      'token_type': 'Bearer',
      'expires_in': 900,
      'role': role,
      'user_id': 'usr-1',
      'session_id': 'ses-1',
      'new_user': false,
    };

/// An `Area`.
Map<String, Object?> areaJson() => <String, Object?>{
  'area_code': 'DHK-DHM',
  'area_name': 'ধানমন্ডি',
  'district_code': 'DHK',
  'division_code': 'DHA',
  'division_name': 'ঢাকা',
};
