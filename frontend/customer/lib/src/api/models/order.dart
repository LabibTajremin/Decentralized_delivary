import 'package:flutter/foundation.dart';
import 'package:goklay_core/goklay_core.dart';

import '../json.dart';
import 'cart.dart';

/// One end of a delivery, as the order shows it.
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

  /// Who is there — the shop, or the recipient.
  final String name;

  /// A number to call. The rider and the customer both need one.
  final String phone;

  /// The address on one line, already assembled.
  final String singleLine;
}

/// One thing that happened to an order.
@immutable
class OrderEvent {
  /// Creates an event.
  const OrderEvent({
    required this.status,
    required this.label,
    required this.reason,
    required this.at,
  });

  /// Reads the `OrderEvent` object.
  factory OrderEvent.fromJson(Map<String, Object?> json) => OrderEvent(
    status: readString(json, 'status'),
    label: readString(json, 'label'),
    reason: readString(json, 'reason'),
    at: DateTime.tryParse(readString(json, 'at')),
  );

  /// The state it moved to.
  final String status;

  /// The line to print in the timeline, composed by the server.
  final String label;

  /// Why, for the transitions that carry a reason.
  final String reason;

  /// When, or null when the timestamp did not parse.
  final DateTime? at;
}

/// One line of an order. Frozen at placement — these prices are what was
/// charged, not what the shop asks today.
@immutable
class OrderLine {
  /// Creates an order line.
  const OrderLine({
    required this.id,
    required this.name,
    required this.quantity,
    required this.note,
    required this.lineTotal,
  });

  /// Reads the `OrderLine` object.
  factory OrderLine.fromJson(Map<String, Object?> json) => OrderLine(
    id: readString(json, 'id'),
    name: readString(json, 'name'),
    quantity: readInt(json, 'quantity'),
    note: readString(json, 'note'),
    lineTotal: Money.fromJson(readObject(json, 'line_total')),
  );

  /// The line's id.
  final String id;

  /// What it was.
  final String name;

  /// How many.
  final int quantity;

  /// The customer's note.
  final String note;

  /// What it came to.
  final Money lineTotal;
}

/// Whether this order can still be cancelled, and what to say when it cannot.
///
/// P11 gives a free cancellation window that closes when the shop starts
/// preparing. [secondsLeft] is the server's number: the screen counts it down
/// for the look of the thing and re-reads the endpoint rather than deciding on
/// its own that the window has shut.
@immutable
class OrderCancellation {
  /// Creates a cancellation state.
  const OrderCancellation({
    required this.allowed,
    required this.reason,
    required this.text,
    required this.secondsLeft,
  });

  /// Reads the `OrderCancellation` object.
  factory OrderCancellation.fromJson(Map<String, Object?> json) =>
      OrderCancellation(
        allowed: readBool(json, 'allowed'),
        reason: readOptionalString(json, 'reason'),
        text: readString(json, 'text'),
        secondsLeft: readInt(json, 'seconds_left'),
      );

  /// Whether the cancel button is live.
  final bool allowed;

  /// A machine code for why not. For branching, not printing.
  final String? reason;

  /// The sentence to show, composed by the server.
  final String text;

  /// How long is left in the window.
  final int secondsLeft;
}

/// An order, as its customer sees it.
@immutable
class Order {
  /// Creates an order.
  const Order({
    required this.id,
    required this.code,
    required this.merchantId,
    required this.partnerId,
    required this.status,
    required this.statusLabel,
    required this.live,
    required this.paymentMethod,
    required this.lines,
    required this.count,
    required this.total,
    required this.receipt,
    required this.pickup,
    required this.destination,
    required this.events,
    required this.nextActions,
    required this.cancel,
    required this.placedAt,
  });

  /// Reads the `Order` response.
  factory Order.fromJson(Map<String, Object?> json) => Order(
    id: readString(json, 'id'),
    code: readString(json, 'code'),
    merchantId: readString(json, 'merchant_id'),
    partnerId: readOptionalString(json, 'partner_id'),
    status: readString(json, 'status'),
    statusLabel: readString(json, 'status_label'),
    live: readBool(json, 'live'),
    paymentMethod: readString(json, 'payment_method'),
    lines: readList(json, 'lines', OrderLine.fromJson),
    count: readInt(json, 'count'),
    total: Money.fromJson(readObject(json, 'total')),
    receipt: readList(json, 'receipt', ReceiptRow.fromJson),
    pickup: OrderPlace.fromJson(readObject(json, 'pickup')),
    destination: OrderPlace.fromJson(readObject(json, 'destination')),
    events: readList(json, 'events', OrderEvent.fromJson),
    nextActions: <String>[
      for (final Object? action in (json['next_actions'] as List<Object?>?) ??
          const <Object?>[])
        if (action is String) action,
    ],
    cancel: OrderCancellation.fromJson(readObject(json, 'cancel')),
    placedAt: DateTime.tryParse(readString(json, 'placed_at')),
  );

  /// The order's id.
  final String id;

  /// The short code the customer reads out to the shop.
  final String code;

  /// The shop. Here so the review screen can name a subject the customer is
  /// entitled to review — P16 refuses one that was not on the order.
  final String merchantId;

  /// The rider who collected it, or null until one has.
  final String? partnerId;

  /// The lifecycle state, as a code.
  final String status;

  /// The state as a sentence, in the caller's language.
  final String statusLabel;

  /// Whether it is still going. The app branches on this rather than keeping
  /// its own list of which states are terminal.
  final bool live;

  /// `cash` or `online`.
  final String paymentMethod;

  /// What was ordered.
  final List<OrderLine> lines;

  /// How many items.
  final int count;

  /// What it came to.
  final Money total;

  /// The receipt, already computed and labelled.
  final List<ReceiptRow> receipt;

  /// Where it is collected from.
  final OrderPlace pickup;

  /// Where it goes.
  final OrderPlace destination;

  /// The timeline.
  final List<OrderEvent> events;

  /// The transitions the server will currently accept. The app offers these
  /// and derives nothing.
  final List<String> nextActions;

  /// Whether it can still be cancelled.
  final OrderCancellation cancel;

  /// When it was placed.
  final DateTime? placedAt;

  /// Whether a rider is carrying it, so the screen can offer live tracking.
  bool get isTrackable => live && status != 'pending_payment';

  /// Whether it still needs paying for online.
  bool get awaitsPayment => status == 'pending_payment';
}
