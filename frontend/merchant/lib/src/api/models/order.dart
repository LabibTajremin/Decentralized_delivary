import 'package:flutter/foundation.dart';
import 'package:goklay_core/goklay_core.dart';

/// One end of the delivery, as the shop's copy of the order shows it.
@immutable
class OrderPlace {
  /// Creates a place.
  const OrderPlace({
    required this.name,
    required this.phone,
    required this.singleLine,
  });

  /// Reads the `OrderPlace` object.
  factory OrderPlace.fromJson(Map<String, Object?> json) => OrderPlace(
    name: readString(json, 'name'),
    phone: readString(json, 'phone'),
    singleLine: readString(json, 'single_line'),
  );

  /// Who is there.
  final String name;

  /// A number to call.
  final String phone;

  /// The address on one line, composed by the server.
  final String singleLine;
}

/// One line of an order, as the kitchen needs it.
@immutable
class OrderLine {
  /// Creates a line.
  const OrderLine({
    required this.name,
    required this.quantity,
    required this.note,
    required this.options,
    required this.lineTotal,
  });

  /// Reads the `OrderLine` object.
  factory OrderLine.fromJson(Map<String, Object?> json) {
    final Object? options = json['options'];
    return OrderLine(
      name: readString(json, 'name'),
      quantity: readInt(json, 'quantity'),
      note: readString(json, 'note'),
      options: <String>[
        if (options is List<Object?>)
          for (final Object? option in options)
            if (option is Map<String, Object?>) readString(option, 'name'),
      ],
      lineTotal: Money.fromJson(readObject(json, 'line_total')),
    );
  }

  /// What was ordered.
  final String name;

  /// How many.
  final int quantity;

  /// What the customer asked for. The one free-text field the kitchen reads.
  final String note;

  /// The chosen options, by name.
  final List<String> options;

  /// What the line came to.
  final Money lineTotal;
}

/// One entry in the order's history.
@immutable
class OrderEvent {
  /// Creates an event.
  const OrderEvent({
    required this.status,
    required this.label,
    required this.reason,
  });

  /// Reads the `OrderEvent` object.
  factory OrderEvent.fromJson(Map<String, Object?> json) => OrderEvent(
    status: readString(json, 'status'),
    label: readString(json, 'label'),
    reason: readString(json, 'reason'),
  );

  /// The state it moved to.
  final String status;

  /// The line to print, composed by the server.
  final String label;

  /// Why, where there is a why.
  final String reason;
}

/// An order on the shop's board.
///
/// [nextActions] is the whole of the board's behaviour: it is what the order
/// state machine will accept **from this shop, right now**, and the buttons
/// are built from it. An app with its own copy of the transition table would
/// offer Accept on an order the customer had already cancelled.
@immutable
class MerchantOrder {
  /// Creates an order.
  const MerchantOrder({
    required this.id,
    required this.code,
    required this.status,
    required this.statusLabel,
    required this.live,
    required this.paymentMethod,
    required this.count,
    required this.total,
    required this.lines,
    required this.receipt,
    required this.destination,
    required this.events,
    required this.nextActions,
    required this.placedAt,
  });

  /// Reads the `Order` response.
  factory MerchantOrder.fromJson(Map<String, Object?> json) {
    final Object? actions = json['next_actions'];
    return MerchantOrder(
      id: readString(json, 'id'),
      code: readString(json, 'code'),
      status: readString(json, 'status'),
      statusLabel: readString(json, 'status_label'),
      live: readBool(json, 'live'),
      paymentMethod: readString(json, 'payment_method'),
      count: readInt(json, 'count'),
      total: Money.fromJson(readObject(json, 'total')),
      lines: readList(json, 'lines', OrderLine.fromJson),
      receipt: readList(json, 'receipt', ReceiptRow.fromJson),
      destination: OrderPlace.fromJson(readObject(json, 'destination')),
      events: readList(json, 'events', OrderEvent.fromJson),
      nextActions: <String>[
        if (actions is List<Object?>)
          for (final Object? action in actions)
            if (action is String) action,
      ],
      placedAt: DateTime.tryParse(readString(json, 'placed_at')),
    );
  }

  /// The order's id.
  final String id;

  /// The short code the shop and the customer say to each other.
  final String code;

  /// Its state, as a code.
  final String status;

  /// Its state as a sentence, composed by the server.
  final String statusLabel;

  /// Whether it is still going.
  final bool live;

  /// `cash` or `online` — which tells the shop whether to take money at the
  /// door. The amount is [total]; the shop is not asked to work anything out.
  final String paymentMethod;

  /// How many items.
  final int count;

  /// What it came to.
  final Money total;

  /// What to make.
  final List<OrderLine> lines;

  /// The receipt, already composed.
  final List<ReceiptRow> receipt;

  /// Where it goes.
  final OrderPlace destination;

  /// The history.
  final List<OrderEvent> events;

  /// **The transitions this shop may make right now.** The board's buttons
  /// are built from this and nothing else.
  final List<String> nextActions;

  /// When it was placed.
  final DateTime? placedAt;

  /// The status a shop moves an order to when it takes it.
  static const String accepted = 'accepted';

  /// The status for "we have started".
  static const String preparing = 'preparing';

  /// The status for "come and get it".
  static const String ready = 'ready';

  /// The status for "we cannot".
  static const String rejected = 'rejected';

  /// Whether [action] is one the server will currently accept.
  bool allows(String action) => nextActions.contains(action);

  /// Whether this order is waiting for the shop to answer at all.
  bool get needsAnswer => allows(accepted);
}

/// A page of the shop's orders.
@immutable
class MerchantOrderList {
  /// Creates a page.
  const MerchantOrderList({required this.orders, required this.total});

  /// Reads the `OrderList` response.
  factory MerchantOrderList.fromJson(Map<String, Object?> json) {
    final Object? raw = json['orders'];
    return MerchantOrderList(
      orders: <MerchantOrder>[
        if (raw is List<Object?>)
          for (final Object? entry in raw)
            if (entry is Map<String, Object?>) MerchantOrder.fromJson(entry),
      ],
      total: readInt(json, 'total'),
    );
  }

  /// This page's orders.
  final List<MerchantOrder> orders;

  /// How many matched before paging.
  final int total;

  /// Whether the board is clear.
  bool get isEmpty => orders.isEmpty;
}
