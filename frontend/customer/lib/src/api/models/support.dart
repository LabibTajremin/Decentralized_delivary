import 'package:flutter/foundation.dart';
import 'package:goklay_core/goklay_core.dart';


/// The three things a review can be about.
enum ReviewSubject {
  /// The shop.
  merchant('merchant'),

  /// The rider.
  partner('partner'),

  /// One thing that was ordered.
  item('item');

  const ReviewSubject(this.wireValue);

  /// The string the API uses.
  final String wireValue;

  /// Reads a subject off the wire, defaulting to [ReviewSubject.merchant] for
  /// a value this build has never heard of — an unknown subject is a review
  /// still worth showing, not a crash.
  static ReviewSubject parse(String value) {
    for (final ReviewSubject subject in ReviewSubject.values) {
      if (subject.wireValue == value) {
        return subject;
      }
    }
    return ReviewSubject.merchant;
  }
}

/// One review, as it was submitted.
@immutable
class Review {
  /// Creates a review.
  const Review({
    required this.id,
    required this.orderId,
    required this.subject,
    required this.subjectId,
    required this.rating,
    required this.comment,
    required this.createdAt,
  });

  /// Reads the `Review` object.
  factory Review.fromJson(Map<String, Object?> json) => Review(
    id: readString(json, 'id'),
    orderId: readString(json, 'order_id'),
    subject: ReviewSubject.parse(readString(json, 'subject')),
    subjectId: readString(json, 'subject_id'),
    rating: readInt(json, 'rating'),
    comment: readString(json, 'comment'),
    createdAt: DateTime.tryParse(readString(json, 'created_at')),
  );

  /// The review's id.
  final String id;

  /// The order it came from. Only somebody who was on an order may review it,
  /// which the server checks.
  final String orderId;

  /// What it is about.
  final ReviewSubject subject;

  /// Which one.
  final String subjectId;

  /// One to five stars.
  final int rating;

  /// What they wrote, or empty.
  final String comment;

  /// When.
  final DateTime? createdAt;
}

/// A subject's aggregate rating.
@immutable
class Rating {
  /// Creates a rating.
  const Rating({
    required this.subject,
    required this.subjectId,
    required this.average,
    required this.count,
  });

  /// Reads the `Rating` response.
  factory Rating.fromJson(Map<String, Object?> json) => Rating(
    subject: ReviewSubject.parse(readString(json, 'subject')),
    subjectId: readString(json, 'subject_id'),
    average: readDouble(json, 'average'),
    count: readInt(json, 'count'),
  );

  /// What it is about.
  final ReviewSubject subject;

  /// Which one.
  final String subjectId;

  /// The mean out of five, **computed by the server**. Zero with a [count] of
  /// zero for a subject nobody has reviewed.
  final double average;

  /// How many reviews it is over.
  final int count;

  /// Whether anybody has reviewed this yet, so the screen shows "no ratings"
  /// rather than a confident 0.0.
  bool get hasRatings => count > 0;
}

/// A support ticket raised against an order.
@immutable
class SupportTicket {
  /// Creates a ticket.
  const SupportTicket({
    required this.id,
    required this.orderId,
    required this.subject,
    required this.status,
    required this.resolution,
    required this.note,
    required this.createdAt,
    required this.resolvedAt,
  });

  /// Reads the `SupportTicket` object.
  factory SupportTicket.fromJson(Map<String, Object?> json) => SupportTicket(
    id: readString(json, 'id'),
    orderId: readString(json, 'order_id'),
    subject: readString(json, 'subject'),
    status: readString(json, 'status'),
    resolution: readOptionalString(json, 'resolution'),
    note: readString(json, 'note'),
    createdAt: DateTime.tryParse(readString(json, 'created_at')),
    resolvedAt: DateTime.tryParse(readString(json, 'resolved_at')),
  );

  /// The ticket's id.
  final String id;

  /// The order it is about.
  final String orderId;

  /// What the customer said is wrong.
  final String subject;

  /// `open` or `resolved`.
  final String status;

  /// `refunded` or `rejected` once an agent has decided, null while open.
  final String? resolution;

  /// The agent's note.
  final String note;

  /// When it was raised.
  final DateTime? createdAt;

  /// When it was closed, or null.
  final DateTime? resolvedAt;

  /// Whether it is still open.
  bool get isOpen => status == 'open';

  /// Whether the resolution was a refund. The amount is not here: what was
  /// refunded is the payment's business, and the payment endpoint states it.
  bool get wasRefunded => resolution == 'refunded';
}
